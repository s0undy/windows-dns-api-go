package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"windows-dns-api-go/internal/api"
	"windows-dns-api-go/internal/config"
	"windows-dns-api-go/internal/dns"
	"windows-dns-api-go/internal/middleware"
	"windows-dns-api-go/internal/powershell"
)

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Set up logging
	logger := setupLogger(cfg)
	slog.SetDefault(logger) // Set as default logger for middleware
	logger.Info("Starting Windows DNS API Server", "config", configPath)

	// Create PowerShell executor
	executor := powershell.New(cfg.PowerShell.Executable, cfg.PowerShell.Timeout)

	// Create DNS provider registry
	registry := dns.NewRegistry()

	// Register A record provider
	aProvider := dns.NewARecordProvider(executor, cfg.DNS.ServerName)
	registry.Register(dns.RecordTypeA, aProvider)

	logger.Info("Registered DNS providers", "types", []string{"A"})

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
	server := &http.Server{
		Addr:         addr,
		Handler:      httpHandler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Server listening", "address", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Server stopped")
}

func setupLogger(cfg *config.Config) *slog.Logger {
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

	var handler slog.Handler
	if cfg.Logging.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
