package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestDeadlockCatalog proves per-engine deadlock-counter SELECTs read
// the right cumulative counters.
func TestDeadlockCatalog(t *testing.T) {
	pg := deadlockQuery("postgres")
	if !strings.Contains(pg, "pg_stat_database") || !strings.Contains(pg, "deadlocks") {
		t.Fatalf("pg catalog wrong:\n%s", pg)
	}
	my := deadlockQuery("mysql")
	if !strings.Contains(my, "Innodb_deadlocks") {
		t.Fatalf("mysql catalog wrong:\n%s", my)
	}
	if deadlockQuery("sqlite") != "" {
		t.Fatal("sqlite should have no deadlock catalog")
	}
}

// TestCheckDeadlocks_Unsupported proves unsupported engines get an
// explicit error.
func TestCheckDeadlocks_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.CheckDeadlocks(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
