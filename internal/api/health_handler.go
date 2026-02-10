package api

import "net/http"

// Health handles GET /api/v1/health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
