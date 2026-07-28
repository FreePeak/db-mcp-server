package mcp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIKeyAuth_DisabledWhenKeyEmpty covers the default dev-friendly
// behaviour: when no API key is configured the middleware is a pass-through,
// so single-user stdio deployments don't need to set anything.
func TestAPIKeyAuth_DisabledWhenKeyEmpty(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	})
	h := APIKeyAuth("")(next)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.True(t, called, "next handler must be invoked when no API key is configured")
	assert.Equal(t, http.StatusTeapot, rec.Code)
}

// TestAPIKeyAuth_RejectsMissingHeader locks in the unauthenticated path:
// any request without the Bearer header must return 401 with a
// WWW-Authenticate challenge.
func TestAPIKeyAuth_RejectsMissingHeader(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	h := APIKeyAuth("secret-key")(next)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.False(t, called, "next handler must NOT run for missing Authorization header")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), "Bearer")
}

// TestAPIKeyAuth_RejectsWrongKey ensures a wrong bearer token returns 401.
func TestAPIKeyAuth_RejectsWrongKey(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	h := APIKeyAuth("secret-key")(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestAPIKeyAuth_AcceptsMatchingKey locks in the success path.
func TestAPIKeyAuth_AcceptsMatchingKey(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	h := APIKeyAuth("secret-key")(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret-key")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.True(t, called, "next handler must run for a matching bearer token")
	assert.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))
}

// TestAPIKeyAuth_RejectsMalformedHeader covers a malformed Authorization
// header that lacks the "Bearer " prefix.
func TestAPIKeyAuth_RejectsMalformedHeader(t *testing.T) {
	h := APIKeyAuth("secret-key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "secret-key") // missing prefix
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
