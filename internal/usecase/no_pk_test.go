package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestFindTablesWithoutPK proves cycle 116: user tables lacking a
// PRIMARY KEY constraint are flagged; keyed tables stay silent.
func TestFindTablesWithoutPK(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE keyed (id INTEGER PRIMARY KEY, email TEXT)`)
	must(`CREATE TABLE keyless (email TEXT, name TEXT)`) // no PK at all
	must(`CREATE TABLE composite (a INTEGER, b INTEGER, PRIMARY KEY (a, b))`)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.FindTablesWithoutPK(context.Background(), "db1")
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if !strings.Contains(out, "keyless") {
		t.Fatalf("PK-less table not flagged:\n%s", out)
	}
	if strings.Contains(out, "keyed") || strings.Contains(out, "composite") {
		t.Fatalf("keyed table misflagged:\n%s", out)
	}

	// All tables keyed: clean state.
	clean := openSQLiteForTest(t)
	if _, err := clean.Exec(`CREATE TABLE fine (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	uc2 := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: clean}, dbType: "sqlite"})
	out2, err := uc2.FindTablesWithoutPK(context.Background(), "db1")
	if err != nil || !strings.Contains(out2, "have a primary key") {
		t.Fatalf("clean state wrong (%v):\n%s", err, out2)
	}
}
