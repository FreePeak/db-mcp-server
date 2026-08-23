package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSizeBaseline proves cycle 104: capture a baseline of per-table
// counts, then compare later state against it as deltas.
func TestSizeBaseline(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := raw.Exec(`INSERT INTO t VALUES (NULL)`); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	// No baseline yet: explicit empty state.
	out, err := uc.CompareSizeBaseline(context.Background(), "db1")
	if err != nil || !strings.Contains(out, "No baseline") {
		t.Fatalf("empty state wrong (%v):\n%s", err, out)
	}

	if _, err := uc.CaptureSizeBaseline(context.Background(), "db1"); err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO t VALUES (NULL), (NULL), (NULL)`); err != nil {
		t.Fatalf("grow failed: %v", err)
	}
	out, err = uc.CompareSizeBaseline(context.Background(), "db1")
	if err != nil {
		t.Fatalf("compare failed: %v", err)
	}
	if !strings.Contains(out, "+3") || !strings.Contains(out, "t") {
		t.Fatalf("delta missing:\n%s", out)
	}

	// New tables since the baseline are reported too.
	if _, err := raw.Exec(`CREATE TABLE extra (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create2 failed: %v", err)
	}
	out, _ = uc.CompareSizeBaseline(context.Background(), "db1")
	if !strings.Contains(out, "new since baseline") {
		t.Fatalf("new-table note missing:\n%s", out)
	}
}
