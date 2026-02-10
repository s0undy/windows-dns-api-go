package powershell

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Executor handles PowerShell command execution
type Executor struct {
	executable string
	timeout    time.Duration
}

// New creates a new PowerShell executor
func New(executable string, timeout time.Duration) *Executor {
	return &Executor{
		executable: executable,
		timeout:    timeout,
	}
}

// Execute runs a PowerShell command with timeout
func (e *Executor) Execute(ctx context.Context, command string) (string, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Build PowerShell command with required flags
	args := []string{
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		command,
	}

	// Create command
	cmd := exec.CommandContext(timeoutCtx, e.executable, args...)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute command
	err := cmd.Run()

	// Get output
	output := strings.TrimSpace(stdout.String())
	errOutput := strings.TrimSpace(stderr.String())

	// Handle errors
	if err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("PowerShell command timed out after %v", e.timeout)
		}

		// Include stderr in error if available
		if errOutput != "" {
			return "", fmt.Errorf("PowerShell command failed: %w (stderr: %s)", err, errOutput)
		}

		return "", fmt.Errorf("PowerShell command failed: %w", err)
	}

	// PowerShell might write errors to stderr even on success
	// But if there's output on stdout, we consider it successful
	if errOutput != "" && output == "" {
		return "", fmt.Errorf("PowerShell error: %s", errOutput)
	}

	return output, nil
}

// ExecuteWithTimeout is a convenience method that creates a context with timeout
func (e *Executor) ExecuteWithTimeout(command string) (string, error) {
	return e.Execute(context.Background(), command)
}
