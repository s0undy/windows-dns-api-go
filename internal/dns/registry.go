package dns

import "fmt"

// Registry maps RecordType to RecordProvider
type Registry struct {
	providers map[RecordType]RecordProvider
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[RecordType]RecordProvider),
	}
}

// Register adds a provider for a specific record type
func (r *Registry) Register(recordType RecordType, provider RecordProvider) {
	r.providers[recordType] = provider
}

// Get retrieves a provider for a specific record type
func (r *Registry) Get(recordType RecordType) (RecordProvider, error) {
	provider, ok := r.providers[recordType]
	if !ok {
		return nil, fmt.Errorf("no provider registered for record type %s", recordType)
	}
	return provider, nil
}

// Has checks if a provider is registered for a specific record type
func (r *Registry) Has(recordType RecordType) bool {
	_, ok := r.providers[recordType]
	return ok
}

// Types returns all registered record types
func (r *Registry) Types() []RecordType {
	types := make([]RecordType, 0, len(r.providers))
	for recordType := range r.providers {
		types = append(types, recordType)
	}
	return types
}
