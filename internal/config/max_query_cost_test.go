package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// loadInlineConfig loads configuration from DB_CONFIG with no config file.
func loadInlineConfig(t *testing.T, inline string) error {
	t.Helper()
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("DB_CONFIG", inline)
	_, err := LoadConfig("")
	return err
}

// TestLoadConfig_MaxQueryCostRejectedOnSQLite locks in the fail-closed
// startup check: a cost limit on an engine without an EXPLAIN cost estimate
// would silently allow every query, so the server must refuse to start.
func TestLoadConfig_MaxQueryCostRejectedOnSQLite(t *testing.T) {
	inline := `{"connections":[{"id":"local","type":"sqlite","database_path":"./local.db","max_query_cost":1000}]}`
	err := loadInlineConfig(t, inline)
	if err == nil || !strings.Contains(err.Error(), `"local"`) || !strings.Contains(err.Error(), "sqlite") {
		t.Fatalf("max_query_cost on sqlite must fail startup naming the database, got: %v", err)
	}
}

func TestLoadConfig_MaxQueryCostNegativeRejected(t *testing.T) {
	inline := `{"connections":[{"id":"pg","type":"postgres","host":"h","port":5432,"user":"u","name":"d","max_query_cost":-1}]}`
	err := loadInlineConfig(t, inline)
	if err == nil || !strings.Contains(err.Error(), `"pg"`) || !strings.Contains(err.Error(), "must not be negative") {
		t.Fatalf("negative max_query_cost must fail startup naming the database, got: %v", err)
	}
}
