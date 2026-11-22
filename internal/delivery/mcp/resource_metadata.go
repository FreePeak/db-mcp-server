package mcp

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	sdkAuth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// protectedResourceMetadataPath is the RFC 9728 section 3.1 well-known prefix.
const protectedResourceMetadataPath = "/.well-known/oauth-protected-resource"

// resourceScopesSupported is advertised in the metadata document.
var resourceScopesSupported = []string{"db:read", "db:write", "db:admin"}

// ResourceMetadataSettings describes this server as an OAuth 2.1 protected
// resource (MCP authorization spec 2025-11-25).
type ResourceMetadataSettings struct {
	// ResourceURL is the canonical URL of the /mcp endpoint (DB_MCP_RESOURCE_URL).
	ResourceURL string
	// Issuer is the authorization server issuer (DB_MCP_OIDC_ISSUER).
	Issuer string
	// MetadataPath is where the metadata document is served (RFC 9728 section 3.1).
	MetadataPath string
	// MetadataURL is the absolute metadata URL placed in WWW-Authenticate.
	MetadataURL string
}

// ResourceMetadataSettingsFromEnv reads DB_MCP_RESOURCE_URL and
// DB_MCP_OIDC_ISSUER. It is enabled only when both are set.
func ResourceMetadataSettingsFromEnv(getenv func(string) string) (ResourceMetadataSettings, bool, error) {
	resourceURL := strings.TrimSpace(getenv("DB_MCP_RESOURCE_URL"))
	issuer := strings.TrimSpace(getenv("DB_MCP_OIDC_ISSUER"))
	if resourceURL == "" {
		return ResourceMetadataSettings{}, false, nil
	}
	if issuer == "" {
		return ResourceMetadataSettings{}, false, errors.New("DB_MCP_RESOURCE_URL requires DB_MCP_OIDC_ISSUER: MCP clients cannot discover an authorization server without it")
	}
	u, err := url.Parse(resourceURL)
	if err != nil {
		return ResourceMetadataSettings{}, false, fmt.Errorf("DB_MCP_RESOURCE_URL is not a valid URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ResourceMetadataSettings{}, false, fmt.Errorf("DB_MCP_RESOURCE_URL must be an absolute http or https URL, got %q", resourceURL)
	}
	metadataPath := protectedResourceMetadataPath + u.EscapedPath()
	return ResourceMetadataSettings{
		ResourceURL:  resourceURL,
		Issuer:       issuer,
		MetadataPath: metadataPath,
		MetadataURL:  u.Scheme + "://" + u.Host + metadataPath,
	}, true, nil
}

// ResourceMetadataHandler serves the RFC 9728 protected resource metadata
// document. The MCP SDK handler provides GET-only semantics and CORS headers
// so browser-based MCP clients can read it.
func ResourceMetadataHandler(resourceURL, issuer string) http.Handler {
	return sdkAuth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               resourceURL,
		AuthorizationServers:   []string{issuer},
		ScopesSupported:        resourceScopesSupported,
		BearerMethodsSupported: []string{"header"},
	})
}

// WithBearerChallenge adds a WWW-Authenticate challenge that points clients at
// the protected resource metadata (RFC 9728 section 5.1) to 401 and 403
// responses from next. The SDK challenge is not used: it is unquoted and has
// no scope parameter.
func WithBearerChallenge(next http.Handler, metadataURL string, scopes []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&challengeWriter{ResponseWriter: w, metadataURL: metadataURL, scopes: scopes}, r)
	})
}

// challengeWriter sets the challenge when the status is first written, so the
// header goes out with the 401/403. An implicit 200 from Write leaves it unset.
type challengeWriter struct {
	http.ResponseWriter
	metadataURL string
	scopes      []string
	wroteHeader bool
}

func (w *challengeWriter) WriteHeader(code int) {
	if code >= 200 && !w.wroteHeader {
		w.wroteHeader = true
		switch code {
		case http.StatusUnauthorized:
			w.Header().Set("WWW-Authenticate", bearerChallenge(w.metadataURL, w.scopes, false))
		case http.StatusForbidden:
			w.Header().Set("WWW-Authenticate", bearerChallenge(w.metadataURL, w.scopes, true))
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *challengeWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Flush keeps Streamable HTTP responses streaming through the wrapper.
func (w *challengeWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *challengeWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func bearerChallenge(metadataURL string, scopes []string, insufficientScope bool) string {
	challenge := `Bearer resource_metadata="` + metadataURL + `"`
	if len(scopes) > 0 {
		challenge += `, scope="` + strings.Join(scopes, " ") + `"`
	}
	if insufficientScope {
		challenge += `, error="insufficient_scope"`
	}
	return challenge
}

// MountStreamableRoutes registers the streamable HTTP endpoint at /mcp. When
// DB_MCP_RESOURCE_URL is set it also serves the metadata document outside the
// auth boundary and adds bearer challenges to /mcp. Without it, /mcp is
// registered unchanged.
func MountStreamableRoutes(mux *http.ServeMux, getenv func(string) string, scopes []string, mcpHandler http.Handler) error {
	settings, enabled, err := ResourceMetadataSettingsFromEnv(getenv)
	if err != nil {
		return err
	}
	if !enabled {
		if strings.TrimSpace(getenv("DB_MCP_OIDC_ISSUER")) != "" {
			logger.Warn("DB_MCP_RESOURCE_URL is not set: MCP clients cannot discover the authorization server from this endpoint")
		}
		mux.Handle("/mcp", mcpHandler)
		return nil
	}
	metadata := ResourceMetadataHandler(settings.ResourceURL, settings.Issuer)
	mux.Handle(protectedResourceMetadataPath, metadata)
	if settings.MetadataPath != protectedResourceMetadataPath {
		mux.Handle(settings.MetadataPath, metadata)
	}
	mux.Handle("/mcp", WithBearerChallenge(mcpHandler, settings.MetadataURL, scopes))
	return nil
}
