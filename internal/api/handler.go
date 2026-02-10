package api

import (
	"windows-dns-api-go/internal/config"
	"windows-dns-api-go/internal/dns"
)

// Handler holds shared dependencies for all API handlers
type Handler struct {
	registry *dns.Registry
	config   *config.Config
}

// NewHandler creates a new handler with shared dependencies
func NewHandler(registry *dns.Registry, config *config.Config) *Handler {
	return &Handler{
		registry: registry,
		config:   config,
	}
}
