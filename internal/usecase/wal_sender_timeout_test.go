package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestWalSenderTimeoutProbe proves the probe targets PostgreSQL only.
func TestWalSenderTimeoutProbe(t *testing.T) {
	if q := walSenderTimeoutProbe("postgres"); !strings.Contains(q, "wal_sender_timeout") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if walSenderTimeoutProbe("mysql") != "" || walSenderTimeoutProbe("sqlite") != "" {
		t.Fatal("only postgres exposes wal_sender_timeout")
	}
}

// TestWalSenderTimeoutVerdict proves the escalation ladder.
func TestWalSenderTimeoutVerdict(t *testing.T) {
	if got := walSenderTimeoutVerdict("60s"); got != "" {
		t.Fatalf("default must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := walSenderTimeoutVerdict("0")
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "detection is disabled") {
		t.Fatalf("disabled not escalated:\n%s", got)
	}
	if !strings.Contains(got, "ALTER SYSTEM SET wal_sender_timeout='60s'") {
		t.Fatalf("verdict must name the fix, got:\n%s", got)
	}
	low := walSenderTimeoutVerdict("2s")
	if !strings.Contains(low, "WARNING") || !strings.Contains(low, "flaky links") {
		t.Fatalf("aggressive value not escalated:\n%s", low)
	}
	if got := walSenderTimeoutVerdict(""); !strings.Contains(got, "unreadable") {
		t.Fatalf("empty/unreadable misjudged:\n%s", got)
	}
	if got := walSenderTimeoutVerdict("soon"); !strings.Contains(got, "unreadable") {
		t.Fatalf("non-numeric misjudged:\n%s", got)
	}
}

// TestAuditWalSenderTimeout_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditWalSenderTimeout_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditWalSenderTimeout(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
