package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestRelatedRows proves cycle 76: outgoing FKs resolve to parent rows and
// incoming references list child rows for one key value.
func TestRelatedRows(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE authors (id INTEGER PRIMARY KEY, name TEXT)`)
	must(`CREATE TABLE books (id INTEGER PRIMARY KEY, title TEXT, author_id INTEGER REFERENCES authors(id))`)
	must(`INSERT INTO authors VALUES (1, 'Ada')`)
	must(`INSERT INTO books VALUES (10, 'Notes', 1)`)

	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	// Outgoing: books(1)... use books row id=10 -> parent author Ada.
	out, err := uc.RelatedRows(context.Background(), "db1", "books", "10")
	if err != nil {
		t.Fatalf("related failed: %v", err)
	}
	if !strings.Contains(out, "authors") || !strings.Contains(out, "Ada") {
		t.Fatalf("parent row not resolved:\n%s", out)
	}

	// Incoming: authors row 1 is referenced by books.
	out, err = uc.RelatedRows(context.Background(), "db1", "authors", "1")
	if err != nil {
		t.Fatalf("related failed: %v", err)
	}
	if !strings.Contains(out, "books") || !strings.Contains(out, "Notes") {
		t.Fatalf("child row not resolved:\n%s", out)
	}

	if _, err := uc.RelatedRows(context.Background(), "db1", "nope", "1"); err == nil {
		t.Fatal("unknown table must error")
	}
}
