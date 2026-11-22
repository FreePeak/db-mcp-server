package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// newHealthHandler serves the unauthenticated /health probe for the HTTP
// transports (SSE and Streamable). It reports liveness, the configured database
// count and the transport name only.
func newHealthHandler(databases int, transport string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprintf(w, `{"status":"ok","databases":%d,"transport":"%s"}`,
			databases, transport); err != nil {
			logger.Warn("health response write failed: %v", err)
		}
	})
	return mux
}

// startHealthServer serves handler on host:port in the background so probes
// never share the MCP listener. It returns nil when port is 0 (disabled).
func startHealthServer(host string, port int, handler http.Handler) *http.Server {
	if port <= 0 {
		return nil
	}
	srv := &http.Server{
		Addr:              net.JoinHostPort(host, strconv.Itoa(port)),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info("Starting health endpoint on :%d/health", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Warn("health endpoint error: %v", err)
		}
	}()
	return srv
}
