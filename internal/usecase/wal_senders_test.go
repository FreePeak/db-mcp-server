package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestWalSendersProbe proves the probe pairs the setting with live
// usage and targets PostgreSQL only.
func TestWalSendersProbe(t *testing.T) {
	q := walSendersProbe("postgres")
	for _, frag := range []string{"max_wal_senders", "pg_stat_replication", "pg_replication_slots"} {
		if !strings.Contains(q, frag) {
			t.Fatalf("probe missing %s:\n%s", frag, q)
		}
	}
	if walSendersProbe("mysql") != "" || walSendersProbe("sqlite") != "" {
		t.Fatal("only postgres exposes max_wal_senders")
	}
}

// TestWalSendersVerdict proves the escalation ladder.
func TestWalSendersVerdict(t *testing.T) {
	if got := walSendersVerdict(10, 2, 3); got != "" {
		t.Fatalf("headroom must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := walSendersVerdict(0, 0, 0)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "streaming replication") {
		t.Fatalf("zero not escalated:\n%s", got)
	}
	if !strings.Contains(got, "ALTER SYSTEM SET max_wal_senders") {
		t.Fatalf("verdict must name the fix, got:\n%s", got)
	}
	atCap := walSendersVerdict(5, 5, 4)
	if !strings.Contains(atCap, "at capacity") || !strings.Contains(atCap, "cannot attach") {
		t.Fatalf("at-capacity not escalated:\n%s", atCap)
	}
	if got := walSendersVerdict(-1, 0, 0); !strings.Contains(got, "unreadable") {
		t.Fatalf("negative/unreadable misjudged:\n%s", got)
	}
}

// TestAuditWalSenders_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditWalSenders_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditWalSenders(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
