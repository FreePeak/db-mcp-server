package mcp

import (
	"crypto/subtle"
	"net/http"
)

// APIKeyAuth returns an http.Handler middleware that requires every request
// to carry an `Authorization: Bearer <key>` header matching the configured
// API key. The middleware is a no-op when apiKey is empty, so single-user
// stdio deployments are unaffected.
//
// This addresses FreePeak/db-mcp-server issue #57: Docker images exposing
// the SSE/HTTP transport must not be open to anyone who can reach the
// container's port. Setting DB_MCP_API_KEY (or the API_KEY env var) at
// container start is sufficient to lock the transport down.
func APIKeyAuth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if apiKey == "" {
				next.ServeHTTP(w, r)
				return
			}
			header := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
				unauthorized(w)
				return
			}
			provided := header[len(prefix):]
			// constant-time compare to avoid timing attacks.
			if subtle.ConstantTimeCompare([]byte(provided), []byte(apiKey)) != 1 {
				unauthorized(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="db-mcp-server"`)
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}
