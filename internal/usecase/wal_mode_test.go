package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestWALModeProbe proves the probe targets SQLite only.
func TestWALModeProbe(t *testing.T) {
	if q := walModeQuery("sqlite"); !strings.Contains(q, "journal_mode") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if walModeQuery("mysql") != "" || walModeQuery("postgres") != "" {
		t.Fatal("journal_mode is a SQLite pragma")
	}
}

// TestWALModeVerdict proves the escalation ladder.
func TestWALModeVerdict(t *testing.T) {
	if got := walModeVerdict("wal"); !strings.Contains(got, "healthy") {
		t.Fatalf("wal misjudged:\n%s", got)
	}
	got := walModeVerdict("delete")
	if !strings.Contains(got, "readers") || !strings.Contains(got, "busy") {
		t.Fatalf("rollback journal not escalated:\n%s", got)
	}
	if got := walModeVerdict("memory"); !strings.Contains(got, "in-memory") {
		t.Fatalf("memory mode misjudged:\n%s", got)
	}
}

// TestAuditWALMode_EndToEnd proves the audit runs against a real
// SQLite database and reports its actual journal mode.
func TestAuditWALMode_EndToEnd(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	out, err := uc.AuditWALMode(context.Background(), "db1")
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}
	if out == "" {
		t.Fatal("expected a verdict")
	}
}

// TestAuditWALMode_Unsupported proves non-SQLite engines get an
// explicit error.
func TestAuditWALMode_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "mysql"})
	if _, err := uc.AuditWALMode(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
