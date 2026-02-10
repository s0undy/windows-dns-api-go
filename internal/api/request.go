package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const maxRequestBodySize = 1 << 20 // 1 MB

// CreateARecordRequest is the request body for creating an A record
type CreateARecordRequest struct {
	Name        string `json:"name"`
	Zone        string `json:"zone"`
	IPv4Address string `json:"ipv4_address"`
	TTL         uint32 `json:"ttl"`
}

// UpdateARecordRequest is the request body for updating an A record
type UpdateARecordRequest struct {
	Zone        string `json:"zone"`
	IPv4Address string `json:"ipv4_address"`
	TTL         uint32 `json:"ttl"`
}

// DecodeJSON decodes JSON request body with size limit
func DecodeJSON(r *http.Request, v interface{}) error {
	// Limit request body size
	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBodySize)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		if err == io.EOF {
			return fmt.Errorf("request body is empty")
		}
		return fmt.Errorf("invalid JSON: %w", err)
	}

	return nil
}

// GetQueryParam retrieves a query parameter by name
func GetQueryParam(r *http.Request, name string) string {
	return r.URL.Query().Get(name)
}

// GetQueryParamWithDefault retrieves a query parameter with a default value
func GetQueryParamWithDefault(r *http.Request, name, defaultValue string) string {
	value := r.URL.Query().Get(name)
	if value == "" {
		return defaultValue
	}
	return value
}
