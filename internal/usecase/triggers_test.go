package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestListTriggers proves cycle 82: triggers are listed with table,
// timing/event, and their SQL body.
func TestListTriggers(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`)
	must(`CREATE TABLE audit (line TEXT)`)
	must(`CREATE TRIGGER audit_insert AFTER INSERT ON users BEGIN INSERT INTO audit (line) VALUES ('user added'); END`)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.ListTriggers(context.Background(), "db1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	for _, want := range []string{"audit_insert", "users", "INSERT INTO audit"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
