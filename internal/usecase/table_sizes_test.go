package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestTableSizes proves cycle 94: every table reports its row count,
// sorted heaviest first, and empty tables are included.
func TestTableSizes(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE small (id INTEGER PRIMARY KEY)`)
	must(`CREATE TABLE big (id INTEGER PRIMARY KEY, note TEXT)`)
	for i := 0; i < 25; i++ {
		must(`INSERT INTO big VALUES (NULL, 'x')`)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.TableSizes(context.Background(), "db1")
	if err != nil {
		t.Fatalf("sizes failed: %v", err)
	}
	if !strings.Contains(out, "big") || !strings.Contains(out, "25") {
		t.Fatalf("big/25 missing:\n%s", out)
	}
	if !strings.Contains(out, "small") || !strings.Contains(out, "0") {
		t.Fatalf("empty table missing:\n%s", out)
	}
	bigIdx := strings.Index(out, "big")
	smallIdx := strings.Index(out, "small")
	if bigIdx == -1 || smallIdx == -1 || bigIdx > smallIdx {
		t.Fatalf("heaviest table should sort first:\n%s", out)
	}
}
