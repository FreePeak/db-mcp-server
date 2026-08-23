package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestWALLevelProbe proves the probe targets PostgreSQL only.
func TestWALLevelProbe(t *testing.T) {
	if q := walLevelQuery("postgres"); !strings.Contains(q, "wal_level") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if walLevelQuery("mysql") != "" || walLevelQuery("sqlite") != "" {
		t.Fatal("only postgres exposes wal_level")
	}
}

// TestWALLevelVerdict proves the escalation ladder.
func TestWALLevelVerdict(t *testing.T) {
	for _, ok := range []string{"replica", "logical"} {
		if got := walLevelVerdict(ok); got != "" {
			t.Fatalf("healthy %q must render empty, got:\n%s", ok, got)
		}
	}
	got := walLevelVerdict("minimal")
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "replication") || !strings.Contains(got, "ALTER SYSTEM") {
		t.Fatalf("minimal not escalated:\n%s", got)
	}
	if got := walLevelVerdict(""); !strings.Contains(got, "unreadable") {
		t.Fatalf("empty misjudged:\n%s", got)
	}
}

// TestAuditWALLevel_Unsupported proves non-PG engines get an explicit
// error.
func TestAuditWALLevel_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditWALLevel(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
