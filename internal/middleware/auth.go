package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Auth creates an authentication middleware
func Auth(apiKeys map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get API key from header
			apiKey := r.Header.Get("X-API-Key")

			if apiKey == "" {
				slog.Warn("missing API key", "remote_addr", r.RemoteAddr, "path", r.URL.Path)
				writeUnauthorized(w, "Missing X-API-Key header")
				return
			}

			// Check if API key is valid
			keyName, valid := apiKeys[apiKey]
			if !valid {
				slog.Warn("invalid API key", "remote_addr", r.RemoteAddr, "path", r.URL.Path)
				writeUnauthorized(w, "Invalid API key")
				return
			}

			// Log successful authentication
			slog.Debug("authenticated request", "key_name", keyName, "remote_addr", r.RemoteAddr, "path", r.URL.Path)

			// Call next handler
			next.ServeHTTP(w, r)
		})
	}
}

// writeUnauthorized writes a 401 Unauthorized JSON response
func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"message": message,
			"code":    "unauthorized",
		},
	})
}
