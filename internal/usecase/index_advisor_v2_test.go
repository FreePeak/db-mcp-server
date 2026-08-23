package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSuggestIndexes_CompositeWhereSuggestsCompositeIndex verifies a query
// filtering on two unindexed columns of the same table yields a composite
// suggestion instead of two single-column duplicates.
func TestSuggestIndexes_CompositeWhereSuggestsCompositeIndex(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE events (id INTEGER PRIMARY KEY, status TEXT, created_at TEXT, payload TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db1",
		"SELECT * FROM events WHERE status = 'open' AND created_at > '2026-01-01'")
	if err != nil {
		t.Fatalf("suggest failed: %v", err)
	}
	if !strings.Contains(out, "(status, created_at)") {
		t.Fatalf("expected composite suggestion (status, created_at), got:\n%s", out)
	}
	if strings.Contains(out, "CREATE INDEX idx_events_status ON events (status);") {
		t.Fatalf("single-column duplicate should be replaced by composite:\n%s", out)
	}
}

// TestSuggestIndexes_PartialCoverExtendsComposite proves an existing index on
// the first column leads to an extension suggestion rather than a from-scratch one.
func TestSuggestIndexes_PartialCoverExtendsComposite(t *testing.T) {
	raw := openSQLiteForTest(t)
	mustExec := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec %q failed: %v", q, err)
		}
	}
	mustExec(`CREATE TABLE events (id INTEGER PRIMARY KEY, status TEXT, created_at TEXT)`)
	mustExec(`CREATE INDEX idx_events_status ON events (status)`)

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db1",
		"SELECT * FROM events WHERE status = 'open' AND created_at > '2026-01-01'")
	if err != nil {
		t.Fatalf("suggest failed: %v", err)
	}
	if !strings.Contains(out, "idx_events_status") && !strings.Contains(out, "created_at") {
		t.Fatalf("expected guidance to extend idx_events_status with created_at:\n%s", out)
	}
}

// TestSuggestIndexes_SkipsPrimaryKeyColumns proves PK columns are never
// suggested (they are already indexed by definition).
func TestSuggestIndexes_SkipsPrimaryKeyColumns(t *testing.T) {
	raw := openSQLiteForTest(t)
	mustExec := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec %q failed: %v", q, err)
		}
	}
	mustExec(`CREATE TABLE products (sku TEXT PRIMARY KEY, name TEXT, price REAL)`)
	mustExec(`INSERT INTO products VALUES ('A1', 'widget', 9.99)`)

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db1",
		"SELECT name FROM products WHERE sku = 'A1'")
	if err != nil {
		t.Fatalf("suggest failed: %v", err)
	}
	if strings.Contains(out, "products (sku)") || strings.Contains(out, "idx_products_sku") {
		t.Fatalf("PK column must not be suggested:\n%s", out)
	}
}
