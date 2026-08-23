package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestLogLockWaitsProbe proves the probe reads the logging flag and
// targets PostgreSQL only.
func TestLogLockWaitsProbe(t *testing.T) {
	q := logLockWaitsProbe("postgres")
	if !strings.Contains(q, "current_setting('log_lock_waits')") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if logLockWaitsProbe("mysql") != "" || logLockWaitsProbe("sqlite") != "" {
		t.Fatal("only postgres exposes log_lock_waits")
	}
}

// TestLogLockWaitsVerdict proves the escalation ladder.
func TestLogLockWaitsVerdict(t *testing.T) {
	if got := logLockWaitsVerdict("on"); got != "" {
		t.Fatalf("enabled must render empty (audit adds the clean line), got:\n%s", got)
	}
	off := logLockWaitsVerdict("off")
	if !strings.Contains(off, "WARNING") || !strings.Contains(off, "never logged") {
		t.Fatalf("off not escalated:\n%s", off)
	}
	if !strings.Contains(off, "ALTER SYSTEM SET log_lock_waits = on") ||
		!strings.Contains(off, "deadlock_timeout") {
		t.Fatalf("verdict must name the fix and threshold, got:\n%s", off)
	}
	blank := logLockWaitsVerdict("")
	if !strings.Contains(blank, "unreadable") {
		t.Fatalf("blank misjudged:\n%s", blank)
	}
}

// TestAuditLogLockWaits_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditLogLockWaits_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditLogLockWaits(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
