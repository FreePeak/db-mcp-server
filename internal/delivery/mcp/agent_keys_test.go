package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cortexServer "github.com/FreePeak/cortex/pkg/server"
	cortexTools "github.com/FreePeak/cortex/pkg/tools"
	sdkAuth "github.com/modelcontextprotocol/go-sdk/auth"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FreePeak/db-mcp-server/internal/usecase"
)

// Fake literals only: these are test fixtures, not credentials.
const (
	testAgentKeyA = "agent-a-fake-test-key"
	testAgentKeyB = "agent-b-fake-test-key"
	testAgentKeyC = "agent-c-fake-test-key"
	testLegacyKey = "legacy-fake-test-key"
)

func agentKeyDigest(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func testAgentKeys() []AgentKey {
	return []AgentKey{
		{ID: "agent-a", KeySHA256: agentKeyDigest(testAgentKeyA), Scopes: []string{"db:read"}, Databases: []string{"sales"}},
		{ID: "agent-b", KeySHA256: agentKeyDigest(testAgentKeyB), Scopes: []string{"db:read", "db:write"}, Databases: []string{"prod"}},
		{ID: "agent-c", KeySHA256: agentKeyDigest(testAgentKeyC), Scopes: []string{"db:admin"}, Databases: []string{"*"}},
	}
}

// serveWithAuth sends one request carrying authHeader ("" means no header)
// through the middleware. It returns the status and the TokenInfo that the
// wrapped handler observed, or nil when the handler did not run.
func serveWithAuth(t *testing.T, mw func(http.Handler) http.Handler, authHeader string) (int, *sdkAuth.TokenInfo) {
	t.Helper()
	var seen *sdkAuth.TokenInfo
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = sdkAuth.TokenInfoFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, seen
}

func TestAgentKeyAuth_RightKeyYieldsEntryIdentity(t *testing.T) {
	mw, err := AgentKeyAuth(testAgentKeys(), "", nil)
	require.NoError(t, err)

	tests := []struct {
		name          string
		key           string
		wantID        string
		wantScopes    []string
		wantDatabases []string
	}{
		// Each position in the list must match, not only the first entry.
		{"first entry", testAgentKeyA, "agent-a", []string{"db:read"}, []string{"sales"}},
		{"middle entry", testAgentKeyB, "agent-b", []string{"db:read", "db:write"}, []string{"prod"}},
		{"last entry", testAgentKeyC, "agent-c", []string{"db:admin"}, []string{"*"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, info := serveWithAuth(t, mw, "Bearer "+tc.key)
			require.Equal(t, http.StatusNoContent, code)
			require.NotNil(t, info, "TokenInfo must reach the handler")
			assert.Equal(t, tc.wantID, info.UserID)
			assert.Equal(t, tc.wantScopes, info.Scopes)
			assert.Equal(t, tc.wantDatabases, info.Extra["databases"])
			assert.False(t, info.Expiration.IsZero(), "SDK rejects tokens without expiration")
		})
	}
}

func TestAgentKeyAuth_RejectsWrongKey(t *testing.T) {
	mw, err := AgentKeyAuth(testAgentKeys(), "", nil)
	require.NoError(t, err)

	tests := []struct {
		name   string
		header string
	}{
		{"unknown key", "Bearer agent-d-fake-test-key"},
		{"near miss", "Bearer " + testAgentKeyA + "x"},
		{"empty bearer", "Bearer "},
		{"missing prefix", testAgentKeyA},
		{"missing header", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, info := serveWithAuth(t, mw, tc.header)
			assert.Equal(t, http.StatusUnauthorized, code)
			assert.Nil(t, info, "handler must not run for a rejected key")
		})
	}
}

