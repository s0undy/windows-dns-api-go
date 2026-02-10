package dns

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"windows-dns-api-go/internal/validate"
)

// ARecordProvider implements RecordProvider for A records
type ARecordProvider struct {
	executor   CommandExecutor
	serverName string
}

// NewARecordProvider creates a new A record provider
func NewARecordProvider(executor CommandExecutor, serverName string) *ARecordProvider {
	return &ARecordProvider{
		executor:   executor,
		serverName: serverName,
	}
}

// List retrieves all A records in the specified zone
func (p *ARecordProvider) List(ctx context.Context, zone string) ([]Record, error) {
	if err := validate.Zone(zone); err != nil {
		return nil, fmt.Errorf("invalid zone: %w", err)
	}

	// Build PowerShell command
	// We flatten RecordData via Select-Object to avoid CIM metadata bloat
	cmd := fmt.Sprintf(`
		$records = Get-DnsServerResourceRecord -ZoneName "%s" -ComputerName "%s" -RRType A -ErrorAction Stop
		$records | Select-Object @{Name='HostName';Expression={$_.HostName}}, @{Name='TimeToLive';Expression={$_.TimeToLive.TotalSeconds}}, @{Name='IPv4Address';Expression={$_.RecordData.IPv4Address.IPAddressToString}} | ConvertTo-Json
	`, zone, p.serverName)

	output, err := p.executor.Execute(ctx, cmd)
	if err != nil {
		// Check if zone doesn't exist or has no records
		if strings.Contains(err.Error(), "ObjectNotFound") || strings.Contains(err.Error(), "Cannot find") {
			return []Record{}, nil
		}
		return nil, fmt.Errorf("failed to list A records: %w", err)
	}

	// Handle empty result
	if output == "" || output == "null" {
		return []Record{}, nil
	}

	// Parse JSON output
	// PowerShell returns single object as object, multiple as array
	var rawRecords []map[string]interface{}

	// Try parsing as array first
	if err := json.Unmarshal([]byte(output), &rawRecords); err != nil {
		// Try parsing as single object
		var singleRecord map[string]interface{}
		if err := json.Unmarshal([]byte(output), &singleRecord); err != nil {
			return nil, fmt.Errorf("failed to parse PowerShell output: %w", err)
		}
		rawRecords = []map[string]interface{}{singleRecord}
	}

	// Convert to ARecord structs
	records := make([]Record, 0, len(rawRecords))
	for _, raw := range rawRecords {
		hostname, _ := raw["HostName"].(string)
		ipv4Address, _ := raw["IPv4Address"].(string)
		ttl, _ := raw["TimeToLive"].(float64)

		record := &ARecord{
			BaseRecord: BaseRecord{
				Name: hostname,
				Zone: zone,
				TTL:  uint32(ttl),
			},
			IPv4Address: ipv4Address,
		}

		records = append(records, record)
	}

	return records, nil
}

// Get retrieves a specific A record by name and zone
func (p *ARecordProvider) Get(ctx context.Context, name, zone string) (Record, error) {
	if err := validate.RecordName(name); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}
	if err := validate.Zone(zone); err != nil {
		return nil, fmt.Errorf("invalid zone: %w", err)
	}

	// Build PowerShell command
	cmd := fmt.Sprintf(`
		$record = Get-DnsServerResourceRecord -ZoneName "%s" -ComputerName "%s" -Name "%s" -RRType A -ErrorAction Stop
		$record | Select-Object @{Name='HostName';Expression={$_.HostName}}, @{Name='TimeToLive';Expression={$_.TimeToLive.TotalSeconds}}, @{Name='IPv4Address';Expression={$_.RecordData.IPv4Address.IPAddressToString}} | ConvertTo-Json
	`, zone, p.serverName, name)

	output, err := p.executor.Execute(ctx, cmd)
	if err != nil {
		if strings.Contains(err.Error(), "ObjectNotFound") || strings.Contains(err.Error(), "Cannot find") {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get A record: %w", err)
	}

	// Handle empty result
	if output == "" || output == "null" {
		return nil, ErrNotFound
	}

	// Parse JSON output (single object or array with one element)
	var rawRecord map[string]interface{}

	// Try parsing as object first
	if err := json.Unmarshal([]byte(output), &rawRecord); err != nil {
		// Try parsing as array
		var rawRecords []map[string]interface{}
		if err := json.Unmarshal([]byte(output), &rawRecords); err != nil {
			return nil, fmt.Errorf("failed to parse PowerShell output: %w", err)
		}
		if len(rawRecords) == 0 {
			return nil, ErrNotFound
		}
		rawRecord = rawRecords[0]
	}

	hostname, _ := rawRecord["HostName"].(string)
	ipv4Address, _ := rawRecord["IPv4Address"].(string)
	ttl, _ := rawRecord["TimeToLive"].(float64)

	record := &ARecord{
		BaseRecord: BaseRecord{
			Name: hostname,
			Zone: zone,
			TTL:  uint32(ttl),
		},
		IPv4Address: ipv4Address,
	}

	return record, nil
}

