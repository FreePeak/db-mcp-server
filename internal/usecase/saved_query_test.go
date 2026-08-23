package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSavedQueries proves cycle 103: save under a name, list names with
// SQL previews, run by name against the owning database.
func TestSavedQueries(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE nums (n INTEGER)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	for i := 1; i <= 5; i++ {
		if _, err := raw.Exec(`INSERT INTO nums VALUES (?)`, i); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	if err := uc.SaveQuery("db1", "top-nums", "SELECT n FROM nums ORDER BY n DESC"); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	list, err := uc.ListSavedQueries("db1")
	if err != nil || !strings.Contains(list, "top-nums") || !strings.Contains(list, "ORDER BY n DESC") {
		t.Fatalf("list wrong (%v):\n%s", err, list)
	}

	out, err := uc.RunSavedQuery(context.Background(), "db1", "top-nums")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(out, "1") {
		t.Fatalf("results missing:\n%s", out)
	}

	// Overwrite same name is allowed; unknown name errors clearly.
	if err := uc.SaveQuery("db1", "top-nums", "SELECT COUNT(*) FROM nums"); err != nil {
		t.Fatalf("overwrite failed: %v", err)
	}
	if _, err := uc.RunSavedQuery(context.Background(), "db1", "ghost"); err == nil ||
		!strings.Contains(err.Error(), "no saved query") {
		t.Fatalf("expected missing-query error, got %v", err)
	}
	if _, err := uc.RunSavedQuery(context.Background(), "db2", "top-nums"); err == nil {
		t.Fatal("saved queries must not cross databases")
	}
}
