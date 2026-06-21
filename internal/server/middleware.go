package server

import (
	"net/http"
	"strings"
)

const maxBodySize = 1 << 20 // 1 MB

// limitBody is a middleware that limits the request body size to 1 MB.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		next.ServeHTTP(w, r)
	})
}

// requireJSON is a middleware that validates the Content-Type header
// for requests that have a body (POST, PATCH, PUT).
func requireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPatch || r.Method == http.MethodPut {
			ct := r.Header.Get("Content-Type")
			if ct == "" {
				writeError(w, http.StatusBadRequest, "Content-Type header is required")
				return
			}
			// Accept application/json with optional charset
			if !strings.HasPrefix(ct, "application/json") {
				writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// chain applies middlewares in order.
func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
