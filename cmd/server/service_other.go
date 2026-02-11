//go:build !windows

package main

import (
	"fmt"
)

// isWindowsService always returns false on non-Windows platforms
func isWindowsService() (bool, error) {
	return false, nil
}

// runAsService is not supported on non-Windows platforms
func runAsService(name string) error {
	return fmt.Errorf("Windows service mode is only supported on Windows")
}
