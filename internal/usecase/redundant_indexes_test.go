package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestFindRedundantIndexes proves cycle 111: a non-unique index whose
// columns are a prefix of a wider index is flagged redundant; unique
// indexes are never redundant (they enforce a constraint).
func TestFindRedundantIndexes(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, name TEXT, age INTEGER)`)
	must(`CREATE INDEX idx_email ON users (email)`)            // redundant
	must(`CREATE INDEX idx_email_name ON users (email, name)`) // covers it
	must(`CREATE UNIQUE INDEX uniq_email ON users (email)`)    // never redundant
	must(`CREATE INDEX idx_age ON users (age)`)                // unrelated
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.FindRedundantIndexes(context.Background(), "db1")
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if !strings.Contains(out, "idx_email") || !strings.Contains(out, "idx_email_name") {
		t.Fatalf("redundant pair not reported:\n%s", out)
	}
	if strings.Contains(out, "uniq_email") {
		t.Fatalf("unique index misreported as redundant:\n%s", out)
	}

	// No redundancy: clean state.
	clean := openSQLiteForTest(t)
	if _, err := clean.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, a TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := clean.Exec(`CREATE INDEX idx_a ON t (a)`); err != nil {
		t.Fatalf("index failed: %v", err)
	}
	uc2 := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: clean}, dbType: "sqlite"})
	out2, err := uc2.FindRedundantIndexes(context.Background(), "db1")
	if err != nil || !strings.Contains(out2, "No redundant") {
		t.Fatalf("clean state wrong (%v):\n%s", err, out2)
	}
}

// TestRedundantIndexes_IdenticalDuplicates proves cycle 119: two
// non-unique indexes with identical column lists are reported exactly
// once as duplicates.
func TestRedundantIndexes_IdenticalDuplicates(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)`)
	must(`CREATE INDEX idx_email_a ON users (email)`)
	must(`CREATE INDEX idx_email_b ON users (email)`)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.FindRedundantIndexes(context.Background(), "db1")
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if !strings.Contains(out, "duplicate") || !(strings.Count(out, "- users.idx_email") == 1) {
		t.Fatalf("identical duplicates not reported once:\n%s", out)
	}
}
