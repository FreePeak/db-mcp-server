package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestDictionary proves cycle 105: one call renders the full schema as
// a Markdown data dictionary — per-table sections with column/type/PK/FK.
func TestDictionary(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id))`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.DataDictionary(context.Background(), "db1")
	if err != nil {
		t.Fatalf("dictionary failed: %v", err)
	}
	for _, want := range []string{"## users", "## orders", "| id |", "INTEGER", "PK", "user_id"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "sqlite_") {
		t.Fatal("internal tables leaked into the dictionary")
	}
}
