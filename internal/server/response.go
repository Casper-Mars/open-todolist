package server

import (
	"encoding/json"
	"net/http"
)

// writeJSON writes a JSON response with the given status code and data.
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If encoding fails, write a fallback error
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
	}
}

// writeError writes a JSON error response with the given status code and message.
func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}
