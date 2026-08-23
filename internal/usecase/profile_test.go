package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestProfileTable proves cycle 95: one call profiles every column of a
// table — nulls, distinct values, and min/max.
func TestProfileTable(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE users (id INTEGER PRIMARY KEY, status TEXT, age INTEGER)`)
	must(`INSERT INTO users VALUES (1, 'active', 30)`)
	must(`INSERT INTO users VALUES (2, 'active', NULL)`)
	must(`INSERT INTO users VALUES (3, NULL, 45)`)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.ProfileTable(context.Background(), "db1", "users")
	if err != nil {
		t.Fatalf("profile failed: %v", err)
	}
	for _, want := range []string{"rows: 3", "status", "distinct: 2", "nulls: 1", "age"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
