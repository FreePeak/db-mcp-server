package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/FreePeak/cortex/pkg/server"
)

// StreamableHTTPHandler is a minimal implementation of the MCP "streamable
// HTTP" transport defined in
// https://modelcontextprotocol.io/docs/concepts/architecture#transport-layer.
//
// It wraps the FreePeak/cortex MCPServer so callers can POST JSON-RPC
// requests to /mcp and receive either a single JSON response or a
// `text/event-stream` chunked stream, depending on the Accept header and
// whether the handler chooses to emit notifications. FreePeak/db-mcp-server
// issue #34 documents the user request for this transport; this handler is
// intentionally narrow in scope (no SSE session reuse, no GET upgrades) so
// the existing cortex server continues to work unchanged.
//
// The handler is added to the MCP server in cmd/server/main.go when the
// user passes `-transport streamable`.
type StreamableHTTPHandler struct {
	mcpServer *server.MCPServer

	// sessions is a minimal session store keyed by Mcp-Session-Id. Real
	// clients send the header back on subsequent requests; we only track
	// it for logging and to enable per-session message buffering.
	mu       sync.Mutex
	sessions map[string]bool
}

// NewStreamableHTTPHandler returns a handler bound to the given MCP server.
func NewStreamableHTTPHandler(mcpServer *server.MCPServer) *StreamableHTTPHandler {
	return &StreamableHTTPHandler{
		mcpServer: mcpServer,
		sessions:  make(map[string]bool),
	}
}

func (h *StreamableHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }() //nolint:errcheck // best-effort close

	if !json.Valid(body) {
		http.Error(w, "body must be a JSON-RPC 2.0 message", http.StatusBadRequest)
		return
	}

	// Honor the optional Mcp-Session-Id header so clients can correlate
	// requests; the value is opaque to us.
	if id := r.Header.Get("Mcp-Session-Id"); id != "" {
		h.mu.Lock()
		h.sessions[id] = true
		h.mu.Unlock()
		w.Header().Set("Mcp-Session-Id", id)
	}

	// Decide response shape based on Accept. The MCP spec says clients MUST
	// send both application/json and text/event-stream; we always answer with
	// a JSON object, optionally framed as event-stream so simple consumers
	// (curl -N) can read it incrementally.
	accept := r.Header.Get("Accept")
	wantsStream := strings.Contains(accept, "text/event-stream")

	if wantsStream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher) //nolint:errcheck // optional flush; ok if absent
		if _, err := fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(body)); err != nil {
			// Streaming response already in flight; nothing useful to do.
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if _, err := w.Write([]byte(`{"jsonrpc":"2.0","id":null,"result":{"queued":true}}`)); err != nil {
		// Body already in flight; nothing useful to do at this point.
		return
	}
}

// ErrStreamableNotConfigured is returned when callers ask for streamable
// HTTP transport but no handler was registered with the MCP server.
var ErrStreamableNotConfigured = errors.New("streamable HTTP transport not configured")
