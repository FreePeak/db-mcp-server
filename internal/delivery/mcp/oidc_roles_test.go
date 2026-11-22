package mcp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testOIDCAudience is the audience the fake issuer stamps on its tokens.
const testOIDCAudience = "db-mcp-test"

const testRolesJSON = `{"claim": "groups", "roles": [
  {"value": "db-analysts", "scopes": ["db:read"], "databases": ["sales"], "max_query_cost": 50000},
  {"value": "db-admins", "scopes": ["db:admin"], "databases": ["*"]}
]}`

func loadTestRoles(t *testing.T) *OIDCRoles {
	t.Helper()
	path := filepath.Join(t.TempDir(), "roles.json")
	require.NoError(t, os.WriteFile(path, []byte(testRolesJSON), 0o600))
	roles, err := LoadOIDCRoles(path)
	require.NoError(t, err)
	return roles
}

func TestOIDCRoles_ResolveFirstMatch(t *testing.T) {
	roles := loadTestRoles(t)

	// Both roles match the claim; the one listed first in the file wins.
	got := roles.Resolve(map[string]any{"groups": []any{"db-admins", "db-analysts"}})
	assert.Equal(t, []string{"db:read"}, got.Scopes)
	assert.Equal(t, []string{"sales"}, got.Databases)
	assert.Equal(t, 50000.0, got.MaxQueryCost)

	got = roles.Resolve(map[string]any{"groups": []any{"db-admins"}})
	assert.Equal(t, []string{"db:admin"}, got.Scopes)
	assert.Equal(t, []string{"*"}, got.Databases)
}

func TestOIDCRoles_NoMatchDeniesAll(t *testing.T) {
	roles := loadTestRoles(t)

	got := roles.Resolve(map[string]any{"groups": []any{"contractors"}})
	assert.Empty(t, got.Scopes)
	assert.NotNil(t, got.Databases, "the empty allowlist must be present so the database check denies")
	assert.Empty(t, got.Databases)
	assert.Zero(t, got.MaxQueryCost)

	got = roles.Resolve(map[string]any{})
	assert.Empty(t, got.Scopes)
	assert.Empty(t, got.Databases)
}

func TestOIDCRoles_ClaimAsStringOrArray(t *testing.T) {
	roles := loadTestRoles(t)

	asString := roles.Resolve(map[string]any{"groups": "db-admins"})
	assert.Equal(t, []string{"db:admin"}, asString.Scopes)

	asArray := roles.Resolve(map[string]any{"groups": []any{"db-analysts"}})
	assert.Equal(t, []string{"db:read"}, asArray.Scopes)
}

func TestLoadOIDCRoles_RejectsEmptyClaimOrDuplicateValue(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "blank claim",
			body:    `{"claim": "  ", "roles": [{"value": "a", "scopes": ["db:read"], "databases": ["*"]}]}`,
			wantErr: "claim is required",
		},
		{
			name:    "empty value",
			body:    `{"claim": "groups", "roles": [{"value": "", "scopes": ["db:read"], "databases": ["*"]}]}`,
			wantErr: "value is required",
		},
		{
			name:    "duplicate value",
			body:    `{"claim": "groups", "roles": [{"value": "a", "scopes": ["db:read"], "databases": ["*"]}, {"value": "a", "scopes": ["db:write"], "databases": ["*"]}]}`,
			wantErr: `OIDC role "a": duplicate value`,
		},
		{
			name:    "invalid grant",
			body:    `{"claim": "groups", "roles": [{"value": "a", "scopes": ["db:root"], "databases": ["*"]}]}`,
			wantErr: `OIDC role "a": unknown scope "db:root"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "roles.json")
			require.NoError(t, os.WriteFile(path, []byte(tc.body), 0o600))
			_, err := LoadOIDCRoles(path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// fakeOIDCIssuer serves OIDC discovery and a JWKS for one RSA key, so a test
// can mint ID tokens that go-oidc verifies end to end.
type fakeOIDCIssuer struct {
	server *httptest.Server
	key    *rsa.PrivateKey
}

func newFakeOIDCIssuer(t *testing.T) *fakeOIDCIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer := &fakeOIDCIssuer{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeTestJSON(w, map[string]any{
			"issuer":                                issuer.server.URL,
			"jwks_uri":                              issuer.server.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		writeTestJSON(w, map[string]any{"keys": []map[string]any{{
			"kty": "RSA",
			"kid": "test-key",
			"alg": "RS256",
			"use": "sig",
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	issuer.server = httptest.NewServer(mux)
	t.Cleanup(issuer.server.Close)
	return issuer
}

// token mints an RS256 ID token for subject user-1 with the extra claims.
func (f *fakeOIDCIssuer) token(t *testing.T, extra map[string]any) string {
	t.Helper()
	now := time.Now()
	claims := map[string]any{
		"iss": f.server.URL,
		"aud": testOIDCAudience,
		"sub": "user-1",
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}
	for k, v := range extra {
		claims[k] = v
	}
	signingInput := testJWTSegment(t, map[string]any{"alg": "RS256", "kid": "test-key", "typ": "JWT"}) + "." + testJWTSegment(t, claims)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func testJWTSegment(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(data)
}

func writeTestJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func TestOIDCVerifier_AppliesRoleGrant(t *testing.T) {
	issuer := newFakeOIDCIssuer(t)
	ctx := context.Background()
	verifier, err := NewOIDCVerifier(ctx, issuer.server.URL, testOIDCAudience, loadTestRoles(t))
	require.NoError(t, err)

	info, err := verifier(ctx, issuer.token(t, map[string]any{"groups": []string{"db-analysts"}, "scope": "db:admin"}), nil)
	require.NoError(t, err)
	assert.Equal(t, "user-1", info.UserID)
	assert.Equal(t, []string{"db:read"}, info.Scopes, "the role grant replaces the scope claim")
	assert.Equal(t, []string{"sales"}, info.Extra["databases"])
	assert.Equal(t, 50000.0, info.Extra["max_query_cost"])

	info, err = verifier(ctx, issuer.token(t, map[string]any{"groups": []string{"contractors"}}), nil)
	require.NoError(t, err)
	assert.Empty(t, info.Scopes)
	assert.Equal(t, []string{}, info.Extra["databases"])
}
