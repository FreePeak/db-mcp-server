package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestBusyTimeoutProbe proves the probe targets SQLite only.
func TestBusyTimeoutProbe(t *testing.T) {
	if q := busyTimeoutQuery("sqlite"); !strings.Contains(q, "busy_timeout") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if busyTimeoutQuery("mysql") != "" || busyTimeoutQuery("postgres") != "" {
		t.Fatal("busy_timeout is a SQLite pragma")
	}
}

// TestBusyTimeoutVerdict proves the escalation ladder.
func TestBusyTimeoutVerdict(t *testing.T) {
	if got := busyTimeoutVerdict(5000); got != "" {
		t.Fatalf("healthy timeout must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := busyTimeoutVerdict(0)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "busy") || !strings.Contains(got, "PRAGMA") {
		t.Fatalf("zero timeout not escalated:\n%s", got)
	}
}

// TestAuditBusyTimeout_EndToEnd proves the audit runs against a real
// SQLite database.
func TestAuditBusyTimeout_EndToEnd(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	out, err := uc.AuditBusyTimeout(context.Background(), "db1")
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}
	if out == "" {
		t.Fatal("expected a verdict")
	}
}

// TestAuditBusyTimeout_Unsupported proves non-SQLite engines get an
// explicit error.
func TestAuditBusyTimeout_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "postgres"})
	if _, err := uc.AuditBusyTimeout(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
