package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestBuildDatabaseConfig_MaskPII verifies the mask_pii governance setting
// is carried from the JSON-facing connection config into engine config for
// every database type (cycle 21: operator-enforced PII masking).
func TestBuildDatabaseConfig_MaskPII(t *testing.T) {
	for _, dbType := range []string{"postgres", "mysql", "sqlite", "oracle"} {
		t.Run(dbType, func(t *testing.T) {
			cfg := buildDatabaseConfig(DatabaseConnectionConfig{
				ID:      "t-" + dbType,
				Type:    dbType,
				MaskPii: true,
			})
			if !cfg.MaskPII {
				t.Errorf("expected MaskPII true for %s, got false", dbType)
			}
		})
	}
}

// TestManager_MaskPIIEndToEnd wires a real SQLite database through the
// manager's JSON config path and confirms MaskPII() surfaces on the
// connection object.
func TestManager_MaskPIIEndToEnd(t *testing.T) {
	m := NewDBManager()
	path := filepath.Join(t.TempDir(), "masked.db")
	if werr := os.WriteFile(path, []byte{}, 0o600); werr != nil {
		t.Fatalf("failed to pre-create sqlite file: %v", werr)
	}

	cfgJSON := []byte(`{"connections":[{"id":"sqlite-mask","type":"sqlite","database_path":"` + path + `","mask_pii":true,"use_modernc_driver":true}]}`)
	if err := m.LoadConfig(cfgJSON); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if err := m.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = m.CloseAll() }()

	database, err := m.GetDatabase("sqlite-mask")
	if err != nil {
		t.Fatalf("failed to get database: %v", err)
	}
	if !database.MaskPII() {
		t.Error("expected mask_pii=true database")
	}
}

// TestMaskPIIJSONTag locks in the wire format: mask_pii parses, and absent
// fields default to false (backward compatible).
func TestMaskPIIJSONTag(t *testing.T) {
	var cfg DatabaseConnectionConfig
	payload := `{"id":"pg1","type":"postgres","mask_pii":true}`
	if err := json.Unmarshal([]byte(payload), &cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !cfg.MaskPii {
		t.Error("expected mask_pii true")
	}

	var empty DatabaseConnectionConfig
	if err := json.Unmarshal([]byte(`{"id":"x","type":"sqlite"}`), &empty); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if empty.MaskPii {
		t.Error("absent mask_pii must default false")
	}
}
