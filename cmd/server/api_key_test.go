package main

import (
	"os"
	"path/filepath"
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
