package mcp

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandler() *StreamableHTTPHandler {
	return &StreamableHTTPHandler{
		sessions: make(map[string]bool),
	}
}

// TestStreamableHTTPHandler_RejectsNonPost ensures the streamable HTTP
// transport only accepts POST per the MCP specification. FreePeak/db-mcp-server
// issue #34 asks for this transport; the contract is documented at
// https://modelcontextprotocol.io/docs/concepts/architecture#transport-layer.
func TestStreamableHTTPHandler_RejectsNonPost(t *testing.T) {
	h := newTestHandler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// TestStreamableHTTPHandler_RejectsInvalidJSON covers the JSON validation
// step. The streamable transport must reject bodies that are not
// well-formed JSON-RPC so the underlying MCP server never sees garbage.
func TestStreamableHTTPHandler_RejectsInvalidJSON(t *testing.T) {
	h := newTestHandler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte("not json")))
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "JSON-RPC")
}

// TestStreamableHTTPHandler_QueuesJSONResponse exercises the non-streaming
// path: when the client only accepts application/json, the handler responds
// with a tiny JSON envelope acknowledging that the message was received.
// Real streaming responses require a long-lived MCP session; the smoke test
// here only verifies the entry point is wired correctly.
func TestStreamableHTTPHandler_QueuesJSONResponse(t *testing.T) {
	h := newTestHandler()

	rec := httptest.NewRecorder()
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Accept", "application/json")
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	respBody, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.Contains(t, string(respBody), `"queued":true`)
}

// TestStreamableHTTPHandler_StreamsWhenEventStreamAccepted verifies the
// streaming path emits text/event-stream frames that contain the original
// JSON-RPC body so consumers can read the response incrementally.
func TestStreamableHTTPHandler_StreamsWhenEventStreamAccepted(t *testing.T) {
	h := newTestHandler()

	rec := httptest.NewRecorder()
	body := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Accept", "application/json, text/event-stream")
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
	out, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	frame := string(out)
	assert.True(t, strings.HasPrefix(frame, "event: message\ndata: "),
		"expected SSE-style framing, got: %q", frame)
	assert.Contains(t, frame, `"method":"tools/list"`)
}

// TestStreamableHTTPHandler_PropagatesSessionID ensures the Mcp-Session-Id
// header is echoed back when supplied so clients can correlate requests.
func TestStreamableHTTPHandler_PropagatesSessionID(t *testing.T) {
	h := newTestHandler()

	rec := httptest.NewRecorder()
	body := []byte(`{"jsonrpc":"2.0","id":3,"method":"ping"}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Mcp-Session-Id", "session-abc-123")
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "session-abc-123", rec.Header().Get("Mcp-Session-Id"))
}
