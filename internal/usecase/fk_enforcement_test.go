package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestFKEnforcementProbe proves the probe targets SQLite only.
func TestFKEnforcementProbe(t *testing.T) {
	if q := fkEnforcementQuery("sqlite"); !strings.Contains(q, "foreign_keys") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if fkEnforcementQuery("mysql") != "" || fkEnforcementQuery("postgres") != "" {
		t.Fatal("foreign_keys is a SQLite pragma")
	}
}

// TestFKEnforcementVerdict proves the ladder.
func TestFKEnforcementVerdict(t *testing.T) {
	if got := fkEnforcementVerdict(true); got != "" {
		t.Fatalf("healthy must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := fkEnforcementVerdict(false)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "orphan") || !strings.Contains(got, "PRAGMA") {
		t.Fatalf("off not escalated:\n%s", got)
	}
}

// TestAuditFKEnforcement_EndToEnd proves the audit runs against a real
// SQLite database.
func TestAuditFKEnforcement_EndToEnd(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	out, err := uc.AuditFKEnforcement(context.Background(), "db1")
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}
	if out == "" {
		t.Fatal("expected a verdict")
	}
}

// TestAuditFKEnforcement_Unsupported proves non-SQLite engines get an
// explicit error.
func TestAuditFKEnforcement_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "mysql"})
	if _, err := uc.AuditFKEnforcement(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
