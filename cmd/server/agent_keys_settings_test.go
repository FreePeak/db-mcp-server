package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentKeysSettings(t *testing.T) {
	tests := []struct {
		name      string
		transport string
		oidc      string
		path      string
		unified   bool
		wantPath  string
		wantWarn  string
		wantErr   string
	}{
		{name: "unset", transport: "streamable", unified: false},
		{name: "whitespace-only path is unset", transport: "streamable", path: "   "},
		{name: "streamable uses trimmed path", transport: "streamable", path: " keys.json ", unified: true, wantPath: "keys.json"},
		{name: "sse ignores file with warning", transport: "sse", path: "keys.json", wantWarn: "DB_MCP_AGENT_KEYS_FILE is ignored"},
		{name: "stdio ignores file with warning", transport: "stdio", path: "keys.json", wantWarn: "DB_MCP_AGENT_KEYS_FILE is ignored"},
		{name: "oidc conflicts with file on streamable", transport: "streamable", oidc: "test-issuer", path: "keys.json", unified: true, wantErr: "cannot be combined with DB_MCP_OIDC_ISSUER"},
		{name: "oidc without file is fine", transport: "streamable", oidc: "test-issuer"},
		{name: "oidc and file on stdio only warns", transport: "stdio", oidc: "test-issuer", path: "keys.json", wantWarn: "DB_MCP_AGENT_KEYS_FILE is ignored"},
		{name: "file on streamable without unified tools is refused", transport: "streamable", path: "keys.json", wantErr: "requires the -unified-tools flag"},
		{name: "file on stdio without unified tools only warns", transport: "stdio", path: "keys.json", wantWarn: "DB_MCP_AGENT_KEYS_FILE is ignored"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path, warn, err := agentKeysSettings(tc.transport, tc.oidc, tc.path, tc.unified)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantPath, path)
			if tc.wantWarn == "" {
				assert.Empty(t, warn)
			} else {
				assert.Contains(t, warn, tc.wantWarn)
			}
		})
	}
}

func TestWriteAgentKeysWarning(t *testing.T) {
	tests := []struct {
		name    string
		warning string
		want    string
	}{
		{name: "empty warning writes nothing", warning: "", want: ""},
		{name: "warning is newline-terminated", warning: "DB_MCP_AGENT_KEYS_FILE is ignored", want: "DB_MCP_AGENT_KEYS_FILE is ignored\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeAgentKeysWarning(&buf, tc.warning)
			assert.Equal(t, tc.want, buf.String())
		})
	}
}

func TestOIDCRolesSettings(t *testing.T) {
	tests := []struct {
		name     string
		oidc     string
		path     string
		unified  bool
		wantPath string
		wantErr  string
	}{
		{name: "unset"},
		{name: "whitespace-only path is unset", oidc: "test-issuer", path: "   "},
		{name: "roles with issuer uses trimmed path", oidc: "test-issuer", path: " roles.json ", unified: true, wantPath: "roles.json"},
		{name: "roles without issuer is refused", path: "roles.json", wantErr: "DB_MCP_OIDC_ROLES_FILE is set but DB_MCP_OIDC_ISSUER is empty"},
		{name: "roles without unified tools is refused", oidc: "test-issuer", path: "roles.json", wantErr: "requires the -unified-tools flag"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path, err := oidcRolesSettings(tc.oidc, tc.path, tc.unified)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				assert.Contains(t, err.Error(), "DB_MCP_OIDC_ROLES_FILE")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantPath, path)
		})
	}
}
