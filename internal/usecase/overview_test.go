package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestOverview proves cycle 100: one call renders the database's shape —
// engine, tables, columns, FK edges, sensitive-column suspects, rows.
func TestOverview(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)`)
	must(`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id), note TEXT)`)
	must(`INSERT INTO users VALUES (1, 'a@b.io')`)
	must(`INSERT INTO orders VALUES (1, 1, 'x')`)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.DatabaseOverview(context.Background(), "db1")
	if err != nil {
		t.Fatalf("overview failed: %v", err)
	}
	for _, want := range []string{"engine", "2 table(s)", "5 column(s)", "1 foreign-key", "row"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if !strings.Contains(strings.ToLower(out), "email") || !strings.Contains(strings.ToLower(out), "pii") {
		if !strings.Contains(strings.ToLower(out), "sensitive") {
			t.Fatalf("sensitive columns not surfaced:\n%s", out)
		}
	}
}
