package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSecretReferencesFromEnvironment(t *testing.T) {
	t.Setenv("DB_MCP_TEST_PASSWORD", "env-password")
	t.Setenv("DB_MCP_TEST_KEY", "env-key")
	conn := &DatabaseConnectionConfig{
		ID:               "db1",
		PasswordEnv:      "DB_MCP_TEST_PASSWORD",
		EncryptionKeyEnv: "DB_MCP_TEST_KEY",
	}
	if err := ResolveSecretReferences(conn, t.TempDir()); err != nil {
		t.Fatalf("ResolveSecretReferences failed: %v", err)
	}
	if conn.Password != "env-password" || conn.EncryptionKey != "env-key" {
		t.Fatalf("environment secrets were not resolved: %#v", conn)
	}
	if conn.PasswordEnv != "" || conn.PasswordFile != "" || conn.EncryptionKeyEnv != "" || conn.EncryptionKeyFile != "" {
		t.Fatal("secret reference fields should be cleared after resolution")
	}
}

func TestResolveSecretReferencesFromRelativeFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "password"), []byte("file-password\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "key"), []byte("file-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	conn := &DatabaseConnectionConfig{
		ID:                "db1",
		PasswordFile:      "password",
		EncryptionKeyFile: "key",
	}
	if err := ResolveSecretReferences(conn, dir); err != nil {
		t.Fatalf("ResolveSecretReferences failed: %v", err)
	}
	if conn.Password != "file-password" || conn.EncryptionKey != "file-key" {
		t.Fatalf("file secrets were not resolved: %#v", conn)
	}
}

func TestResolveSecretReferencesRejectsAmbiguousOrMissingValues(t *testing.T) {
	tests := []struct {
		name string
		conn DatabaseConnectionConfig
		want string
	}{
		{
			name: "both references",
			conn: DatabaseConnectionConfig{PasswordEnv: "A", PasswordFile: "b"},
			want: "both",
		},
		{
			name: "inline and reference",
			conn: DatabaseConnectionConfig{Password: "inline", PasswordEnv: "A"},
			want: "combine",
		},
		{
			name: "missing file",
			conn: DatabaseConnectionConfig{PasswordFile: "missing"},
			want: "no such file",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ResolveSecretReferences(&tt.conn, t.TempDir())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}
