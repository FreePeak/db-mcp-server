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
	for _, col := range []string{"customer_id", "status"} {
		if !strings.Contains(out, "CREATE INDEX idx_orders_"+col+" ON orders ("+col+")") {
			t.Errorf("expected suggestion for orders.%s, got:\n%s", col, out)
		}
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
