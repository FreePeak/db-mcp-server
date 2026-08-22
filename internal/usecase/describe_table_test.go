package usecase

import (
	"context"
	"strings"
	"testing"
)

func TestIsPlainIdentifier(t *testing.T) {
	valid := []string{"users", "public.users", "_tmp1", "t$1", "v2.orders"}
	invalid := []string{"", "1abc", "users; DROP TABLE x", "a b", "users--x"}
	for _, s := range valid {
		if !isPlainIdentifier(s) {
			t.Errorf("expected %q to be valid identifier", s)
		}
	}
	for _, s := range invalid {
		if isPlainIdentifier(s) {
			t.Errorf("expected %q to be rejected as identifier", s)
		}
	}
}

// TestDescribeTable_EndToEnd describes a real table in in-memory SQLite
// through the full use-case path.
func TestDescribeTable_EndToEnd(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT 'anon', price REAL)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := raw.Exec("INSERT INTO items VALUES (1, 'a', 9.5), (2, 'b', 3.0)"); err != nil {
		t.Fatalf("failed to insert rows: %v", err)
	}

	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	info, err := uc.DescribeTable(context.Background(), "sqlite1", "items")
	if err != nil {
		t.Fatalf("describe failed: %v", err)
	}

	columns := info["columns"].([]map[string]interface{})
	if len(columns) != 3 {
		t.Fatalf("expected 3 columns, got %d: %v", len(columns), columns)
	}
	names := map[string]bool{}
	for _, c := range columns {
		if n, ok := c["column_name"].(string); ok {
			names[n] = true
		}
	}
	for _, want := range []string{"id", "name", "price"} {
		if !names[want] {
			t.Fatalf("missing column %q in %v", want, names)
		}
	}

	if rc, ok := info["rowCount"].(string); !ok || rc != "2" {
		t.Fatalf("expected rowCount 2, got %v", info["rowCount"])
	}
}

// TestDescribeTable_RejectsInjectionInput locks the identifier guard.
func TestDescribeTable_RejectsInjectionInput(t *testing.T) {
	uc := NewDatabaseUseCase(&fakeRepo{db: &fakeDB{}})
	for _, bad := range []string{"", "users; DROP TABLE users", "users--x", "a b"} {
		if _, err := uc.DescribeTable(context.Background(), "db", bad); err == nil {
			t.Fatalf("expected rejection for %q", bad)
		} else if !strings.Contains(err.Error(), "table parameter") && !strings.Contains(err.Error(), "must not be empty") {
			t.Fatalf("unexpected error for %q: %v", bad, err)
		}
	}
}
