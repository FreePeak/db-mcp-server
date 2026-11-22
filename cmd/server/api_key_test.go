package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeAPIKeyFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "db_mcp_api_key")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestResolveAPIKey(t *testing.T) {
	const (
		flagKey = "test-flag-key"
		envKey  = "test-env-key"
		fileKey = "test-file-key"
	)
	missingFile := func(t *testing.T) string { return filepath.Join(t.TempDir(), "absent") }

	tests := []struct {
		name       string
		flag       string
		env        string
		file       func(t *testing.T) string
		wantKey    string
		wantSource string
		wantErr    []string
	}{
		{
			name:       "flag only",
			flag:       flagKey,
			wantKey:    flagKey,
			wantSource: apiKeySourceFlag,
		},
		{
			name:       "env only",
			env:        envKey,
			wantKey:    envKey,
			wantSource: apiKeySourceEnv,
		},
		{
			name:       "file only with trailing newline",
			file:       func(t *testing.T) string { return writeAPIKeyFile(t, fileKey+"\n") },
			wantKey:    fileKey,
			wantSource: apiKeySourceFile,
		},
		{
			name:       "file only with CRLF line ending",
			file:       func(t *testing.T) string { return writeAPIKeyFile(t, fileKey+"\r\n") },
			wantKey:    fileKey,
			wantSource: apiKeySourceFile,
		},
		{
			name:    "empty file is an error",
			file:    func(t *testing.T) string { return writeAPIKeyFile(t, "") },
			wantErr: []string{"api_key file", "empty"},
		},
		{
			name:    "missing file is an error",
			file:    missingFile,
			wantErr: []string{"api_key file"},
		},
		{
			name:    "flag and env conflict",
			flag:    flagKey,
			env:     envKey,
			wantErr: []string{apiKeySourceFlag, apiKeySourceEnv},
		},
		{
			name:    "flag and file conflict",
			flag:    flagKey,
			file:    func(t *testing.T) string { return writeAPIKeyFile(t, fileKey) },
			wantErr: []string{apiKeySourceFlag, apiKeySourceFile},
		},
		{
			name:    "env and file conflict",
			env:     envKey,
			file:    func(t *testing.T) string { return writeAPIKeyFile(t, fileKey) },
			wantErr: []string{apiKeySourceEnv, apiKeySourceFile},
		},
		{
			name:    "blank flag is rejected",
			flag:    "   ",
			wantErr: []string{apiKeySourceFlag, "blank"},
		},
		{
			name:    "blank env is rejected",
			env:     "   ",
			wantErr: []string{apiKeySourceEnv, "blank"},
		},
		{
			name:    "blank file is rejected",
			file:    func(t *testing.T) string { return writeAPIKeyFile(t, "  \n") },
			wantErr: []string{apiKeySourceFile, "blank"},
		},
		{
			name:       "none set returns empty so the startup check refuses",
			wantKey:    "",
			wantSource: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyFile := ""
			if tt.file != nil {
				keyFile = tt.file(t)
			}

			key, source, err := resolveAPIKey(tt.flag, tt.env, keyFile)

			if len(tt.wantErr) > 0 {
				require.Error(t, err)
				for _, part := range tt.wantErr {
					assert.Contains(t, err.Error(), part)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantKey, key)
			assert.Equal(t, tt.wantSource, source)
		})
	}
}

func TestResolveAPIKeyFileSizeLimit(t *testing.T) {
	const limit = 64 * 1024

	key, source, err := resolveAPIKey("", "", writeAPIKeyFile(t, strings.Repeat("k", limit)))
	require.NoError(t, err)
	assert.Len(t, key, limit)
	assert.Equal(t, apiKeySourceFile, source)

	_, _, err = resolveAPIKey("", "", writeAPIKeyFile(t, strings.Repeat("k", limit+1)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "larger than 64 KiB")
}

func TestResolveAPIKeyFollowsSymlinkChain(t *testing.T) {
	// Kubernetes secret mounts expose the key through a chain of symlinks.
	target := writeAPIKeyFile(t, "test-file-key\n")
	middle := filepath.Join(t.TempDir(), "..data")
	require.NoError(t, os.Symlink(target, middle))
	link := filepath.Join(t.TempDir(), "db_mcp_api_key")
	require.NoError(t, os.Symlink(middle, link))

	key, source, err := resolveAPIKey("", "", link)
	require.NoError(t, err)
	assert.Equal(t, "test-file-key", key)
	assert.Equal(t, apiKeySourceFile, source)
}

func TestResolveAPIKeyErrorsDoNotEchoKey(t *testing.T) {
	const secret = "test-dummy-secret-7f3a"

	tests := []struct {
		name string
		flag string
		env  string
		file func(t *testing.T) string
	}{
		{
			name: "conflicting sources",
			flag: secret,
			env:  secret + "-env",
		},
		{
			name: "file over the size limit",
			file: func(t *testing.T) string {
				return writeAPIKeyFile(t, secret+strings.Repeat("x", 64*1024))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyFile := ""
			if tt.file != nil {
				keyFile = tt.file(t)
			}

			_, _, err := resolveAPIKey(tt.flag, tt.env, keyFile)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), secret)
		})
	}
}

func TestResolveAPIKeyForTransport(t *testing.T) {
	const fileKey = "test-file-key"
	missingFile := func(t *testing.T) string { return filepath.Join(t.TempDir(), "absent") }

	tests := []struct {
		name       string
		transport  string
		flag       string
		env        string
		file       func(t *testing.T) string
		wantKey    string
		wantSource string
		wantErr    []string
	}{
		{
			name:      "stdio ignores a blank env key",
			transport: "stdio",
			env:       "   ",
		},
		{
			name:      "stdio ignores a blank flag key",
			transport: "stdio",
			flag:      " ",
		},
		{
			name:      "stdio ignores a missing key file",
			transport: "stdio",
			file:      missingFile,
		},
		{
			name:      "stdio ignores an empty key file",
			transport: "stdio",
			file:      func(t *testing.T) string { return writeAPIKeyFile(t, "") },
		},
		{
			name:      "stdio ignores conflicting sources",
			transport: "stdio",
			env:       "test-env-key",
			file:      func(t *testing.T) string { return writeAPIKeyFile(t, fileKey) },
		},
		{
			name:      "sse ignores a blank env key",
			transport: "sse",
			env:       " ",
		},
		{
			name:      "sse ignores a whitespace-only key file",
			transport: "sse",
			file:      func(t *testing.T) string { return writeAPIKeyFile(t, "  \n") },
		},
		{
			name:       "streamable uses a valid key file",
			transport:  "streamable",
			file:       func(t *testing.T) string { return writeAPIKeyFile(t, fileKey+"\n") },
			wantKey:    fileKey,
			wantSource: apiKeySourceFile,
		},
		{
			name:      "streamable still refuses a blank env key",
			transport: "streamable",
			env:       " ",
			wantErr:   []string{apiKeySourceEnv, "blank"},
		},
		{
			name:      "streamable still refuses a missing key file",
			transport: "streamable",
			file:      missingFile,
			wantErr:   []string{"api_key file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyFile := ""
			if tt.file != nil {
				keyFile = tt.file(t)
			}

			key, source, err := resolveAPIKeyForTransport(tt.transport, tt.flag, tt.env, keyFile)

			if len(tt.wantErr) > 0 {
				require.Error(t, err)
				for _, part := range tt.wantErr {
					assert.Contains(t, err.Error(), part)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantKey, key)
			assert.Equal(t, tt.wantSource, source)
		})
	}
}
