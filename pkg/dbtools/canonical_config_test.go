package dbtools

import (
	"path/filepath"
	"testing"

	"github.com/FreePeak/db-mcp-server/pkg/db"
)

func TestInitDatabaseCanonicalConfigPreservesLiveGuardrails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guardrails.db")
	config := &Config{
		MultiDBConfig: &db.MultiDBConfig{
			SchemaVersion: 1,
			Connections: []db.DatabaseConnectionConfig{{
				ID:               "sqlite-guardrails",
				Type:             "sqlite",
				DatabasePath:     path,
				UseModerncDriver: true,
				QueryTimeout:     7,
				MaxRows:          2,
				MaskingRules:     []db.MaskingRule{{Pattern: "email", Strategy: "null"}},
			}},
		},
	}

	if err := InitDatabase(config); err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}
	defer func() { _ = CloseDatabase() }()

	database, err := GetDatabase("sqlite-guardrails")
	if err != nil {
		t.Fatalf("GetDatabase failed: %v", err)
	}
	if got := database.MaxRows(); got != 2 {
		t.Fatalf("expected live max_rows 2, got %d", got)
	}
	if got := database.QueryTimeout(); got != 7 {
		t.Fatalf("expected live query timeout 7, got %d", got)
	}
	rules, ok := database.(interface{ MaskingRules() []db.MaskingRule })
	if !ok {
		t.Fatal("live database did not expose masking rules")
	}
	if got := rules.MaskingRules(); len(got) != 1 || got[0].Strategy != "null" {
		t.Fatalf("expected live masking rule, got %#v", got)
	}
}
