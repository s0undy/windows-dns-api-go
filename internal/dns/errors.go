package dns

import (
	"errors"
	"fmt"
)

// Sentinel errors for DNS operations
var (
	// ErrNotFound indicates a DNS record was not found
	ErrNotFound = errors.New("record not found")

	// ErrAlreadyExists indicates a DNS record already exists
	ErrAlreadyExists = errors.New("record already exists")
)

// ValidationError represents a validation error
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s", e.Message)
}

// ErrValidation creates a new validation error
func ErrValidation(message string) error {
	return &ValidationError{Message: message}
}

// IsNotFound checks if an error is ErrNotFound
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsAlreadyExists checks if an error is ErrAlreadyExists
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsValidation checks if an error is a ValidationError
func IsValidation(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}
