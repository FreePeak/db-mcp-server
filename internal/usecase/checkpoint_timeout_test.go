package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestCheckpointTimeoutProbe proves the probe reads the setting in
// seconds and targets PostgreSQL only.
func TestCheckpointTimeoutProbe(t *testing.T) {
	q := checkpointTimeoutProbe("postgres")
	if !strings.Contains(q, "checkpoint_timeout") || !strings.Contains(q, "EXTRACT") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if checkpointTimeoutProbe("mysql") != "" || checkpointTimeoutProbe("sqlite") != "" {
		t.Fatal("only postgres exposes checkpoint_timeout")
	}
}

// TestCheckpointTimeoutVerdict proves the escalation ladder.
func TestCheckpointTimeoutVerdict(t *testing.T) {
	if got := checkpointTimeoutVerdict(900); got != "" {
		t.Fatalf("healthy must render empty (audit adds the clean line), got:\n%s", got)
	}
	storm := checkpointTimeoutVerdict(60)
	if !strings.Contains(storm, "WARNING") || !strings.Contains(storm, "full-page writes") {
		t.Fatalf("short value not escalated:\n%s", storm)
	}
	if !strings.Contains(storm, "ALTER SYSTEM SET checkpoint_timeout") {
		t.Fatalf("verdict must name the fix, got:\n%s", storm)
	}
	recovery := checkpointTimeoutVerdict(7200)
	if !strings.Contains(recovery, "WARNING") || !strings.Contains(recovery, "crash recovery") {
		t.Fatalf("long value not escalated:\n%s", recovery)
	}
	if got := checkpointTimeoutVerdict(-1); !strings.Contains(got, "unreadable") {
		t.Fatalf("negative/unreadable misjudged:\n%s", got)
	}
}

// TestAuditCheckpointTimeout_Unsupported proves non-PG engines get
// an explicit error.
func TestAuditCheckpointTimeout_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditCheckpointTimeout(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
