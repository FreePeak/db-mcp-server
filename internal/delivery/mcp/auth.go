package mcp

import (
	"context"
	"crypto/subtle"
	"net/http"
	"time"

	sdkAuth "github.com/modelcontextprotocol/go-sdk/auth"
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

// OIDCBearerAuth applies OIDC/JWT verification and required scopes to the
// official MCP SDK request context.
func OIDCBearerAuth(verifier sdkAuth.TokenVerifier, scopes []string) func(http.Handler) http.Handler {
	return sdkAuth.RequireBearerToken(verifier, &sdkAuth.RequireBearerTokenOptions{Scopes: scopes})
}

// APIKeyAuthWithScopes is the Streamable HTTP authentication boundary. It
// keeps the static-key deployment simple while producing the same TokenInfo
// context that an OIDC verifier will provide later, so tool-level scopes and
// SDK session ownership work consistently.
func APIKeyAuthWithScopes(apiKey string, scopes []string) func(http.Handler) http.Handler {
	grantedScopes := append([]string(nil), scopes...)
	if len(grantedScopes) == 0 {
		grantedScopes = []string{"db:read"}
	}
	verifier := func(_ context.Context, token string, _ *http.Request) (*sdkAuth.TokenInfo, error) {
		if subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) != 1 {
			return nil, sdkAuth.ErrInvalidToken
		}
		return &sdkAuth.TokenInfo{
			UserID:     "api-key",
			Scopes:     grantedScopes,
			Expiration: time.Now().Add(10 * 365 * 24 * time.Hour),
		}, nil
	}
	return sdkAuth.RequireBearerToken(verifier, &sdkAuth.RequireBearerTokenOptions{Scopes: grantedScopes})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="db-mcp-server"`)
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}
