package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSuggestIndexes_EndToEnd_SuggestsUnindexedColumns runs the advisor
// against real in-memory SQLite: filter and sort columns with no covering
// index must produce CREATE INDEX suggestions.
func TestSuggestIndexes_EndToEnd_SuggestsUnindexedColumns(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE orders (id INTEGER PRIMARY KEY, customer_id INTEGER, status TEXT)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db",
		`SELECT id FROM orders WHERE customer_id = 42 AND status = 'paid'`)
	if err != nil {
		t.Fatalf("suggest_indexes failed: %v", err)
	}
	// Two equality predicates fold into one composite index instead of two
	// single-column suggestions.
	if !strings.Contains(out, "CREATE INDEX idx_orders_customer_id_status ON orders (customer_id, status)") {
		t.Errorf("expected composite suggestion for (customer_id, status), got:\n%s", out)
	}
	if strings.Contains(out, "idx_orders_status ON orders (status)") {
		t.Errorf("expected composite members not re-suggested singly, got:\n%s", out)
	}
}

// TestSuggestIndexes_CompositeEqualityThenSort locks in btree column order:
// equality columns first, then ORDER BY columns.
func TestSuggestIndexes_CompositeEqualityThenSort(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE events (id INTEGER PRIMARY KEY, tenant_id INTEGER, kind TEXT, created_at TEXT)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db",
		`SELECT * FROM events WHERE tenant_id = 7 ORDER BY created_at DESC`)
	if err != nil {
		t.Fatalf("suggest_indexes failed: %v", err)
	}
	if !strings.Contains(out, "(tenant_id, created_at)") {
		t.Fatalf("expected composite (tenant_id, created_at), got:\n%s", out)
	}
}

// TestSuggestIndexes_PureRangeQueriesStaySingleColumn checks that range-only
// predicates never form composites — there is no equality prefix to lead one.
func TestSuggestIndexes_PureRangeQueriesStaySingleColumn(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE products (id INTEGER PRIMARY KEY, price REAL, stock INTEGER)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db",
		`SELECT * FROM products WHERE price > 10 AND stock < 5`)
	if err != nil {
		t.Fatalf("suggest_indexes failed: %v", err)
	}
	if !strings.Contains(out, "idx_products_price ON products (price)") ||
		!strings.Contains(out, "idx_products_stock ON products (stock)") {
		t.Fatalf("expected separate single-column suggestions, got:\n%s", out)
	}
	if strings.Contains(out, ", ") && strings.Contains(out, "CREATE INDEX idx_products_price_stock") {
		t.Errorf("range-only query must not produce a composite, got:\n%s", out)
	}
}

// TestSuggestIndexes_EndToEnd_CoversIndexedColumn locks in that an existing
// index on a column suppresses the duplicate suggestion.
func TestSuggestIndexes_EndToEnd_CoversIndexedColumn(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE orders (id INTEGER PRIMARY KEY, customer_id INTEGER, status TEXT)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := raw.Exec(`CREATE INDEX idx_orders_customer ON orders (customer_id)`); err != nil {
		t.Fatalf("failed to create index: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db",
		`SELECT id FROM orders WHERE customer_id = 42`)
	if err != nil {
		t.Fatalf("suggest_indexes failed: %v", err)
	}
	if strings.Contains(out, "customer_id") && strings.Contains(out, "CREATE INDEX") {
		t.Fatalf("expected indexed column to be covered, got:\n%s", out)
	}
	if !strings.Contains(out, "(none") {
		t.Errorf("expected no-suggestion note, got:\n%s", out)
	}
}

// TestSuggestIndexes_SubstringNotConfused guards the exact-token match:
// an index on surname must not mark name as covered.
func TestSuggestIndexes_SubstringNotConfused(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE people (id INTEGER PRIMARY KEY, surname TEXT, name TEXT)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := raw.Exec(`CREATE INDEX idx_people_surname ON people (surname)`); err != nil {
		t.Fatalf("failed to create index: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db",
		`SELECT id FROM people WHERE name = 'ana'`)
	if err != nil {
		t.Fatalf("suggest_indexes failed: %v", err)
	}
	if !strings.Contains(out, "idx_people_name") {
		t.Fatalf("expected name suggested despite surname index, got:\n%s", out)
	}
	if strings.Contains(out, "surname") {
		t.Errorf("expected surname not re-suggested, got:\n%s", out)
	}
}

// TestSuggestIndexes_DropsNonColumnReferences checks that ORDER BY aliases
// that are not table columns are filtered out when the catalog is readable.
func TestSuggestIndexes_DropsNonColumnReferences(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, price REAL)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db",
		`SELECT id, price * 2 AS total FROM items ORDER BY total DESC`)
	if err != nil {
		t.Fatalf("suggest_indexes failed: %v", err)
	}
	if strings.Contains(out, "total") {
		t.Fatalf("expected alias 'total' to be dropped, got:\n%s", out)
	}
	if !strings.Contains(out, "not columns of their table") {
		t.Errorf("expected skipped-references note, got:\n%s", out)
	}
}

