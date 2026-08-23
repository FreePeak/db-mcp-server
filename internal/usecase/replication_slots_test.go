package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestReplicationSlotVerdict proves the classification: inactive
// slots always warn (they retain WAL), exhausted capacity warns,
// clean fleets state health.
func TestReplicationSlotVerdict(t *testing.T) {
	if got := replicationSlotVerdict(nil, 10); !strings.Contains(got, "healthy") || !strings.Contains(got, "0/10") {
		t.Fatalf("empty fleet misjudged:\n%s", got)
	}

	// All active, capacity free -> quiet.
	if got := replicationSlotVerdict([]replicationSlot{{"s1", true, 1024}}, 2); got != "" {
		t.Fatalf("active-only fleet must render empty, got:\n%s", got)
	}

	got := replicationSlotVerdict([]replicationSlot{{"stale1", false, 5 << 30}, {"live", true, 4096}}, 3)
	for _, want := range []string{"stale1", "retaining", "WAL"} {
		if !strings.Contains(got, want) {
			t.Fatalf("verdict missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "live") {
		t.Fatal("active slot must not be reported")
	}

	cap := replicationSlotVerdict(
		[]replicationSlot{{"a", true, 1}, {"b", true, 1}, {"c", true, 1}}, 3)
	if !strings.Contains(cap, "capacity") || !strings.Contains(cap, "full") {
		t.Fatalf("exhausted capacity not escalated:\n%s", cap)
	}
}

// TestReplicationSlotsProbe proves the probe targets PostgreSQL only
// and joins retained-WAL accounting.
func TestReplicationSlotsProbe(t *testing.T) {
	q := replicationSlotsProbe("postgres")
	if !strings.Contains(q, "pg_replication_slots") || !strings.Contains(q, "pg_wal_lsn_diff") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if replicationSlotsProbe("mysql") != "" || replicationSlotsProbe("sqlite") != "" {
		t.Fatal("only postgres exposes replication slots")
	}
}

// TestAuditReplicationSlots_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditReplicationSlots_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditReplicationSlots(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
