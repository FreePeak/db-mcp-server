package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestListViews proves cycle 80: views are listed with their definitions.
func TestListViews(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, active INTEGER)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`CREATE VIEW active_users AS SELECT id FROM users WHERE active = 1`); err != nil {
		t.Fatalf("view failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.ListViews(context.Background(), "db1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(out, "active_users") || !strings.Contains(out, "active = 1") {
		t.Fatalf("expected view name and definition:\n%s", out)
	}
}
