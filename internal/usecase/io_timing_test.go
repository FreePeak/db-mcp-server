package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestTrackIoTimingProbe proves the probe targets PostgreSQL only.
func TestTrackIoTimingProbe(t *testing.T) {
	q := trackIoTimingQuery("postgres")
	if !strings.Contains(q, "track_io_timing") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if trackIoTimingQuery("mysql") != "" || trackIoTimingQuery("sqlite") != "" {
		t.Fatal("only postgres exposes track_io_timing")
	}
}

// TestTrackIoTimingVerdict proves the escalation ladder.
func TestTrackIoTimingVerdict(t *testing.T) {
	if got := trackIoTimingVerdict("on"); got != "" {
		t.Fatalf("healthy must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := trackIoTimingVerdict("off")
	if !strings.Contains(got, "I/O") || !strings.Contains(got, "EXPLAIN") || !strings.Contains(got, "ALTER SYSTEM") {
		t.Fatalf("off not escalated:\n%s", got)
	}
	if got := trackIoTimingVerdict(""); !strings.Contains(got, "unreadable") {
		t.Fatalf("empty value misjudged:\n%s", got)
	}
}

// TestAuditTrackIoTiming_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditTrackIoTiming_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditTrackIoTiming(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
