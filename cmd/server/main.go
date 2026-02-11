package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
	"windows-dns-api-go/internal/api"
	"windows-dns-api-go/internal/config"
	"windows-dns-api-go/internal/dns"
	"windows-dns-api-go/internal/middleware"
	"windows-dns-api-go/internal/powershell"
)

const serviceName = "WindowsDNSAPI"

var (
	// Global server reference for service control
	httpServer *http.Server
	// serviceStopChan is used to signal shutdown from Windows service handler
	serviceStopChan chan struct{}
)

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		// When running as a service, look for config next to executable
		exePath, err := os.Executable()
		if err == nil {
			exeDir := filepath.Dir(exePath)
			configPath = filepath.Join(exeDir, "config.yaml")
		} else {
			configPath = "config.yaml"
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Detect if running as Windows service
	isService := false
	if runtime.GOOS == "windows" {
		isWindowsSvc, err := isWindowsService()
		if err == nil {
			isService = isWindowsSvc
		}
	}

	// Set up logging
	logger, logFile := setupLogger(cfg, isService)
	slog.SetDefault(logger) // Set as default logger for middleware
	logger.Info("Starting Windows DNS API Server",
		"config", configPath,
		"service_mode", isService)

	// Start log rotation timer if enabled
	if cfg.Logging.RotateDays > 0 {
		startLogRotationTimer(logFile, cfg.Logging.RotateDays)
	}

	// Check if running as Windows service
	if runtime.GOOS == "windows" {
		isService, err := isWindowsService()
		if err != nil {
			logger.Error("Failed to determine service status", "error", err)
			os.Exit(1)
		}

		if isService {
			logger.Info("Running as Windows service")
			// Run as Windows service
			if err := runService(cfg); err != nil {
				logger.Error("Service failed", "error", err)
				os.Exit(1)
			}
			return
		}
	}

	// Run in console mode (development or non-Windows)
	logger.Info("Running in console mode")
	if err := runConsole(cfg); err != nil {
		logger.Error("Console mode failed", "error", err)
		os.Exit(1)
	}
}

// runService runs the application as a Windows service
func runService(cfg *config.Config) error {
	// Create stop channel for service control
	stopChan := make(chan struct{})
	serviceStopChan = stopChan // Set global for Windows service handler

	errChan := make(chan error, 1)

	// Start HTTP server in background
	go func() {
		if err := startHTTPServer(cfg, stopChan); err != nil {
			errChan <- err
		}
	}()

	// Run Windows service handler (this blocks until service stops)
	if err := runAsService(serviceName); err != nil {
		return err
	}

	// Wait for HTTP server to finish shutdown
	select {
	case err := <-errChan:
		return err
	case <-time.After(20 * time.Second):
		// Timeout waiting for server shutdown
		return nil
	}
}

// runConsole runs the application in console mode (Ctrl+C to stop)
func runConsole(cfg *config.Config) error {
	stopChan := make(chan struct{})

	// Start HTTP server in background
	go func() {
		if err := startHTTPServer(cfg, stopChan); err != nil {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")
	close(stopChan)

	return nil
}

// startHTTPServer initializes and starts the HTTP server
func startHTTPServer(cfg *config.Config, stopChan chan struct{}) error {
	// Create PowerShell executor
	executor := powershell.New(cfg.PowerShell.Executable, cfg.PowerShell.Timeout)

	// Create DNS provider registry
	registry := dns.NewRegistry()

	// Register A record provider
	aProvider := dns.NewARecordProvider(executor, cfg.DNS.ServerName)
	registry.Register(dns.RecordTypeA, aProvider)

	slog.Info("Registered DNS providers", "types", []string{"A"})

	// Create API handler
	handler := api.NewHandler(registry, cfg)

	// Set up HTTP mux with middleware
	mux := http.NewServeMux()
	apiKeys := cfg.GetAPIKeyMap()
	handler.RegisterRoutes(mux, apiKeys)

	// Wrap mux with middleware stack: Recover -> Logging
	var httpHandler http.Handler = mux
	httpHandler = middleware.Logging(httpHandler)
	httpHandler = middleware.Recover(httpHandler)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Address, cfg.Server.Port)
	httpServer = &http.Server{
		Addr:         addr,
		Handler:      httpHandler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		slog.Info("Server listening", "address", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrChan <- err
		}
	}()

	// Wait for stop signal or server error
	select {
	case err := <-serverErrChan:
		return fmt.Errorf("server error: %w", err)
	case <-stopChan:
		// Graceful shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown error: %w", err)
		}

		slog.Info("Server stopped")
		return nil
	}
}

