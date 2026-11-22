package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateTransportAuth(t *testing.T) {
	tests := []struct {
		name             string
		transport        string
		hasAPIKey        bool
		hasOIDC          bool
		allowInsecureSSE bool
		wantErr          string
	}{
		{
			name:      "streamable without key or OIDC is refused",
			transport: "streamable",
			wantErr:   "DB_MCP_AGENT_KEYS_FILE",
		},
		{
			name:      "streamable with API key",
			transport: "streamable",
			hasAPIKey: true,
		},
		{
			name:      "streamable with OIDC",
			transport: "streamable",
			hasOIDC:   true,
		},
		{
			name:      "streamable with both API key and OIDC",
			transport: "streamable",
			hasAPIKey: true,
			hasOIDC:   true,
		},
		{
			name:      "sse without opt-in is refused",
			transport: "sse",
			wantErr:   "legacy SSE is disabled",
		},
		{
			name:             "sse with opt-in",
			transport:        "sse",
			allowInsecureSSE: true,
		},
		{
			name:      "stdio needs no auth",
			transport: "stdio",
		},
		{
			name:      "unknown transport is not refused here",
			transport: "bogus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransportAuth(tt.transport, tt.hasAPIKey, tt.hasOIDC, tt.allowInsecureSSE)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
