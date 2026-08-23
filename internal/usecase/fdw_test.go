package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestFDWCatalog proves the SELECT joins foreign tables to their
// servers across user schemas.
func TestFDWCatalog(t *testing.T) {
	q := foreignTableQuery("postgres")
	for _, want := range []string{"pg_foreign_table", "pg_foreign_server", "relnamespace"} {
		if !strings.Contains(q, want) {
			t.Fatalf("catalog missing %q:\n%s", want, q)
		}
	}
	if foreignTableQuery("mysql") != "" || foreignTableQuery("sqlite") != "" {
		t.Fatal("only postgres has SQL/MED foreign tables")
	}
}

// TestListForeignTables_Unsupported proves unsupported engines get an
// explicit error.
func TestListForeignTables_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListForeignTables(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
