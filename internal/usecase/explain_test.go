package usecase

import (
	"context"
	"strings"
	"testing"
)

func TestBuildExplainSQL(t *testing.T) {
	cases := []struct {
		name      string
		dbType    string
		statement string
		analyze   bool
		want      string
	}{
		{"postgres plain", "postgres", "SELECT 1", false, "EXPLAIN SELECT 1"},
		{"postgres analyze", "postgres", "SELECT 1", true, "EXPLAIN (ANALYZE, BUFFERS) SELECT 1"},
		{"timescale inherits postgres", "timescale", "SELECT 1", false, "EXPLAIN SELECT 1"},
		{"mysql plain", "mysql", "SELECT 1", false, "EXPLAIN SELECT 1"},
		{"mysql analyze", "mysql", "SELECT 1", true, "EXPLAIN ANALYZE SELECT 1"},
		{"sqlite always query plan", "sqlite", "SELECT 1", true, "EXPLAIN QUERY PLAN SELECT 1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildExplainSQL(tc.dbType, tc.statement, tc.analyze)
			if got != tc.want {
				t.Fatalf("BuildExplainSQL(%q, %q, %v) = %q, want %q", tc.dbType, tc.statement, tc.analyze, got, tc.want)
			}
		})
	}
}

// TestExecuteExplain_ReadOnlyBlocksWrites ensures the classifier sees
// through the EXPLAIN prefix: EXPLAIN ANALYZE DELETE must be refused.
func TestExecuteExplain_ReadOnlyBlocksWrites(t *testing.T) {
	db := &fakeDB{readOnly: true}
	uc := NewDatabaseUseCase(&fakeRepo{db: db})

	_, err := uc.ExecuteExplain(context.Background(), "pg_prod", "DELETE FROM users", true)
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only refusal for EXPLAIN ANALYZE DELETE, got: %v", err)
	}
	if db.queryCalls != 0 {
		t.Fatalf("statement must not reach the database; got %d query calls", db.queryCalls)
	}

	// Reads explain fine on a read-only database.
	if _, err := uc.ExecuteExplain(context.Background(), "pg_prod", "SELECT * FROM users", false); err != nil {
		t.Fatalf("expected EXPLAIN SELECT to succeed, got: %v", err)
	}
}

// TestExecuteExplain_EmptyStatementRejected covers input validation.
func TestExecuteExplain_EmptyStatementRejected(t *testing.T) {
	uc := NewDatabaseUseCase(&fakeRepo{db: &fakeDB{}})
	if _, err := uc.ExecuteExplain(context.Background(), "db", "   ", false); err == nil {
		t.Fatal("expected error for empty statement")
	}
}

// TestExecuteExplain_EndToEnd runs a real plan query against in-memory
// SQLite through the full use-case path.
func TestExecuteExplain_EndToEnd(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	out, err := uc.ExecuteExplain(context.Background(), "sqlite1", "SELECT * FROM items WHERE id = 1", false)
	if err != nil {
		t.Fatalf("explain failed: %v", err)
	}
	if !strings.Contains(out, "SEARCH") && !strings.Contains(out, "SCAN") {
		t.Fatalf("expected SQLite plan output, got:\n%s", out)
	}
}

// TestExecuteExplain_AppendsIndexSuggestions locks in backlog #9's
// loop-closing wiring: a plan whose statement filters on an uncovered
// column carries concrete CREATE INDEX advice under the plan, while a
// fully covered statement's plan stays clean.
func TestExecuteExplain_AppendsIndexSuggestions(t *testing.T) {
	raw := openSQLiteForTest(t)
	for _, s := range []string{
		`CREATE TABLE orders_live (id INTEGER PRIMARY KEY, customer_id INTEGER, region TEXT)`,
		`CREATE INDEX idx_orders_live_customer ON orders_live (customer_id)`,
	} {
		if _, err := raw.Exec(s); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}

	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.ExecuteExplain(context.Background(), "sqlite1", "SELECT * FROM orders_live WHERE region = 'r1'", false)
	if err != nil {
		t.Fatalf("explain failed: %v", err)
	}
	if !strings.Contains(out, "Index suggestions") || !strings.Contains(out, "CREATE INDEX") {
		t.Errorf("uncovered predicate column should surface index advice under the plan, got:\n%s", out)
	}

	out, err = uc.ExecuteExplain(context.Background(), "sqlite1", "SELECT * FROM orders_live WHERE customer_id = 7", false)
	if err != nil {
		t.Fatalf("explain failed: %v", err)
	}
	if strings.Contains(out, "CREATE INDEX") {
		t.Errorf("covered predicate column should not produce advice, got:\n%s", out)
	}
}
