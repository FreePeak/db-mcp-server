package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestBuildDatabaseConfig_MaxRows verifies the max_rows guardrail setting is
// carried from the JSON-facing connection config into the engine config for
// every database type (issue: production guardrails pack).
func TestBuildDatabaseConfig_MaxRows(t *testing.T) {
	for _, dbType := range []string{"postgres", "mysql", "sqlite", "oracle"} {
		t.Run(dbType, func(t *testing.T) {
			cfg := buildDatabaseConfig(DatabaseConnectionConfig{
				ID:      "t-" + dbType,
				Type:    dbType,
				MaxRows: 250,
			})
			if cfg.MaxRows != 250 {
				t.Errorf("expected MaxRows 250 for %s, got %d", dbType, cfg.MaxRows)
			}
		})
	}
}

func TestBuildDatabaseConfig_MaxRowsDefaultUnlimited(t *testing.T) {
	cfg := buildDatabaseConfig(DatabaseConnectionConfig{ID: "t", Type: "postgres"})
	if cfg.MaxRows != 0 {
		t.Errorf("expected default MaxRows 0 (unlimited), got %d", cfg.MaxRows)
	}
}

// TestManager_MaxRowsEndToEnd wires a real SQLite database through the
// manager's JSON config path and confirms MaxRows() surfaces on the
// connection object.
func TestManager_MaxRowsEndToEnd(t *testing.T) {
	m := NewDBManager()
	path := filepath.Join(t.TempDir(), "guardrails.db")
	// Read-only SQLite opens use mode=ro, so the file must already exist
	// (an empty file is a valid SQLite database).
	if werr := os.WriteFile(path, []byte{}, 0o600); werr != nil {
		t.Fatalf("failed to pre-create sqlite file: %v", werr)
	}

	cfgJSON := []byte(`{"connections":[{"id":"sqlite-guard","type":"sqlite","database_path":"` + path + `","read_only":true,"max_rows":42,"use_modernc_driver":true}]}`)
	if err := m.LoadConfig(cfgJSON); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if err := m.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = m.CloseAll() }()

	database, err := m.GetDatabase("sqlite-guard")
	if err != nil {
		t.Fatalf("failed to get database: %v", err)
	}
	if !database.IsReadOnly() {
		t.Error("expected read-only database")
	}
	if got := database.MaxRows(); got != 42 {
		t.Errorf("expected MaxRows 42, got %d", got)
	}
}

// TestMaxRowsJSONTag locks in the wire format so existing configs stay
// backward compatible and new configs parse max_rows correctly.
func TestMaxRowsJSONTag(t *testing.T) {
	var cfg DatabaseConnectionConfig
	payload := `{"id":"pg1","type":"postgres","max_rows":100}`
	if err := json.Unmarshal([]byte(payload), &cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if cfg.MaxRows != 100 {
		t.Errorf("expected max_rows 100, got %d", cfg.MaxRows)
	}

	out, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	back := DatabaseConnectionConfig{}
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}
	if back.MaxRows != 100 {
		t.Errorf("round-trip lost max_rows: got %d", back.MaxRows)
	}
}

// TestBuildDatabaseConfig_MaskPII verifies the mask_pii guardrail is carried
// from the JSON-facing connection config into the engine config for every
// database type, and surfaces on the connection as MaskPII().
func TestBuildDatabaseConfig_MaskPII(t *testing.T) {
	for _, dbType := range []string{"postgres", "mysql", "sqlite", "oracle"} {
		t.Run(dbType, func(t *testing.T) {
			cfg := buildDatabaseConfig(DatabaseConnectionConfig{
				ID:      "t-" + dbType,
				Type:    dbType,
				MaskPII: true,
			})
			if !cfg.MaskPII {
				t.Errorf("expected MaskPII true for %s", dbType)
			}
		})
	}
	if buildDatabaseConfig(DatabaseConnectionConfig{ID: "t", Type: "postgres"}).MaskPII {
		t.Error("mask_pii must default to false")
	}
}

// TestManager_MaskPIIEndToEnd wires a real SQLite database through the
// manager's JSON config path and confirms MaskPII() surfaces on the
// connection object the use case asserts against.
func TestManager_MaskPIIEndToEnd(t *testing.T) {
	m := NewDBManager()
	path := filepath.Join(t.TempDir(), "maskpii.db")
	if werr := os.WriteFile(path, []byte{}, 0o600); werr != nil {
		t.Fatalf("failed to pre-create sqlite file: %v", werr)
	}

	cfgJSON := []byte(`{"connections":[{"id":"sqlite-pii","type":"sqlite","database_path":"` + path + `","mask_pii":true,"use_modernc_driver":true}]}`)
	if err := m.LoadConfig(cfgJSON); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if err := m.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = m.CloseAll() }()

	database, err := m.GetDatabase("sqlite-pii")
	if err != nil {
		t.Fatalf("failed to get database: %v", err)
	}
	masker, ok := database.(interface{ MaskPII() bool })
	if !ok {
		t.Fatal("connection must expose MaskPII()")
	}
	if !masker.MaskPII() {
		t.Error("expected MaskPII() true from JSON config")
	}
}

// TestMaskPIIJSONTag locks the wire format so operators can write mask_pii
// in config.json without a schema surprise.
func TestMaskPIIJSONTag(t *testing.T) {
	var cfg DatabaseConnectionConfig
	if err := json.Unmarshal([]byte(`{"id":"pg1","type":"postgres","mask_pii":true}`), &cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !cfg.MaskPII {
		t.Error("expected mask_pii to parse")
	}
}
