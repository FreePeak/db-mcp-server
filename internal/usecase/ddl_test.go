package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestDumpDDL proves cycle 86: the engine's stored CREATE statements are
// dumped verbatim for tables and indexes.
func TestDumpDDL(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)`)
	must(`CREATE INDEX idx_email ON users (email)`)
	must(`CREATE VIEW adults AS SELECT id FROM users`)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.DumpDDL(context.Background(), "db1")
	if err != nil {
		t.Fatalf("dump failed: %v", err)
	}
	for _, want := range []string{"CREATE TABLE users", "CREATE INDEX idx_email", "CREATE VIEW adults"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