func TestAgentKeyAuth_LegacyKeyStillWorks(t *testing.T) {
	mw, err := AgentKeyAuth(testAgentKeys(), testLegacyKey, []string{"db:read", "db:write"})
	require.NoError(t, err)

	code, info := serveWithAuth(t, mw, "Bearer "+testLegacyKey)
	require.Equal(t, http.StatusNoContent, code)
	require.NotNil(t, info)
	assert.Equal(t, "api-key", info.UserID)
	assert.Equal(t, []string{"db:read", "db:write"}, info.Scopes)

	code, _ = serveWithAuth(t, mw, "Bearer "+testAgentKeyA+"x")
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestAgentKeyAuth_NoLegacyKeyRejectsEmptyBearer(t *testing.T) {
	mw, err := AgentKeyAuth(testAgentKeys(), "", nil)
	require.NoError(t, err)

	code, info := serveWithAuth(t, mw, "Bearer ")
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Nil(t, info, "an empty legacy key must never match an empty bearer")
}

func TestLoadAgentKeys_StartupValidation(t *testing.T) {
	validDigest := agentKeyDigest(testAgentKeyA)
	otherDigest := agentKeyDigest(testAgentKeyB)

	tests := []struct {
		name    string
		body    string
		wantErr string // substring; empty means the file must load
		wantIDs []string
	}{
		{
			name:    "valid file loads",
			body:    `[{"id":"agent-a","key_sha256":"` + validDigest + `","scopes":["db:read"],"databases":["sales"]},{"id":"agent-b","key_sha256":"` + otherDigest + `","scopes":["db:read","db:write"],"databases":["*"]}]`,
			wantIDs: []string{"agent-a", "agent-b"},
		},
		{
			name:    "missing id",
			body:    `[{"key_sha256":"` + validDigest + `","scopes":["db:read"],"databases":["sales"]}]`,
			wantErr: "id is required",
		},
		{
			name:    "blank id",
			body:    `[{"id":"   ","key_sha256":"` + validDigest + `","scopes":["db:read"],"databases":["sales"]}]`,
			wantErr: "id is required",
		},
		{
			name:    "id with trailing whitespace",
			body:    `[{"id":"agent-a ","key_sha256":"` + validDigest + `","scopes":["db:read"],"databases":["sales"]}]`,
			wantErr: `agent key "agent-a ": id must not have leading or trailing whitespace`,
		},
		{
			name:    "id with leading whitespace",
			body:    `[{"id":" agent-a","key_sha256":"` + validDigest + `","scopes":["db:read"],"databases":["sales"]}]`,
			wantErr: `agent key " agent-a": id must not have leading or trailing whitespace`,
		},
		{
			name:    "missing databases",
			body:    `[{"id":"agent-a","key_sha256":"` + validDigest + `","scopes":["db:read"]}]`,
			wantErr: `agent key "agent-a": databases is required`,
		},
		{
			name:    "null databases",
			body:    `[{"id":"agent-a","key_sha256":"` + validDigest + `","scopes":["db:read"],"databases":null}]`,
			wantErr: `agent key "agent-a": databases is required`,
		},
		{
			name:    "empty databases",
			body:    `[{"id":"agent-a","key_sha256":"` + validDigest + `","scopes":["db:read"],"databases":[]}]`,
			wantErr: `agent key "agent-a": databases is required`,
		},
		{
			name:    "non-string database",
			body:    `[{"id":"agent-a","key_sha256":"` + validDigest + `","scopes":["db:read"],"databases":[1]}]`,
			wantErr: "parse agent keys file",
		},
		{
			name:    "empty list file",
			body:    `[]`,
			wantErr: "agent keys file contains no entries",
		},
		{
			name:    "duplicate id",
			body:    `[{"id":"agent-a","key_sha256":"` + validDigest + `","scopes":["db:read"],"databases":["sales"]},{"id":"agent-a","key_sha256":"` + otherDigest + `","scopes":["db:read"],"databases":["sales"]}]`,
			wantErr: `agent key "agent-a": duplicate id`,
		},
		{
			name:    "duplicate hash",
			body:    `[{"id":"agent-a","key_sha256":"` + validDigest + `","scopes":["db:read"],"databases":["sales"]},{"id":"agent-b","key_sha256":"` + strings.ToUpper(validDigest) + `","scopes":["db:read"],"databases":["sales"]}]`,
			wantErr: `agent key "agent-b": duplicate key_sha256`,
		},
		{
			name:    "malformed hex",
			body:    `[{"id":"agent-a","key_sha256":"not-hex-fake-value","scopes":["db:read"]}]`,
			wantErr: `agent key "agent-a": key_sha256 must be 64 hex characters`,
		},
		{
			name:    "short hex",
			body:    `[{"id":"agent-a","key_sha256":"abcd","scopes":["db:read"]}]`,
			wantErr: "must be 64 hex characters",
		},
		{
			name:    "unknown scope",
			body:    `[{"id":"agent-a","key_sha256":"` + validDigest + `","scopes":["db:owner"]}]`,
			wantErr: `agent key "agent-a": unknown scope "db:owner"`,
		},
		{
			name:    "no scopes",
			body:    `[{"id":"agent-a","key_sha256":"` + validDigest + `"}]`,
			wantErr: `agent key "agent-a": at least one scope is required`,
		},
		{
			name:    "invalid json",
			body:    `[{"id":`,
			wantErr: "parse agent keys file",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "agent-keys.json")
			require.NoError(t, os.WriteFile(path, []byte(tc.body), 0o600))

			keys, err := LoadAgentKeys(path)
			if tc.wantErr == "" {
				require.NoError(t, err)
				var ids []string
				for _, key := range keys {
					ids = append(ids, key.ID)
				}
				assert.Equal(t, tc.wantIDs, ids)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.NotContains(t, err.Error(), "fake-value", "errors must not echo key material")
		})
	}
}

