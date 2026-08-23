package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestReplicationCatalog proves per-engine replica-status SELECTs exist.
func TestReplicationCatalog(t *testing.T) {
	pg := replicationStatusQuery("postgres")
	if !strings.Contains(pg, "pg_stat_replication") || !strings.Contains(pg, "replay_lag") {
		t.Fatalf("pg catalog wrong:\n%s", pg)
	}
	my := replicationStatusQuery("mysql")
	if !strings.Contains(my, "SHOW REPLICA STATUS") {
		t.Fatalf("mysql catalog wrong:\n%s", my)
	}
	if replicationStatusQuery("sqlite") != "" {
		t.Fatal("sqlite should have no replication catalog")
	}
}

// TestListReplication_Unsupported proves unsupported engines get an
// explicit error rather than fabricated output.
func TestListReplication_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListReplication(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
