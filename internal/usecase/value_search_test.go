package usecase

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestSearchValues proves cycle 75: a literal is located across all
// textual columns of every table, with match counts reported.
func TestSearchValues(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, note TEXT)`)
	must(`CREATE TABLE events (id INTEGER PRIMARY KEY, payload TEXT)`)
	must(`CREATE TABLE nums (n INTEGER)`)
	must(`INSERT INTO users VALUES (1, 'alice@corp.io', 'nothing')`)
	must(`INSERT INTO users VALUES (2, 'bob@corp.io', 'mentions alice@corp.io twice')`)
	must(`INSERT INTO events VALUES (1, 'user alice@corp.io signed up')`)
	must(`INSERT INTO nums VALUES (42)`)

	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	out, err := uc.SearchValues(context.Background(), "db1", "alice@corp.io")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	for _, want := range []string{"users.email", "users.note", "events.payload"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "nums") {
		t.Fatalf("non-textual table scanned:\n%s", out)
	}

	out2, err := uc.SearchValues(context.Background(), "db1", "no-such-literal-anywhere")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if !strings.Contains(out2, "No matches") {
		t.Fatalf("expected no-match report:\n%s", out2)
	}
}

// TestSearchValues_RankedByHits proves cycle 91: tables with more matches
// are reported before tables with fewer.
func TestSearchValues_RankedByHits(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE rare (id INTEGER PRIMARY KEY, tag TEXT)`)
	must(`CREATE TABLE dense (id INTEGER PRIMARY KEY, tag TEXT)`)
	must(`INSERT INTO rare VALUES (1, 'needle')`)
	for i := 0; i < 5; i++ {
		must(fmt.Sprintf(`INSERT INTO dense VALUES (%d, 'needle')`, i+10))
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.SearchValues(context.Background(), "db1", "needle")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	denseIdx := strings.Index(out, "dense")
	rareIdx := strings.Index(out, "rare")
	if denseIdx == -1 || rareIdx == -1 {
		t.Fatalf("both tables missing:\n%s", out)
	}
	if denseIdx > rareIdx {
		t.Fatalf("dense table should be ranked first:\n%s", out)
	}
}
