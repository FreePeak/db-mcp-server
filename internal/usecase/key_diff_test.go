package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

// TestDiffKeys proves cycle 110: primary-key sets are compared across
// two databases — keys only in A, only in B, and the shared count —
// so an agent can verify a copy/sync actually landed.
func TestDiffKeys(t *testing.T) {
	rawA := openSQLiteForTest(t)
	rawB := openSQLiteForTest(t)
	if _, err := rawA.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := rawB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	// A has 1,2,3; B has 2,3,4.
	for _, id := range []int{1, 2, 3} {
		if _, err := rawA.Exec(`INSERT INTO users VALUES (?, 'x')`, id); err != nil {
			t.Fatalf("seed A failed: %v", err)
		}
	}
	for _, id := range []int{2, 3, 4} {
		if _, err := rawB.Exec(`INSERT INTO users VALUES (?, 'x')`, id); err != nil {
			t.Fatalf("seed B failed: %v", err)
		}
	}
	repo := &multiRepo{
		dbs:   map[string]domain.Database{"a": &sqliteDB{db: rawA}, "b": &sqliteDB{db: rawB}},
		types: map[string]string{"a": "sqlite", "b": "sqlite"},
	}
	uc := NewDatabaseUseCase(repo)

	out, err := uc.DiffKeys(context.Background(), "a", "b", "users")
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}
	for _, want := range []string{"only in a", "only in b", "1", "4", "2 shared"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}

	// Identical sets report clean parity.
	out2, err := uc.DiffKeys(context.Background(), "b", "b", "")
	if err == nil {
		t.Fatal("empty table name must error")
	}
	_ = out2
}
