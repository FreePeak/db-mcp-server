package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestFormatConnectionReport locks in cycle 30's pressure threshold math:
// normal utilization renders an observation, >=80% adds a warning.
func TestFormatConnectionReport(t *testing.T) {
	calm := formatConnectionReport(5, 12, 100)
	if !strings.Contains(calm, "Connections: 5 active of 12 open (12% of 100 max)") {
		t.Errorf("unexpected calm report: %q", calm)
	}
	if strings.Contains(calm, "WARNING") {
		t.Errorf("calm pool must not warn: %q", calm)
	}

	hot := formatConnectionReport(90, 90, 100)
	if !strings.Contains(hot, "WARNING") || !strings.Contains(hot, "idle-in-transaction") {
		t.Errorf("expected pressure warning at 90%% capacity, got: %q", hot)
	}

	// Degenerate inputs render nothing rather than nonsense.
	if got := formatConnectionReport(0, 0, 0); got != "" {
		t.Errorf("expected empty report for unknown max_connections, got %q", got)
	}
}

// TestDbHealth_SQLiteGraceful verifies the unified db_health action end to
// end on an engine without connection catalogs: the index section renders
// and no connections section appears.
func TestDbHealth_SQLiteGraceful(t *testing.T) {
	raw := openSQLiteForTest(t)
	for _, s := range []string{
		`CREATE TABLE items (id INTEGER PRIMARY KEY, sku TEXT)`,
	} {
		if _, err := raw.Exec(s); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.DbHealth(context.Background(), "db")
	if err != nil {
		t.Fatalf("db_health failed: %v", err)
	}
	if !strings.Contains(out, "No duplicate or redundant indexes") {
		t.Errorf("expected index-health content, got:\n%s", out)
	}
	if strings.Contains(out, "Connections:") {
		t.Errorf("SQLite has no connection catalogs; unexpected section:\n%s", out)
	}
}
