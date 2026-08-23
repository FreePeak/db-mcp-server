package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestBuildDatabaseConfig_Verbosity verifies the per-database verbosity
// default flows from JSON config into engine config for every type.
func TestBuildDatabaseConfig_Verbosity(t *testing.T) {
	for _, dbType := range []string{"postgres", "mysql", "sqlite", "oracle"} {
		t.Run(dbType, func(t *testing.T) {
			cfg := buildDatabaseConfig(DatabaseConnectionConfig{
				ID: "t-" + dbType, Type: dbType, Verbosity: "normal",
			})
			if cfg.DefaultVerbosity != "normal" {
				t.Errorf("expected DefaultVerbosity normal for %s, got %q", dbType, cfg.DefaultVerbosity)
			}
		})
	}
}

// TestManager_VerbosityEndToEnd proves the setting surfaces on live connections.
func TestManager_VerbosityEndToEnd(t *testing.T) {
	m := NewDBManager()
	path := filepath.Join(t.TempDir(), "v.db")
	if werr := os.WriteFile(path, []byte{}, 0o600); werr != nil {
		t.Fatalf("pre-create failed: %v", werr)
	}

	cfgJSON := []byte(`{"connections":[{"id":"sqlite-v","type":"sqlite","database_path":"` + path + `","verbosity":"minimal","use_modernc_driver":true}]}`)
	if err := m.LoadConfig(cfgJSON); err != nil {
		t.Fatalf("load config: %v", err)
	}
	if err := m.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = m.CloseAll() }()

	database, err := m.GetDatabase("sqlite-v")
	if err != nil {
		t.Fatalf("get database: %v", err)
	}
	if v, ok := database.(interface{ Verbosity() string }); !ok || v.Verbosity() != "minimal" {
		t.Fatalf("expected Verbosity()=minimal, got ok=%v", ok)
	}
}

// TestVerbosityJSONTag locks the wire format and empty-default behavior.
func TestVerbosityJSONTag(t *testing.T) {
	var cfg DatabaseConnectionConfig
	if err := json.Unmarshal([]byte(`{"id":"x","type":"pg","verbosity":"minimal"}`), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Verbosity != "minimal" {
		t.Fatalf("expected minimal, got %q", cfg.Verbosity)
	}
	var empty DatabaseConnectionConfig
	if err := json.Unmarshal([]byte(`{"id":"x","type":"pg"}`), &empty); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if empty.Verbosity != "" {
		t.Fatalf("absent verbosity must be empty, got %q", empty.Verbosity)
	}
}
