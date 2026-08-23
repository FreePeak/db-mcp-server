package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestWaitTimeoutProbe proves the probe targets MySQL only.
func TestWaitTimeoutProbe(t *testing.T) {
	q := waitTimeoutQuery("mysql")
	if !strings.Contains(q, "wait_timeout") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if waitTimeoutQuery("postgres") != "" || waitTimeoutQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose wait_timeout")
	}
}

// TestWaitTimeoutVerdict proves the two-sided escalation ladder.
func TestWaitTimeoutVerdict(t *testing.T) {
	if got := waitTimeoutVerdict(600); got != "" {
		t.Fatalf("healthy must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := waitTimeoutVerdict(28800 * 7)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "idle") || !strings.Contains(got, "SET GLOBAL") {
		t.Fatalf("high timeout not escalated:\n%s", got)
	}
	got = waitTimeoutVerdict(5)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "gone away") {
		t.Fatalf("low timeout not escalated:\n%s", got)
	}
	if got := waitTimeoutVerdict(0); !strings.Contains(got, "unreadable") {
		t.Fatalf("zero misjudged:\n%s", got)
	}
}

// TestAuditWaitTimeout_Unsupported proves non-MySQL engines get an
// explicit error.
func TestAuditWaitTimeout_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditWaitTimeout(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
