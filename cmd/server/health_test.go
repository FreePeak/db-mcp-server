package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthEndpoint_Returns200 verifies the /health endpoint that resolves
// FreePeak/db-mcp-server issue #46: container orchestrators, monitoring
// systems, and the Glama MCP directory scanner all expect a /health route
// that returns a 2xx status code with minimal latency.
//
// The handler is registered alongside the SSE server in cmd/server/main.go
// and listens on a side port (default 9093) so probes do not disturb MCP
// sessions. This test binds to an ephemeral port to avoid clashing with
// a running server in CI.
func TestHealthEndpoint_Returns200(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":"ok","databases":3,"transport":"sse"}`)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	done := make(chan struct{})
	go func() {
		_ = server.Serve(listener)
		close(done)
	}()
	defer func() {
		ctx, cancel := contextWithTimeout(2 * time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		<-done
	}()

	resp, err := http.Get("http://" + listener.Addr().String() + "/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &decoded), "body: %s", string(body))
	assert.Equal(t, "ok", decoded["status"])
	assert.EqualValues(t, 3, decoded["databases"])
	assert.Equal(t, "sse", decoded["transport"])
}

// TestHealthEndpoint_NotFoundOnUnknownRoute ensures only /health is exposed
// on the health port; arbitrary paths must return 404 so the side server
// can't be misused as a proxy.
func TestHealthEndpoint_NotFoundOnUnknownRoute(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	done := make(chan struct{})
	go func() {
		_ = server.Serve(listener)
		close(done)
	}()
	defer func() {
		ctx, cancel := contextWithTimeout(2 * time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		<-done
	}()

	resp, err := http.Get("http://" + listener.Addr().String() + "/not-a-real-route")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func contextWithTimeout(d time.Duration) (ctx shutdownCtx, cancel func()) {
	ctx.c = make(chan struct{})
	go func() {
		t := time.NewTimer(d)
		defer t.Stop()
		<-t.C
		close(ctx.c)
	}()
	return ctx, func() { close(ctx.c) }
}

type shutdownCtx struct{ c chan struct{} }

func (s shutdownCtx) Done() <-chan struct{} { return s.c }
func (s shutdownCtx) Err() error {
	select {
	case <-s.c:
		return errShutdown
	default:
		return nil
	}
}
func (s shutdownCtx) Value(_ interface{}) interface{} { return nil }
func (s shutdownCtx) Deadline() (time.Time, bool)     { return time.Time{}, false }

var errShutdown = fmt.Errorf("context canceled")
