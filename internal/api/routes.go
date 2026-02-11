package api

import (
	"net/http"

	"windows-dns-api-go/internal/middleware"
)

// RegisterRoutes registers all API routes on the provided mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux, apiKeys map[string]string) {
	// Create auth middleware
	authMiddleware := middleware.Auth(apiKeys)

	// Health endpoint (no auth)
	mux.HandleFunc("GET /api/v1/health", h.Health)

	// API Documentation (no auth)
	mux.HandleFunc("GET /docs", h.DocsHandler)

	// A record endpoints (with auth)
	mux.Handle("GET /api/v1/records/a", authMiddleware(http.HandlerFunc(h.ListARecords)))
	mux.Handle("GET /api/v1/records/a/{name}", authMiddleware(http.HandlerFunc(h.GetARecord)))
	mux.Handle("POST /api/v1/records/a", authMiddleware(http.HandlerFunc(h.CreateARecord)))
	mux.Handle("PUT /api/v1/records/a/{name}", authMiddleware(http.HandlerFunc(h.UpdateARecord)))
	mux.Handle("DELETE /api/v1/records/a/{name}", authMiddleware(http.HandlerFunc(h.DeleteARecord)))
}
