package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeMultiDBConfigRejectsUnknownFields(t *testing.T) {
	_, err := decodeMultiDBConfig([]byte(`{"connections":[{"id":"db1","type":"sqlite","database_path":"db.sqlite","max_rowz":10}]}`), "test")
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestDecodeMultiDBConfigPreservesGuardrailsAndOracleFields(t *testing.T) {
	raw := []byte(`{
		"schema_version": 1,
		"connections": [{
			"id": "pg",
			"type": "postgres",
			"host": "db",
			"port": 5432,
			"name": "app",
			"user": "reader",
			"password": "secret",
			"query_timeout": 17,
			"max_rows": 25,
			"masking_rules": [{"pattern":"email","strategy":"null"}]
		}, {
			"id": "ora",
			"type": "oracle",
			"user": "reader",
			"service_name": "ORCLPDB1",
			"wallet_location": "/secrets/wallet"
		}]
	}`)

	cfg, err := decodeMultiDBConfig(raw, "test")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if cfg.SchemaVersion != 1 {
		t.Fatalf("expected schema version 1, got %d", cfg.SchemaVersion)
	}
	if got := cfg.Connections[0].MaxRows; got != 25 {
		t.Fatalf("expected max_rows 25, got %d", got)
	}
	if got := cfg.Connections[0].QueryTimeout; got != 17 {
		t.Fatalf("expected query_timeout 17, got %d", got)
	}
	if len(cfg.Connections[0].MaskingRules) != 1 || cfg.Connections[0].MaskingRules[0].Strategy != "null" {
		t.Fatalf("masking rules were not preserved: %#v", cfg.Connections[0].MaskingRules)
	}
	if got := cfg.Connections[1].ServiceName; got != "ORCLPDB1" {
		t.Fatalf("expected Oracle service name, got %q", got)
	}
}

func TestLoadConfigUsesInlineDBConfigAndResolvesSQLitePath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "missing.json")
	inline := `{"connections":[{"id":"local","type":"sqlite","database_path":"./local.db","read_only":true,"max_rows":5}]}`
	t.Setenv("CONFIG_PATH", configPath)
	t.Setenv("DB_CONFIG", inline)

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if len(cfg.MultiDBConfig.Connections) != 1 {
		t.Fatalf("expected one inline connection, got %d", len(cfg.MultiDBConfig.Connections))
	}
	if !filepath.IsAbs(cfg.MultiDBConfig.Connections[0].DatabasePath) {
		t.Fatalf("expected absolute SQLite path, got %q", cfg.MultiDBConfig.Connections[0].DatabasePath)
	}
	if cfg.MultiDBConfig.Connections[0].MaxRows != 5 || !cfg.MultiDBConfig.Connections[0].ReadOnly {
		t.Fatalf("inline guardrails were not preserved: %#v", cfg.MultiDBConfig.Connections[0])
	}
}

func TestLoadConfigAppliesTimeoutOverrideToInlineConfig(t *testing.T) {
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("DB_CONFIG", `{"connections":[{"id":"local","type":"sqlite","database_path":"./local.db"}]}`)
	t.Setenv("QUERY_TIMEOUT_SECONDS", "9")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if got := cfg.MultiDBConfig.Connections[0].QueryTimeout; got != 9 {
		t.Fatalf("expected timeout override 9, got %d", got)
	}
}

func TestLoadConfigResolvesPasswordEnvironmentReference(t *testing.T) {
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("DB_CONFIG", `{"connections":[{"id":"local","type":"postgres","host":"db","port":5432,"name":"app","user":"reader","password_env":"DB_MCP_TEST_PASSWORD"}]}`)
	t.Setenv("DB_MCP_TEST_PASSWORD", "from-env")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if got := cfg.MultiDBConfig.Connections[0].Password; got != "from-env" {
		t.Fatalf("expected environment-resolved password, got %q", got)
	}
	if cfg.MultiDBConfig.Connections[0].PasswordEnv != "" {
		t.Fatal("password_env should be cleared after resolution")
	}
}

func TestLoadConfigRejectsEmptyConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"connections":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	t.Setenv("DB_CONFIG", `{"connections":[{"id":"should-not-win","type":"sqlite","database_path":"other.db"}]}`)

	_, err := LoadConfig("")
	if err == nil || !strings.Contains(err.Error(), "contains no database connections") {
		t.Fatalf("expected empty file to fail closed, got %v", err)
	}
}

func TestLoadConfigLegacyEnvironmentUsesCanonicalConnection(t *testing.T) {
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("DB_CONFIG", "")
	t.Setenv("DB_TYPE", "postgres")
	t.Setenv("DB_HOST", "db.example")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "reader")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "app")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	conn := cfg.MultiDBConfig.Connections[0]
	if conn.Type != "postgres" || conn.Host != "db.example" || conn.Port != 5432 || conn.Name != "app" {
		t.Fatalf("legacy environment was not mapped to canonical config: %#v", conn)
	}
}

func TestDecodeMultiDBConfigRejectsUnsupportedSchemaVersion(t *testing.T) {
	_, err := decodeMultiDBConfig([]byte(`{"schema_version":2,"connections":[]}`), "test")
	if err == nil || !strings.Contains(err.Error(), "unsupported schema_version") {
		t.Fatalf("expected unsupported schema version error, got %v", err)
	}
}

func TestDecodeMultiDBConfigRejectsTrailingJSON(t *testing.T) {
	_, err := decodeMultiDBConfig([]byte(`{"connections":[]} {"connections":[]}`), "test")
	if err == nil {
		t.Fatal("expected trailing JSON value to be rejected")
	}
}