// TestSuggestIndexes_JoinColumns suggests indexes on both sides of a join
// condition when they are uncovered; primary keys stay covered.
func TestSuggestIndexes_JoinColumns(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE authors (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE books (id INTEGER PRIMARY KEY, author_id INTEGER, title TEXT)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db",
		`SELECT b.title FROM books b JOIN authors a ON a.id = b.author_id WHERE a.name = 'x'`)
	if err != nil {
		t.Fatalf("suggest_indexes failed: %v", err)
	}
	if !strings.Contains(out, "idx_books_author_id ON books (author_id)") {
		t.Errorf("expected join-column suggestion for books.author_id, got:\n%s", out)
	}
	// Alias artifacts must be resolved to real table names.
	if strings.Contains(out, "ON a (") || strings.Contains(out, "ON b (") {
		t.Errorf("expected join aliases resolved to real tables, got:\n%s", out)
	}
	if strings.Contains(out, "idx_a_") || strings.Contains(out, "idx_b_") {
		t.Errorf("expected no suggestions keyed by alias name, got:\n%s", out)
	}
}

// TestSuggestIndexes_InputGuards covers empty queries and queries without
// table references.
func TestSuggestIndexes_InputGuards(t *testing.T) {
	uc := NewDatabaseUseCase(&fakeRepo{db: &fakeDB{}})
	if _, err := uc.SuggestIndexes(context.Background(), "db", "   "); err == nil {
		t.Fatal("expected error for empty query")
	}

	raw := openSQLiteForTest(t)
	uc2 := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	out, err := uc2.SuggestIndexes(context.Background(), "db", `SELECT 1`)
	if err != nil {
		t.Fatalf("unexpected error for SELECT 1: %v", err)
	}
	if !strings.Contains(out, "No tables detected") {
		t.Fatalf("expected no-tables notice for SELECT 1, got:\n%s", out)
	}
}

// TestSuggestIndexes_ConstraintBackedColumns locks in cycle 42's
// constraint-aware coverage: columns already enforced by PRIMARY KEY or
// UNIQUE constraints need no new index, even when the engine's index
// catalog hides constraint-backed indexes (SQLite autoindexes have NULL
// sql and never reach parseIndexRows).
func TestSuggestIndexes_ConstraintBackedColumns(t *testing.T) {
	raw := openSQLiteForTest(t)
	for _, s := range []string{
		`CREATE TABLE sessions (id INTEGER PRIMARY KEY, token TEXT UNIQUE, payload TEXT)`,
	} {
		if _, err := raw.Exec(s); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db", "SELECT * FROM sessions WHERE token = 'x'")
	if err != nil {
		t.Fatalf("suggest failed: %v", err)
	}
	if strings.Contains(out, "CREATE INDEX") {
		t.Errorf("UNIQUE-constrained column must count as covered, got:\n%s", out)
	}

	out, err = uc.SuggestIndexes(context.Background(), "db", "SELECT * FROM sessions WHERE id = 7")
	if err != nil {
		t.Fatalf("suggest failed: %v", err)
	}
	if strings.Contains(out, "CREATE INDEX") {
		t.Errorf("PK column must count as covered, got:\n%s", out)
	}
}

// TestExtractIndexAdvice_MySQLDigestBackticks locks in the digest
// normalization: MySQL statement digests quote identifiers in backticks,
// which previously made every workload/slow-query statement invisible to
// the advisor.
func TestExtractIndexAdvice_MySQLDigestBackticks(t *testing.T) {
	advice := extractIndexAdvice("SELECT `id` FROM `slow46` WHERE `tenant_id` = ?")
	a, ok := advice["slow46"]
	if !ok {
		t.Fatalf("expected table slow46 from backticked digest, got %v", advice)
	}
	if len(a.eq) != 1 || a.eq[0] != "tenant_id" {
		t.Errorf("expected equality column tenant_id, got %v", a.eq)
	}
}
