package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandler() *StreamableHTTPHandler {
	return NewStreamableHTTPHandler(sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}, nil))
}

func postStreamable(t *testing.T, handler http.Handler, body string, sessionID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func initializeBody(id int) string {
	return `{"jsonrpc":"2.0","id":` + strconv.Itoa(id) + `,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
}

func TestStreamableHTTPHandler_RejectsInvalidJSON(t *testing.T) {
	rec := postStreamable(t, newTestHandler(), "not json", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestStreamableHTTPHandler_RejectsDisallowedOrigin(t *testing.T) {
	t.Setenv("MCP_ALLOWED_ORIGINS", "https://allowed.example")
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(initializeBody(1)))
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestStreamableHTTPHandler_AllowsConfiguredOrigin(t *testing.T) {
	t.Setenv("MCP_ALLOWED_ORIGINS", "https://allowed.example")
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(initializeBody(1)))
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://allowed.example")
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestStreamableHTTPHandler_RejectsOversizedBody(t *testing.T) {
	body := append([]byte(`{"jsonrpc":"2.0","id":1,"method":"`), bytes.Repeat([]byte("a"), maxStreamableRequestBody+1)...)
	rec := postStreamable(t, newTestHandler(), string(body), "")
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestStreamableHTTPHandler_InitializeCreatesSession(t *testing.T) {
	rec := postStreamable(t, newTestHandler(), initializeBody(1), "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	assert.NotEmpty(t, rec.Header().Get("Mcp-Session-Id"))

	var response struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
			ServerInfo      struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	assert.Equal(t, "2025-06-18", response.Result.ProtocolVersion)
	assert.Equal(t, "test-server", response.Result.ServerInfo.Name)
}

func TestStreamableHTTPHandler_InitializedNotificationReturnsAccepted(t *testing.T) {
	handler := newTestHandler()
	initialize := postStreamable(t, handler, initializeBody(1), "")
	require.Equal(t, http.StatusOK, initialize.Code)
	sessionID := initialize.Header().Get("Mcp-Session-Id")
	require.NotEmpty(t, sessionID)

	rec := postStreamable(t, handler, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, sessionID)
	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Empty(t, rec.Body.Bytes())
}

func TestStreamableHTTPHandler_RejectsNonJSONContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(initializeBody(1)))
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
}

func TestStreamableHTTPHandler_APIKeyMiddlewareRejectsMissingKey(t *testing.T) {
	handler := APIKeyAuth("secret")(newTestHandler())
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(initializeBody(1)))
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestStreamableHTTPHandler_RequiresBothAcceptTypes(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(initializeBody(1)))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestStreamableHTTPHandler_DeleteSession(t *testing.T) {
	handler := newTestHandler()
	initialize := postStreamable(t, handler, initializeBody(1), "")
	require.Equal(t, http.StatusOK, initialize.Code)
	sessionID := initialize.Header().Get("Mcp-Session-Id")
	require.NotEmpty(t, sessionID)

	req := httptest.NewRequest(http.MethodDelete, "/mcp", nil)
	req.Header.Set("Mcp-Session-Id", sessionID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestStreamableHTTPHandler_RejectsUnknownSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/mcp", nil)
	req.Header.Set("Mcp-Session-Id", "unknown-session")
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
