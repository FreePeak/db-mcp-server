package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSlotWalCapProbe proves the probe targets PostgreSQL only.
func TestSlotWalCapProbe(t *testing.T) {
	if q := slotWalCapQuery("postgres"); !strings.Contains(q, "max_slot_wal_keep_size") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if slotWalCapQuery("mysql") != "" || slotWalCapQuery("sqlite") != "" {
		t.Fatal("only postgres exposes max_slot_wal_keep_size")
	}
}

// TestSlotWalCapVerdict proves the escalation ladder.
func TestSlotWalCapVerdict(t *testing.T) {
	if got := slotWalCapVerdict("500GB"); got != "" {
		t.Fatalf("capped must render empty (audit adds the clean line), got:\n%s", got)
	}
	if got := slotWalCapVerdict("-1"); !strings.Contains(got, "WARNING") || !strings.Contains(got, "unbounded") {
		t.Fatalf("unlimited not escalated:\n%s", got)
	}
	if got := slotWalCapVerdict(""); !strings.Contains(got, "unreadable") {
		t.Fatalf("empty misjudged:\n%s", got)
	}
}

// TestAuditSlotWalCap_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditSlotWalCap_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditSlotWalCap(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
