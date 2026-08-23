package usecase

import (
	"strings"
	"testing"
)

// TestHealthTrend proves cycle 102: repeated RecordHealthSample calls
// build a per-database history that HealthTrend renders as deltas.
func TestHealthTrend(t *testing.T) {
	raw := openSQLiteForTest(t)
	db := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: db, dbType: "sqlite"})

	// No samples yet: explicit empty state.
	out, err := uc.HealthTrend("db1")
	if err != nil {
		t.Fatalf("trend failed: %v", err)
	}
	if !strings.Contains(out, "No health") {
		t.Fatalf("empty state wrong:\n%s", out)
	}

	// Two samples with different open connections show the delta.
	uc.RecordHealthSample("db1", 2, 10)
	uc.RecordHealthSample("db1", 6, 10)
	out, err = uc.HealthTrend("db1")
	if err != nil {
		t.Fatalf("trend failed: %v", err)
	}
	if !strings.Contains(out, "2") || !strings.Contains(out, "6") {
		t.Fatalf("samples missing:\n%s", out)
	}
	if !strings.Contains(out, "+4") {
		t.Fatalf("delta missing:\n%s", out)
	}

	// Other databases don't leak into db1's trend.
	uc.RecordHealthSample("other", 9, 10)
	out, _ = uc.HealthTrend("db1")
	if strings.Contains(out, "9") {
		t.Fatalf("cross-database leak:\n%s", out)
	}
}
