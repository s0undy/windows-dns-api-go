package dns

import "context"

// CommandExecutor is an interface for executing commands (abstraction for testing)
type CommandExecutor interface {
	Execute(ctx context.Context, command string) (string, error)
}

// RecordProvider defines the CRUD interface for DNS record operations
type RecordProvider interface {
	// List retrieves all records of this type in the specified zone
	List(ctx context.Context, zone string) ([]Record, error)

	// Get retrieves a specific record by name and zone
	Get(ctx context.Context, name, zone string) (Record, error)

	// Create adds a new DNS record
	Create(ctx context.Context, record Record) error

	// Update modifies an existing DNS record
	Update(ctx context.Context, record Record) error

	// Delete removes a DNS record
	Delete(ctx context.Context, name, zone, value string) error
}
