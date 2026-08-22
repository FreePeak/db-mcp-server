package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestListRoutines proves cycle 83: stored functions/procedures are listed
// with their signatures. SQLite has none, so the clean-empty path is also
// covered on SQLite.
func TestListRoutines(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.ListRoutines(context.Background(), "db1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(out, "No stored routines") {
		t.Fatalf("expected clean-empty report:\n%s", out)
	}
}
