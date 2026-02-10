package dns

// RecordType represents the type of DNS record
type RecordType string

const (
	RecordTypeA RecordType = "A"
	// Future record types can be added here:
	// RecordTypeAAAA   RecordType = "AAAA"
	// RecordTypeCNAME  RecordType = "CNAME"
	// RecordTypeMX     RecordType = "MX"
	// RecordTypeTXT    RecordType = "TXT"
)

// Record is the interface that all DNS record types must implement
type Record interface {
	// GetType returns the record type (A, CNAME, etc.)
	GetType() RecordType

	// GetName returns the hostname of the record
	GetName() string

	// GetZone returns the DNS zone the record belongs to
	GetZone() string

	// GetTTL returns the time-to-live in seconds
	GetTTL() uint32

	// Validate checks if the record is valid
	Validate() error
}

// BaseRecord contains fields common to all DNS record types
type BaseRecord struct {
	Name string `json:"name"`
	Zone string `json:"zone"`
	TTL  uint32 `json:"ttl"`
}

// GetName returns the hostname
func (b *BaseRecord) GetName() string {
	return b.Name
}

// GetZone returns the DNS zone
func (b *BaseRecord) GetZone() string {
	return b.Zone
}

// GetTTL returns the time-to-live
func (b *BaseRecord) GetTTL() uint32 {
	return b.TTL
}

// ARecord represents an A (IPv4 address) DNS record
type ARecord struct {
	BaseRecord
	IPv4Address string `json:"ipv4_address"`
}

// GetType returns the record type
func (a *ARecord) GetType() RecordType {
	return RecordTypeA
}

// Validate checks if the A record is valid
func (a *ARecord) Validate() error {
	if a.Name == "" {
		return ErrValidation("name is required")
	}
	if a.Zone == "" {
		return ErrValidation("zone is required")
	}
	if a.IPv4Address == "" {
		return ErrValidation("ipv4_address is required")
	}
	return nil
}
