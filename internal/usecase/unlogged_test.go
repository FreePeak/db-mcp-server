package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestUnloggedCatalog proves the SELECT scans user-schema tables for
// relpersistence='u'.
func TestUnloggedCatalog(t *testing.T) {
	q := unloggedTableQuery("postgres")
	for _, want := range []string{"relpersistence", "'r', 'p'", "pg_namespace"} {
		if !strings.Contains(q, want) {
			t.Fatalf("catalog missing %q:\n%s", want, q)
		}
	}
	if unloggedTableQuery("mysql") != "" || unloggedTableQuery("sqlite") != "" {
		t.Fatal("only postgres has UNLOGGED persistence")
	}
}

// TestListUnloggedTables_Unsupported proves unsupported engines get an
// explicit error.
func TestListUnloggedTables_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListUnloggedTables(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
