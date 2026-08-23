package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestIdleSessionsCatalog proves per-engine idle-session SELECTs target
// the slot-holders saturation diagnostics need.
func TestIdleSessionsCatalog(t *testing.T) {
	pg := idleSessionsQuery("postgres")
	if !strings.Contains(pg, "state = 'idle'") || !strings.Contains(pg, "pg_stat_activity") {
		t.Fatalf("pg catalog wrong:\n%s", pg)
	}
	my := idleSessionsQuery("mysql")
	if !strings.Contains(my, "Sleep") || !strings.Contains(my, "processlist") {
		t.Fatalf("mysql catalog wrong:\n%s", my)
	}
	if idleSessionsQuery("sqlite") != "" {
		t.Fatal("sqlite should have no idle-sessions catalog")
	}
}

// TestListIdleSessions_Unsupported proves unsupported engines get an
// explicit error.
func TestListIdleSessions_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListIdleSessions(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
