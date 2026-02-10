package api

import (
	"net/http"

	"windows-dns-api-go/internal/dns"
)

// ListARecords handles GET /api/v1/records/a
func (h *Handler) ListARecords(w http.ResponseWriter, r *http.Request) {
	// Get zone from query param or use default
	zone := GetQueryParamWithDefault(r, "zone", h.config.DNS.DefaultZone)

	// Get provider
	provider, err := h.registry.Get(dns.RecordTypeA)
	if err != nil {
		WriteInternalError(w, err)
		return
	}

	// List records
	records, err := provider.List(r.Context(), zone)
	if err != nil {
		HandleDNSError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, records)
}

// GetARecord handles GET /api/v1/records/a/{name}
func (h *Handler) GetARecord(w http.ResponseWriter, r *http.Request) {
	// Get name from path
	name := r.PathValue("name")
	if name == "" {
		WriteBadRequest(w, "name is required")
		return
	}

	// Get zone from query param or use default
	zone := GetQueryParamWithDefault(r, "zone", h.config.DNS.DefaultZone)

	// Get provider
	provider, err := h.registry.Get(dns.RecordTypeA)
	if err != nil {
		WriteInternalError(w, err)
		return
	}

	// Get record
	record, err := provider.Get(r.Context(), name, zone)
	if err != nil {
		HandleDNSError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, record)
}

// CreateARecord handles POST /api/v1/records/a
func (h *Handler) CreateARecord(w http.ResponseWriter, r *http.Request) {
	// Decode request body
	var req CreateARecordRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteBadRequest(w, err.Error())
		return
	}

	// Use default zone if not specified
	if req.Zone == "" {
		req.Zone = h.config.DNS.DefaultZone
	}

	// Create record
	record := &dns.ARecord{
		BaseRecord: dns.BaseRecord{
			Name: req.Name,
			Zone: req.Zone,
			TTL:  req.TTL,
		},
		IPv4Address: req.IPv4Address,
	}

	// Get provider
	provider, err := h.registry.Get(dns.RecordTypeA)
	if err != nil {
		WriteInternalError(w, err)
		return
	}

	// Create record
	if err := provider.Create(r.Context(), record); err != nil {
		HandleDNSError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, record)
}

// UpdateARecord handles PUT /api/v1/records/a/{name}
func (h *Handler) UpdateARecord(w http.ResponseWriter, r *http.Request) {
	// Get name from path
	name := r.PathValue("name")
	if name == "" {
		WriteBadRequest(w, "name is required")
		return
	}

	// Decode request body
	var req UpdateARecordRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteBadRequest(w, err.Error())
		return
	}

	// Use default zone if not specified
	if req.Zone == "" {
		req.Zone = h.config.DNS.DefaultZone
	}

	// Create record
	record := &dns.ARecord{
		BaseRecord: dns.BaseRecord{
			Name: name,
			Zone: req.Zone,
			TTL:  req.TTL,
		},
		IPv4Address: req.IPv4Address,
	}

	// Get provider
	provider, err := h.registry.Get(dns.RecordTypeA)
	if err != nil {
		WriteInternalError(w, err)
		return
	}

	// Update record
	if err := provider.Update(r.Context(), record); err != nil {
		HandleDNSError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, record)
}

// DeleteARecord handles DELETE /api/v1/records/a/{name}
func (h *Handler) DeleteARecord(w http.ResponseWriter, r *http.Request) {
	// Get name from path
	name := r.PathValue("name")
	if name == "" {
		WriteBadRequest(w, "name is required")
		return
	}

	// Get zone from query param or use default
	zone := GetQueryParamWithDefault(r, "zone", h.config.DNS.DefaultZone)

	// Get value (IP address) from query param - required for A records
	value := GetQueryParam(r, "value")
	if value == "" {
		WriteBadRequest(w, "value query parameter (IP address) is required")
		return
	}

	// Get provider
	provider, err := h.registry.Get(dns.RecordTypeA)
	if err != nil {
		WriteInternalError(w, err)
		return
	}

	// Delete record
	if err := provider.Delete(r.Context(), name, zone, value); err != nil {
		HandleDNSError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Record deleted successfully",
	})
}
