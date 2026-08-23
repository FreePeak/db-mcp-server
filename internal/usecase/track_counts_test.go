package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestTrackCountsProbe proves the probe reads the stats-collection
// flag and targets PostgreSQL only.
func TestTrackCountsProbe(t *testing.T) {
	q := trackCountsProbe("postgres")
	if !strings.Contains(q, "current_setting('track_counts')") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if trackCountsProbe("mysql") != "" || trackCountsProbe("sqlite") != "" {
		t.Fatal("only postgres exposes track_counts")
	}
}

// TestTrackCountsVerdict proves the escalation ladder.
func TestTrackCountsVerdict(t *testing.T) {
	if got := trackCountsVerdict("on"); got != "" {
		t.Fatalf("enabled must render empty (audit adds the clean line), got:\n%s", got)
	}
	off := trackCountsVerdict("off")
	if !strings.Contains(off, "WARNING") || !strings.Contains(off, "frozen") {
		t.Fatalf("off not escalated:\n%s", off)
	}
	if !strings.Contains(off, "autovacuum") || !strings.Contains(off, "ALTER SYSTEM SET track_counts = on") {
		t.Fatalf("verdict must name the blast radius and fix, got:\n%s", off)
	}
	blank := trackCountsVerdict("")
	if !strings.Contains(blank, "unreadable") {
		t.Fatalf("blank misjudged:\n%s", blank)
	}
}

// TestAuditTrackCounts_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditTrackCounts_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditTrackCounts(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