func setupLogger(cfg *config.Config, isService bool) (*slog.Logger, *lumberjack.Logger) {
	var level slog.Level
	switch cfg.Logging.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	// Determine log file path
	logFilePath := cfg.Logging.FilePath
	if logFilePath == "" {
		// Default: same directory as executable
		exePath, err := os.Executable()
		if err != nil {
			// Fallback to current directory if we can't get executable path
			logFilePath = "windows-dns-api.log"
		} else {
			exeDir := filepath.Dir(exePath)
			logFilePath = filepath.Join(exeDir, "windows-dns-api.log")
		}
	}

	// Ensure log directory exists
	logDir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log directory %s: %v\n", logDir, err)
	}

	// Test that we can create/write to the log file
	testFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Cannot write to log file %s: %v\n", logFilePath, err)
		fmt.Fprintf(os.Stderr, "Logs will only be written to stdout\n")
		// Continue with just stdout logging
	} else {
		testFile.Close()
	}

	// Set up log rotation with lumberjack
	logFile := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    cfg.Logging.MaxSize,    // megabytes
		MaxAge:     0,                      // days (0 = never delete old logs)
		MaxBackups: 0,                      // number of backups (0 = keep all)
		LocalTime:  true,                   // use local time for backup filenames
		Compress:   false,                  // don't compress old logs
	}

	// Force log file creation and test write
	if _, err := logFile.Write([]byte("")); err != nil {
		fmt.Fprintf(os.Stderr, "CRITICAL: Failed to write to log file %s: %v\n", logFilePath, err)
		fmt.Fprintf(os.Stderr, "The service may not have permissions to write to this directory.\n")
		// Continue anyway - at least stderr will show the error
	}

	// Choose output writer based on execution mode
	var writer io.Writer
	if isService {
		// Services don't have stdout, write only to file
		writer = logFile
	} else {
		// Console mode: write to both stdout and file
		writer = io.MultiWriter(os.Stdout, logFile)
	}

	var handler slog.Handler
	if cfg.Logging.Format == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	logger := slog.New(handler)

	// Get absolute path for logging
	absLogPath, _ := filepath.Abs(logFilePath)
	exePath, _ := os.Executable()
	workDir, _ := os.Getwd()

	logger.Info("Logging configured",
		"file_path", absLogPath,
		"max_size_mb", cfg.Logging.MaxSize,
		"rotate_days", cfg.Logging.RotateDays,
		"executable_path", exePath,
		"working_directory", workDir)

	return logger, logFile
}

// startLogRotationTimer starts a background goroutine that rotates the log file
// every N days. The goroutine runs until the program exits.
func startLogRotationTimer(logFile *lumberjack.Logger, rotateDays int) {
	interval := time.Duration(rotateDays) * 24 * time.Hour
	ticker := time.NewTicker(interval)

	go func() {
		for range ticker.C {
			if err := logFile.Rotate(); err != nil {
				slog.Error("Failed to rotate log file", "error", err)
			} else {
				slog.Info("Log file rotated on schedule", "interval_days", rotateDays)
			}
		}
	}()

	slog.Info("Log rotation timer started", "interval_days", rotateDays)
}
