package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestStaleSlotCatalog proves the slot SELECT reads activity and
// retained-WAL size.
func TestStaleSlotCatalog(t *testing.T) {
	q := staleSlotQuery("postgres")
	for _, want := range []string{"pg_replication_slots", "active", "restart_lsn"} {
		if !strings.Contains(q, want) {
			t.Fatalf("catalog missing %q:\n%s", want, q)
		}
	}
	if staleSlotQuery("mysql") != "" || staleSlotQuery("sqlite") != "" {
		t.Fatal("only postgres has logical/physical replication slots")
	}
}

// TestListStaleSlots_Unsupported proves unsupported engines get an
// explicit error.
func TestListStaleSlots_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListStaleSlots(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
