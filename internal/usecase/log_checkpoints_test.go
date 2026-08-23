package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestLogCheckpointsProbe proves the probe reads the logging flag
// and targets PostgreSQL only.
func TestLogCheckpointsProbe(t *testing.T) {
	q := logCheckpointsProbe("postgres")
	if !strings.Contains(q, "current_setting('log_checkpoints')") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if logCheckpointsProbe("mysql") != "" || logCheckpointsProbe("sqlite") != "" {
		t.Fatal("only postgres exposes log_checkpoints")
	}
}

// TestLogCheckpointsVerdict proves the escalation ladder.
func TestLogCheckpointsVerdict(t *testing.T) {
	if got := logCheckpointsVerdict("on"); got != "" {
		t.Fatalf("enabled must render empty (audit adds the clean line), got:\n%s", got)
	}
	off := logCheckpointsVerdict("off")
	if !strings.Contains(off, "WARNING") || !strings.Contains(off, "invisible") {
		t.Fatalf("off not escalated:\n%s", off)
	}
	if !strings.Contains(off, "ALTER SYSTEM SET log_checkpoints = on") ||
		!strings.Contains(off, "max_wal_size") {
		t.Fatalf("verdict must name the fix and tuning targets, got:\n%s", off)
	}
	blank := logCheckpointsVerdict("")
	if !strings.Contains(blank, "unreadable") {
		t.Fatalf("blank misjudged:\n%s", blank)
	}
}

// TestAuditLogCheckpoints_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditLogCheckpoints_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditLogCheckpoints(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
