//go:build windows

package main

import (
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sys/windows/svc"
)

// windowsService implements the Windows service interface
type windowsService struct{}

// Execute is called by the Windows Service Control Manager
func (ws *windowsService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	// Tell SCM we're starting
	changes <- svc.Status{State: svc.StartPending}
	slog.Info("Windows service starting...")

	// Give the HTTP server a moment to start
	time.Sleep(2 * time.Second)

	// Signal that we're running
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	slog.Info("Windows service status: Running")

	// Wait for service control commands
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			slog.Info("Received service stop signal")
			changes <- svc.Status{State: svc.StopPending}

			// Signal the HTTP server to stop
			if serviceStopChan != nil {
				close(serviceStopChan)
			}

			// Give server time to shutdown gracefully
			time.Sleep(3 * time.Second)

			return false, 0
		default:
			slog.Warn("Unexpected service control request", "cmd", c.Cmd)
		}
	}

	// Service control channel closed unexpectedly
	return false, 0
}

// runAsService starts the application as a Windows service
func runAsService(name string) error {
	slog.Info("Starting as Windows service", "service_name", name)

	ws := &windowsService{}

	// Start the service (this blocks until service stops)
	err := svc.Run(name, ws)
	if err != nil {
		return fmt.Errorf("service failed: %w", err)
	}

	return nil
}

// isWindowsService returns true if the process is running as a Windows service
func isWindowsService() (bool, error) {
	return svc.IsWindowsService()
}
