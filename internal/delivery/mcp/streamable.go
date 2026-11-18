package mcp

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

const maxStreamableRequestBody = 1 << 20

// StreamableHTTPHandler applies the deployment-level Origin and body-size
// checks, then delegates protocol semantics to the maintained MCP SDK.
type StreamableHTTPHandler struct {
	handler http.Handler
}

// NewStreamableHTTPHandler returns a bounded wrapper around the official SDK
// Streamable HTTP handler. The same SDK server is populated by ServerWrapper
// as tools are registered.
func NewStreamableHTTPHandler(sdkServer *sdkmcp.Server) *StreamableHTTPHandler {
	return &StreamableHTTPHandler{
		handler: sdkmcp.NewStreamableHTTPHandler(
			func(*http.Request) *sdkmcp.Server { return sdkServer },
			&sdkmcp.StreamableHTTPOptions{
				JSONResponse:          true,
				SessionTimeout:        5 * time.Minute,
				CrossOriginProtection: crossOriginProtection(),
			},
		),
	}
}

// crossOriginProtection makes the SDK trust the same browser origins that
// originAllowed admits. Without it, SDK 1.4.1 rejects allowed cross-origin
// POSTs with 403. Invalid entries are left untrusted (fail closed).
func crossOriginProtection() *http.CrossOriginProtection {
	protection := &http.CrossOriginProtection{}
	for _, origin := range allowedOrigins() {
		if err := protection.AddTrustedOrigin(origin); err != nil {
			logger.Warn("Ignoring MCP_ALLOWED_ORIGINS entry '%s': %v", origin, err)
		}
	}
	return protection
}

func (h *StreamableHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !originAllowed(r) {
		http.Error(w, "origin is not allowed", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodPost {
		contentType := r.Header.Get("Content-Type")
		if contentType == "" {
			http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		// SDK 1.4.1 compares Content-Type byte-for-byte, so drop parameters
		// such as charset=utf-8 that the check above has already accepted.
		r.Header.Set("Content-Type", "application/json")
		if r.Body != nil {
			body, readErr := io.ReadAll(http.MaxBytesReader(w, r.Body, maxStreamableRequestBody))
			closeErr := r.Body.Close()
			if readErr != nil {
				var maxErr *http.MaxBytesError
				if errors.As(readErr, &maxErr) {
					http.Error(w, "request body is too large", http.StatusRequestEntityTooLarge)
					return
				}
				http.Error(w, "read body: "+readErr.Error(), http.StatusBadRequest)
				return
			}
			if closeErr != nil {
				http.Error(w, "close request body: "+closeErr.Error(), http.StatusInternalServerError)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
	}
	if h.handler == nil {
		http.Error(w, "streamable handler is not configured", http.StatusNotImplemented)
		return
	}
	h.handler.ServeHTTP(w, r)
}

func originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	return slices.Contains(allowedOrigins(), origin)
}

func allowedOrigins() []string {
	var origins []string
	for _, allowed := range strings.Split(os.Getenv("MCP_ALLOWED_ORIGINS"), ",") {
		if origin := strings.TrimSpace(allowed); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}
