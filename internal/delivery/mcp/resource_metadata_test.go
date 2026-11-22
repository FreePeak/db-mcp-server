package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testResourceURL = "https://db.example.com/mcp"
	testIssuer      = "https://idp.example.com/realms/db"
	testMetadataURL = "https://db.example.com/.well-known/oauth-protected-resource/mcp"
)

func TestResourceMetadata_ServesRFC9728Document(t *testing.T) {
	h := ResourceMetadataHandler(testResourceURL, testIssuer)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource/mcp", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var doc map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &doc))
	assert.Equal(t, testResourceURL, doc["resource"])
	assert.Equal(t, []any{testIssuer}, doc["authorization_servers"])
	assert.Equal(t, []any{"db:read", "db:write", "db:admin"}, doc["scopes_supported"])
	assert.Equal(t, []any{"header"}, doc["bearer_methods_supported"])
}

func TestResourceMetadata_MethodNotAllowed(t *testing.T) {
	h := ResourceMetadataHandler(testResourceURL, testIssuer)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/.well-known/oauth-protected-resource/mcp", nil))
		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code, method)
	}
}

func TestBearerChallenge_401HasQuotedResourceMetadataAndScope(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	h := WithBearerChallenge(next, testMetadataURL, []string{"db:read", "db:write"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t,
		`Bearer resource_metadata="https://db.example.com/.well-known/oauth-protected-resource/mcp", scope="db:read db:write"`,
		rec.Header().Get("WWW-Authenticate"))
}

func TestBearerChallenge_403InsufficientScope(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "insufficient scope", http.StatusForbidden)
	})
	h := WithBearerChallenge(next, testMetadataURL, []string{"db:admin"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t,
		`Bearer resource_metadata="https://db.example.com/.well-known/oauth-protected-resource/mcp", scope="db:admin", error="insufficient_scope"`,
		rec.Header().Get("WWW-Authenticate"))
}

func TestBearerChallenge_200Untouched(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	h := WithBearerChallenge(next, testMetadataURL, []string{"db:read"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("WWW-Authenticate"))
	assert.Equal(t, "ok", rec.Body.String())
}

func TestBearerChallenge_ImplicitWrite200Untouched(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("implicit"))
	})
	h := WithBearerChallenge(next, testMetadataURL, []string{"db:read"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("WWW-Authenticate"))
	assert.Equal(t, "implicit", rec.Body.String())
}

func TestBearerChallenge_FlushReachesUnderlyingWriter(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
	})
	h := WithBearerChallenge(next, testMetadataURL, []string{"db:read"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	assert.True(t, rec.Flushed)
}

func TestResourceMetadataSettings(t *testing.T) {
	tests := []struct {
		name            string
		env             map[string]string
		wantEnabled     bool
		wantErr         bool
		wantMetadataURL string
	}{
		{
			name:            "both set adds resource path to metadata URL",
			env:             map[string]string{"DB_MCP_OIDC_ISSUER": testIssuer, "DB_MCP_RESOURCE_URL": testResourceURL},
			wantEnabled:     true,
			wantMetadataURL: testMetadataURL,
		},
		{
			name:            "root resource URL has no path suffix",
			env:             map[string]string{"DB_MCP_OIDC_ISSUER": testIssuer, "DB_MCP_RESOURCE_URL": "https://db.example.com"},
			wantEnabled:     true,
			wantMetadataURL: "https://db.example.com/.well-known/oauth-protected-resource",
		},
		{
			name:        "only issuer is disabled without error",
			env:         map[string]string{"DB_MCP_OIDC_ISSUER": testIssuer},
			wantEnabled: false,
		},
		{
			name:    "only resource URL is an error",
			env:     map[string]string{"DB_MCP_RESOURCE_URL": testResourceURL},
			wantErr: true,
		},
		{
			name:    "ftp scheme is an error",
			env:     map[string]string{"DB_MCP_OIDC_ISSUER": testIssuer, "DB_MCP_RESOURCE_URL": "ftp://db.example.com/mcp"},
			wantErr: true,
		},
		{
			name:    "garbage is an error",
			env:     map[string]string{"DB_MCP_OIDC_ISSUER": testIssuer, "DB_MCP_RESOURCE_URL": "not a url"},
			wantErr: true,
		},
		{
			name:    "unparsable URL is an error",
			env:     map[string]string{"DB_MCP_OIDC_ISSUER": testIssuer, "DB_MCP_RESOURCE_URL": "://bad"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(key string) string { return tc.env[key] }

			settings, enabled, err := ResourceMetadataSettingsFromEnv(getenv)

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantEnabled, enabled)
			if tc.wantEnabled {
				assert.Equal(t, tc.wantMetadataURL, settings.MetadataURL)
				assert.Equal(t, tc.env["DB_MCP_RESOURCE_URL"], settings.ResourceURL)
				assert.Equal(t, tc.env["DB_MCP_OIDC_ISSUER"], settings.Issuer)
			}
		})
	}
}

func TestMountResourceMetadata_ServesBothPathsWithoutAuth(t *testing.T) {
	env := map[string]string{"DB_MCP_OIDC_ISSUER": testIssuer, "DB_MCP_RESOURCE_URL": testResourceURL}
	protected := APIKeyAuthWithScopes("test-key", []string{"db:read"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	mux := http.NewServeMux()
	require.NoError(t, MountStreamableRoutes(mux, func(key string) string { return env[key] }, []string{"db:read"}, protected))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for _, path := range []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp"} {
		resp, err := http.Get(srv.URL + path)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, path)
	}

	resp, err := http.Post(srv.URL+"/mcp", "application/json", nil)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t,
		`Bearer resource_metadata="https://db.example.com/.well-known/oauth-protected-resource/mcp", scope="db:read"`,
		resp.Header.Get("WWW-Authenticate"))
}

func TestMountStreamableRoutes_DisabledKeepsTodayBehaviour(t *testing.T) {
	env := map[string]string{"DB_MCP_OIDC_ISSUER": testIssuer}
	protected := APIKeyAuthWithScopes("test-key", []string{"db:read"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	mux := http.NewServeMux()
	require.NoError(t, MountStreamableRoutes(mux, func(key string) string { return env[key] }, []string{"db:read"}, protected))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.NotContains(t, rec.Header().Get("WWW-Authenticate"), "resource_metadata")

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource/mcp", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
