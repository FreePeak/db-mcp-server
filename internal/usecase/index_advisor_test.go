package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSuggestIndexes_EndToEndRecommendsMissingIndex runs a real two-table
// SQLite schema with an index on only one join column and asserts the
// advisor proposes an index for the uncovered column.
func TestSuggestIndexes_EndToEndRecommendsMissingIndex(t *testing.T) {
	raw := openSQLiteForTest(t)
	mustExec := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec %q failed: %v", q, err)
		}
	}
	mustExec(`CREATE TABLE authors (id INTEGER PRIMARY KEY, name TEXT)`)
	mustExec(`CREATE TABLE books (id INTEGER PRIMARY KEY, author_id INTEGER, title TEXT)`)
	mustExec(`CREATE INDEX idx_books_title ON books (title)`)

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db1",
		"SELECT b.title FROM books b JOIN authors a ON a.id = b.author_id WHERE b.title = 'Go'")
	if err != nil {
		t.Fatalf("suggest_indexes failed: %v", err)
	}

	if !strings.Contains(out, "idx_books_author_id") && !strings.Contains(out, "ON books (author_id)") {
		t.Fatalf("expected suggestion for books.author_id (join column without index), got:\n%s", out)
	}
	// title is already indexed — must not be suggested again.
	if strings.Contains(out, "books (title)") || strings.Contains(out, "idx_books_title") {
		t.Fatalf("title is already indexed; should not be suggested:\n%s", out)
	}
}

// TestSuggestIndexes_AllCoveredReportsNone verifies the "nothing to do"
// path when every filter/join column already has an index.
func TestSuggestIndexes_AllCoveredReportsNone(t *testing.T) {
	raw := openSQLiteForTest(t)
	mustExec := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec %q failed: %v", q, err)
		}
	}
	mustExec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)`)
	mustExec(`CREATE INDEX idx_users_email ON users (email)`)

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db1",
		"SELECT * FROM users WHERE email = 'x@y.z'")
	if err != nil {
		t.Fatalf("suggest_indexes failed: %v", err)
	}
	if !strings.Contains(out, "none") {
		t.Fatalf("expected '(none ...)' when all columns are covered, got:\n%s", out)
	}
	if strings.Contains(out, "CREATE INDEX") {
		t.Fatalf("expected no CREATE INDEX statements, got:\n%s", out)
	}
}

// TestSuggestIndexes_WhereColumnOnSingleTable checks that plain WHERE
// equality columns are proposed for single-table queries.
func TestSuggestIndexes_WhereColumnOnSingleTable(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE orders (id INTEGER PRIMARY KEY, status TEXT, total REAL)`); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db1",
		"SELECT id FROM orders WHERE status = 'pending' ORDER BY total")
	if err != nil {
		t.Fatalf("suggest_indexes failed: %v", err)
	}
	for _, want := range []string{"status", "total"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected suggestion mentioning %q, got:\n%s", want, out)
		}
	}
}

// TestSuggestIndexes_EmptyQueryErrors locks in validation behavior.
func TestSuggestIndexes_EmptyQueryErrors(t *testing.T) {
	raw := openSQLiteForTest(t)
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	if _, err := uc.SuggestIndexes(context.Background(), "db1", "   "); err == nil {
		t.Fatal("expected error for empty query")
	}
}

// TestSuggestIndexes_NoTablesDetected returns guidance instead of an error
// for non-SELECT statements like INSERT/UPDATE.
func TestSuggestIndexes_NoTablesDetected(t *testing.T) {
	raw := openSQLiteForTest(t)
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	out, err := uc.SuggestIndexes(context.Background(), "db1", "UPDATE stats SET n = n + 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No tables detected") {
		t.Fatalf("expected 'no tables detected' note, got:\n%s", out)
	}
}

// TestSuggestIndexes_AnalyzePerformanceDispatch proves the performance tool's
// suggest_indexes action routes to the advisor end-to-end.
func TestSuggestIndexes_AnalyzePerformanceDispatch(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t1 (id INTEGER PRIMARY KEY, code TEXT)`); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	out, err := uc.AnalyzePerformance(context.Background(), "db1", "suggest_indexes", "SELECT * FROM t1 WHERE code = 'a'", 0, 0)
	if err != nil {
		t.Fatalf("analyze performance dispatch failed: %v", err)
	}
	if !strings.Contains(out, "code") {
		t.Fatalf("expected advisor output mentioning 'code', got:\n%s", out)
	}
}
