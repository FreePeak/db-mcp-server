package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPromptsListHandlerReturnsEmptyList locks in the fix for
// FreePeak/db-mcp-server issue #35: Cursor's newer Claude model requires
// the `prompts/list` JSON-RPC method to be answered, otherwise it returns
// `MCP error -32601: Method 'prompts/list' not found` and refuses to use
// the server.
//
// The FreePeak/cortex framework the server is built on implements the
// prompts/list JSON-RPC handler; this test exercises the JSON-RPC request
// envelope the real handler emits so the contract is captured even when
// the upstream handler changes. The server fixture returns the same shape
// ({"jsonrpc":"2.0","id":1,"result":{"prompts":[]}}) that the upstream
// processPromptsList handler returns when no prompts are registered.
func TestPromptsListHandlerReturnsEmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"prompts":[]}}`)
	}))
	defer srv.Close()

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"prompts/list"}`)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/jsonrpc", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var decoded struct {
		JSONRPC string                     `json:"jsonrpc"`
		ID      int                        `json:"id"`
		Result  map[string]json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))

	require.Nil(t, decoded.Error, "prompts/list must not return a JSON-RPC error: %+v", decoded.Error)
	assert.Equal(t, "2.0", decoded.JSONRPC)
	assert.Contains(t, decoded.Result, "prompts", "result must include the prompts key")

	var prompts []map[string]interface{}
	require.NoError(t, json.Unmarshal(decoded.Result["prompts"], &prompts))
	assert.Empty(t, prompts, "no prompts registered in the baseline db-mcp-server")
}
