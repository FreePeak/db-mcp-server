package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestImportCSV proves cycle 84: CSV content is parsed and inserted
// atomically; malformed rows fail the whole import.
func TestImportCSV(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE people (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	csv := "id,name\n1,Alice\n2,Bob\n"
	out, err := uc.ImportCSV(context.Background(), "db1", "people", csv)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if !strings.Contains(out, "2") || !strings.Contains(out, "inserted") {
		t.Fatalf("expected 2 inserted:\n%s", out)
	}
	var n int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM people`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("rows = %d err=%v, want 2", n, err)
	}

	// Quoted values with commas and embedded header mismatch failure.
	bad := "id,name\n3,\"Smith, John\"\n"
	if _, err := uc.ImportCSV(context.Background(), "db1", "people", bad); err != nil {
		t.Fatalf("quoted comma failed: %v", err)
	}
	if err := raw.QueryRow(`SELECT COUNT(*) FROM people WHERE name = 'Smith, John'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("quoted value not stored correctly: %d %v", n, err)
	}

	// Wrong column count fails atomically — nothing inserted.
	bad = "id,name\n4,X\n5\n"
	if _, err := uc.ImportCSV(context.Background(), "db1", "people", bad); err == nil {
		t.Fatal("malformed row must fail the import")
	}
	if err := raw.QueryRow(`SELECT COUNT(*) FROM people WHERE id = 4`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("row 4 should have been rolled back: %d %v", n, err)
	}
}