// Create adds a new A record
func (p *ARecordProvider) Create(ctx context.Context, record Record) error {
	aRecord, ok := record.(*ARecord)
	if !ok {
		return fmt.Errorf("invalid record type: expected *ARecord")
	}

	// Validate record
	if err := aRecord.Validate(); err != nil {
		return err
	}

	// Validate individual fields
	if err := validate.RecordName(aRecord.Name); err != nil {
		return fmt.Errorf("invalid name: %w", err)
	}
	if err := validate.Zone(aRecord.Zone); err != nil {
		return fmt.Errorf("invalid zone: %w", err)
	}
	if err := validate.IPv4(aRecord.IPv4Address); err != nil {
		return fmt.Errorf("invalid IPv4 address: %w", err)
	}
	if err := validate.TTL(aRecord.TTL); err != nil {
		return fmt.Errorf("invalid TTL: %w", err)
	}

	// Build PowerShell command
	cmd := fmt.Sprintf(`
		Add-DnsServerResourceRecordA -ZoneName "%s" -ComputerName "%s" -Name "%s" -IPv4Address "%s" -TimeToLive (New-TimeSpan -Seconds %d) -ErrorAction Stop
	`, aRecord.Zone, p.serverName, aRecord.Name, aRecord.IPv4Address, aRecord.TTL)

	_, err := p.executor.Execute(ctx, cmd)
	if err != nil {
		if strings.Contains(err.Error(), "ResourceExists") || strings.Contains(err.Error(), "already exists") {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to create A record: %w", err)
	}

	return nil
}

// Update modifies an existing A record
// Strategy: Remove old + Add new (simpler than Set-DnsServerResourceRecord)
func (p *ARecordProvider) Update(ctx context.Context, record Record) error {
	aRecord, ok := record.(*ARecord)
	if !ok {
		return fmt.Errorf("invalid record type: expected *ARecord")
	}

	// Validate record
	if err := aRecord.Validate(); err != nil {
		return err
	}

	// Validate individual fields
	if err := validate.RecordName(aRecord.Name); err != nil {
		return fmt.Errorf("invalid name: %w", err)
	}
	if err := validate.Zone(aRecord.Zone); err != nil {
		return fmt.Errorf("invalid zone: %w", err)
	}
	if err := validate.IPv4(aRecord.IPv4Address); err != nil {
		return fmt.Errorf("invalid IPv4 address: %w", err)
	}
	if err := validate.TTL(aRecord.TTL); err != nil {
		return fmt.Errorf("invalid TTL: %w", err)
	}

	// Check if record exists
	existingRecord, err := p.Get(ctx, aRecord.Name, aRecord.Zone)
	if err != nil {
		if IsNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to check existing record: %w", err)
	}

	existingARecord := existingRecord.(*ARecord)

	// Remove old record
	deleteCmd := fmt.Sprintf(`
		Remove-DnsServerResourceRecord -ZoneName "%s" -ComputerName "%s" -Name "%s" -RRType A -RecordData "%s" -Force -ErrorAction Stop
	`, aRecord.Zone, p.serverName, aRecord.Name, existingARecord.IPv4Address)

	_, err = p.executor.Execute(ctx, deleteCmd)
	if err != nil {
		return fmt.Errorf("failed to remove old record during update: %w", err)
	}

	// Add new record
	addCmd := fmt.Sprintf(`
		Add-DnsServerResourceRecordA -ZoneName "%s" -ComputerName "%s" -Name "%s" -IPv4Address "%s" -TimeToLive (New-TimeSpan -Seconds %d) -ErrorAction Stop
	`, aRecord.Zone, p.serverName, aRecord.Name, aRecord.IPv4Address, aRecord.TTL)

	_, err = p.executor.Execute(ctx, addCmd)
	if err != nil {
		return fmt.Errorf("failed to add new record during update: %w", err)
	}

	return nil
}

// Delete removes an A record
func (p *ARecordProvider) Delete(ctx context.Context, name, zone, value string) error {
	if err := validate.RecordName(name); err != nil {
		return fmt.Errorf("invalid name: %w", err)
	}
	if err := validate.Zone(zone); err != nil {
		return fmt.Errorf("invalid zone: %w", err)
	}
	if err := validate.IPv4(value); err != nil {
		return fmt.Errorf("invalid IPv4 address: %w", err)
	}

	// Build PowerShell command
	cmd := fmt.Sprintf(`
		Remove-DnsServerResourceRecord -ZoneName "%s" -ComputerName "%s" -Name "%s" -RRType A -RecordData "%s" -Force -ErrorAction Stop
	`, zone, p.serverName, name, value)

	_, err := p.executor.Execute(ctx, cmd)
	if err != nil {
		if strings.Contains(err.Error(), "ObjectNotFound") || strings.Contains(err.Error(), "Cannot find") {
			return ErrNotFound
		}
		return fmt.Errorf("failed to delete A record: %w", err)
	}

	return nil
}
