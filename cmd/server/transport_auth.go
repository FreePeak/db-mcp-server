package main

import "errors"

// validateTransportAuth refuses to start a transport that would serve clients
// without authentication. Streamable HTTP needs a static key (API key or agent
// keys file) or OIDC; legacy SSE has no built-in authentication and needs the
// explicit -allow-insecure-sse opt-in. Other transports, including unknown
// values, are not checked here.
func validateTransportAuth(transport string, hasStaticKeys, hasOIDC, allowInsecureSSE bool) error {
	switch transport {
	case "streamable":
		if !hasStaticKeys && !hasOIDC {
			return errors.New("streamable HTTP requires OIDC configuration or DB_MCP_API_KEY or DB_MCP_API_KEY_FILE or DB_MCP_AGENT_KEYS_FILE; refusing to start unauthenticated")
		}
	case "sse":
		if !allowInsecureSSE {
			return errors.New("legacy SSE is disabled by default because it has no built-in authentication; use STDIO or implement the Streamable transport, or pass -allow-insecure-sse only behind a trusted private proxy")
		}
	}
	return nil
}