func TestLoadAgentKeys_MissingFile(t *testing.T) {
	_, err := LoadAgentKeys(filepath.Join(t.TempDir(), "absent.json"))
	require.Error(t, err)
}

func TestLoadAgentKeys_RejectsNonFileAndEmptyPaths(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.json")
	require.NoError(t, os.WriteFile(empty, nil, 0o600))

	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{name: "directory", path: dir, wantErr: "not a regular file"},
		{name: "empty file", path: empty, wantErr: "is empty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadAgentKeys(tc.path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r)
}

// TestAgentKeysEndToEndOverStreamableHTTP drives agent keys through the real
// middleware, SDK streamable handler and tool bridge over httptest. Each key
// must reach the use case with its own principal and only its own allowlist.
func TestAgentKeysEndToEndOverStreamableHTTP(t *testing.T) {
	wrapper := NewServerWrapper(cortexServer.NewMCPServer("test", "1.0.0", nil))
	var principals []string
	tool := cortexTools.NewTool("query",
		cortexTools.WithDescription("query"),
		cortexTools.WithString("database", cortexTools.Required()),
	)
	require.NoError(t, wrapper.AddTool(context.Background(), tool, func(ctx context.Context, _ cortexServer.ToolCallRequest) (interface{}, error) {
		principals = append(principals, usecase.CallerFromContext(ctx).Principal)
		return &Response{Content: []TextContent{{Type: "text", Text: "ok"}}}, nil
	}))

	mw, err := AgentKeyAuth(testAgentKeys(), "", nil)
	require.NoError(t, err)
	httpServer := httptest.NewServer(mw(NewStreamableHTTPHandler(wrapper.SDKServer())))
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	connect := func(key string) (*sdkmcp.ClientSession, error) {
		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
		return client.Connect(ctx, &sdkmcp.StreamableClientTransport{
			Endpoint:             httpServer.URL,
			HTTPClient:           &http.Client{Transport: bearerTransport{token: key, base: http.DefaultTransport}},
			DisableStandaloneSSE: true,
		}, nil)
	}
	callAs := func(key, database string) *sdkmcp.CallToolResult {
		session, err := connect(key)
		require.NoError(t, err)
		defer session.Close()
		result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name:      "query",
			Arguments: map[string]any{"database": database, "query": "SELECT 1"},
		})
		require.NoError(t, err)
		return result
	}

	_, err = connect("agent-d-fake-test-key")
	require.ErrorContains(t, err, "Unauthorized", "a wrong key must not open a session")

	require.False(t, callAs(testAgentKeyA, "sales").IsError, "agent-a may use sales")
	require.True(t, callAs(testAgentKeyA, "prod").IsError, "agent-a is denied prod")
	require.False(t, callAs(testAgentKeyB, "prod").IsError, "agent-b may use prod")
	require.True(t, callAs(testAgentKeyB, "sales").IsError, "agent-b is denied sales")
	require.False(t, callAs(testAgentKeyC, "sales").IsError, "agent-c (\"*\") may use sales")
	require.Equal(t, []string{"agent-a", "agent-b", "agent-c"}, principals, "denied calls must not reach the use case")
}

// TestAgentKey_ReadScopeCannotWriteViaQuery authenticates agent-a, which holds
// only db:read, and sends an INSERT through the query tool against an
// allowlisted database. The scope check must refuse it before the handler runs.
func TestAgentKey_ReadScopeCannotWriteViaQuery(t *testing.T) {
	mw, err := AgentKeyAuth(testAgentKeys(), "", nil)
	require.NoError(t, err)
	status, token := serveWithAuth(t, mw, "Bearer "+testAgentKeyA)
	require.Equal(t, http.StatusNoContent, status)
	require.NotNil(t, token)
	require.Equal(t, []string{"db:read"}, token.Scopes)

	calls := 0
	handler := adaptCortexHandler(func(context.Context, cortexServer.ToolCallRequest) (interface{}, error) {
		calls++
		return &Response{Content: []TextContent{{Type: "text", Text: "ok"}}}, nil
	})
	request := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{Name: "query", Arguments: json.RawMessage(`{"database":"sales","query":"INSERT INTO orders VALUES (1)"}`)},
		Extra:  &sdkmcp.RequestExtra{TokenInfo: token},
	}

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Equal(t, 0, calls, "a db:read agent key must not reach the handler for an INSERT")
	text, ok := result.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	require.Contains(t, text.Text, "lacks the required database scope")
}
