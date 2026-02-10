package validate

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// Hostname validation based on RFC 952 and RFC 1123
// - Max 63 characters per label
// - Labels can contain alphanumeric characters and hyphens
// - Labels cannot start or end with a hyphen
// - Total hostname max 253 characters
var hostnameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

const (
	maxHostnameLength = 253
	maxLabelLength    = 63
	minTTL            = 0
	maxTTL            = 604800 // 7 days in seconds
)

// Hostname validates a DNS hostname
func Hostname(hostname string) error {
	if hostname == "" {
		return fmt.Errorf("hostname cannot be empty")
	}

	if len(hostname) > maxHostnameLength {
		return fmt.Errorf("hostname exceeds maximum length of %d characters", maxHostnameLength)
	}

	// Split into labels and validate each
	labels := strings.Split(hostname, ".")
	if len(labels) == 0 {
		return fmt.Errorf("invalid hostname format")
	}

	for _, label := range labels {
		if label == "" {
			return fmt.Errorf("hostname contains empty label")
		}
		if len(label) > maxLabelLength {
			return fmt.Errorf("hostname label '%s' exceeds maximum length of %d characters", label, maxLabelLength)
		}
		if !hostnameRegex.MatchString(label) {
			return fmt.Errorf("invalid hostname label '%s': must contain only alphanumeric characters and hyphens, and cannot start or end with a hyphen", label)
		}
	}

	return nil
}

// IPv4 validates an IPv4 address
func IPv4(ip string) error {
	if ip == "" {
		return fmt.Errorf("IP address cannot be empty")
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid IP address format")
	}

	// Ensure it's IPv4 (not IPv6)
	if parsed.To4() == nil {
		return fmt.Errorf("not a valid IPv4 address")
	}

	return nil
}

// Zone validates a DNS zone name
func Zone(zone string) error {
	if zone == "" {
		return fmt.Errorf("zone cannot be empty")
	}

	if len(zone) > maxHostnameLength {
		return fmt.Errorf("zone exceeds maximum length of %d characters", maxHostnameLength)
	}

	// Split into labels and validate each
	labels := strings.Split(zone, ".")
	if len(labels) == 0 {
		return fmt.Errorf("invalid zone format")
	}

	for _, label := range labels {
		if label == "" {
			return fmt.Errorf("zone contains empty label")
		}
		if len(label) > maxLabelLength {
			return fmt.Errorf("zone label '%s' exceeds maximum length of %d characters", label, maxLabelLength)
		}
		if !hostnameRegex.MatchString(label) {
			return fmt.Errorf("invalid zone label '%s': must contain only alphanumeric characters and hyphens, and cannot start or end with a hyphen", label)
		}
	}

	return nil
}

// TTL validates a time-to-live value
func TTL(ttl uint32) error {
	if ttl < minTTL || ttl > maxTTL {
		return fmt.Errorf("TTL must be between %d and %d seconds", minTTL, maxTTL)
	}
	return nil
}

// RecordName validates a full record name (hostname within a zone)
// For example: "www" or "www.example.com"
func RecordName(name string) error {
	if name == "" {
		return fmt.Errorf("record name cannot be empty")
	}

	// "@" is a valid record name (represents the zone itself)
	if name == "@" {
		return nil
	}

	// Otherwise validate as hostname
	return Hostname(name)
}
