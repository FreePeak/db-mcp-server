package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSessionsQuery covers cycle 61 engine catalog SQL generation.
func TestSessionsQuery(t *testing.T) {
	pg := sessionsQuery("postgres")
	if !strings.Contains(pg, "pg_stat_activity") || !strings.Contains(pg, "pid") {
		t.Fatalf("postgres sessions query wrong: %s", pg)
	}
	my := sessionsQuery("mysql")
	if !strings.Contains(my, "processlist") || !strings.Contains(my, "id") {
		t.Fatalf("mysql sessions query wrong: %s", my)
	}
	for _, t2 := range []string{"sqlite", "oracle", ""} {
		if sessionsQuery(t2) != "" {
			t.Fatalf("engine %q must have no sessions query", t2)
		}
	}
}

func TestCancelQueryStmt(t *testing.T) {
	if s, ok := cancelQueryStmt("postgres", 123); !ok || s != "SELECT pg_cancel_backend(123)" {
		t.Fatalf("pg cancel wrong: %q %v", s, ok)
	}
	if s, ok := cancelQueryStmt("mysql", 42); !ok || s != "KILL QUERY 42" {
		t.Fatalf("mysql cancel wrong: %q %v", s, ok)
	}
	if _, ok := cancelQueryStmt("sqlite", 1); ok {
		t.Fatal("sqlite must not support cancel")
	}
}

// TestListActiveSessions_SQLiteUnsupported proves a clean, explicit error
// on engines without a server session catalog.
func TestListActiveSessions_SQLiteUnsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	_, err := uc.ListActiveSessions(context.Background(), "db1")
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected unsupported error, got: %v", err)
	}
	_, err = uc.CancelQuery(context.Background(), "db1", 1)
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected unsupported error, got: %v", err)
	}
}
