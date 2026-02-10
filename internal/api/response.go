package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"windows-dns-api-go/internal/dns"
)

// Response is the standard JSON response envelope
type Response struct {
	Data  interface{} `json:"data,omitempty"`
	Error *ErrorInfo  `json:"error,omitempty"`
}

// ErrorInfo contains error details
type ErrorInfo struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// WriteJSON writes a JSON response
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := Response{Data: data}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// WriteError writes an error JSON response
func WriteError(w http.ResponseWriter, statusCode int, message string, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := Response{
		Error: &ErrorInfo{
			Message: message,
			Code:    code,
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}

// WriteInternalError writes a 500 Internal Server Error response
func WriteInternalError(w http.ResponseWriter, err error) {
	slog.Error("internal server error", "error", err)
	WriteError(w, http.StatusInternalServerError, "Internal server error", "internal_error")
}

// WriteBadRequest writes a 400 Bad Request response
func WriteBadRequest(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusBadRequest, message, "bad_request")
}

// WriteNotFound writes a 404 Not Found response
func WriteNotFound(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusNotFound, message, "not_found")
}

// WriteConflict writes a 409 Conflict response
func WriteConflict(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusConflict, message, "conflict")
}

// WriteUnauthorized writes a 401 Unauthorized response
func WriteUnauthorized(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusUnauthorized, message, "unauthorized")
}

// HandleDNSError maps DNS errors to appropriate HTTP responses
func HandleDNSError(w http.ResponseWriter, err error) {
	if dns.IsNotFound(err) {
		WriteNotFound(w, "Record not found")
		return
	}

	if dns.IsAlreadyExists(err) {
		WriteConflict(w, "Record already exists")
		return
	}

	if dns.IsValidation(err) {
		WriteBadRequest(w, err.Error())
		return
	}

	WriteInternalError(w, err)
}
