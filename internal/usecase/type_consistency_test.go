package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestFindTypeInconsistencies proves cycle 118: a column name shared
// across tables with divergent types is flagged with the per-table
// types; consistently-typed shared columns stay silent.
func TestFindTypeInconsistencies(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE orders (id INTEGER PRIMARY KEY, customer_id INTEGER)`)
	must(`CREATE TABLE shipments (id INTEGER PRIMARY KEY, customer_id TEXT)`) // drift
	must(`CREATE TABLE refunds (id INTEGER PRIMARY KEY, customer_id INTEGER)`)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.FindTypeInconsistencies(context.Background(), "db1")
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if !strings.Contains(out, "customer_id") || !strings.Contains(out, "shipments") {
		t.Fatalf("type divergence not flagged:\n%s", out)
	}

	// Consistent schemas: clean state.
	clean := openSQLiteForTest(t)
	for _, q := range []string{
		`CREATE TABLE a (id INTEGER PRIMARY KEY, ref_id INTEGER)`,
		`CREATE TABLE b (id INTEGER PRIMARY KEY, ref_id INTEGER)`,
	} {
		if _, err := clean.Exec(q); err != nil {
			t.Fatalf("setup failed: %v", err)
		}
	}
	uc2 := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: clean}, dbType: "sqlite"})
	out2, err := uc2.FindTypeInconsistencies(context.Background(), "db1")
	if err != nil || !strings.Contains(out2, "consistent") {
		t.Fatalf("clean state wrong (%v):\n%s", err, out2)
	}

	// Single table: nothing to compare.
	bare := openSQLiteForTest(t)
	if _, err := bare.Exec(`CREATE TABLE lone (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	uc3 := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: bare}, dbType: "sqlite"})
	out3, err := uc3.FindTypeInconsistencies(context.Background(), "db1")
	if err != nil || !strings.Contains(out3, "No column appears") {
		t.Fatalf("single-table state wrong (%v):\n%s", err, out3)
	}
}
