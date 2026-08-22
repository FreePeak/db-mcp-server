package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestFindDuplicates proves cycle 79: duplicated values in one column are
// reported with occurrence counts and example row ids.
func TestFindDuplicates(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)`)
	must(`INSERT INTO users VALUES (1, 'a@x.io'), (2, 'b@x.io'), (3, 'a@x.io'), (4, 'a@x.io')`)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.FindDuplicates(context.Background(), "db1", "users", "email")
	if err != nil {
		t.Fatalf("dupes failed: %v", err)
	}
	for _, want := range []string{"a@x.io", "3", "1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	// Unique column reports clean.
	out, err = uc.FindDuplicates(context.Background(), "db1", "users", "id")
	if err != nil {
		t.Fatalf("dupes failed: %v", err)
	}
	if !strings.Contains(out, "No duplicates") {
		t.Fatalf("expected clean report:\n%s", out)
	}

	if _, err := uc.FindDuplicates(context.Background(), "db1", "missing", "x"); err == nil {
		t.Fatal("unknown table must error")
	}
}
