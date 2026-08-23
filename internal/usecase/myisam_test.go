package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestMyISAMCatalog proves the SELECT reads information_schema.TABLES
// for BASE TABLEs on the MyISAM engine.
func TestMyISAMCatalog(t *testing.T) {
	q := myISAMQuery("mysql")
	for _, want := range []string{"information_schema.TABLES", "ENGINE", "'MyISAM'", "TABLE_TYPE"} {
		if !strings.Contains(q, want) {
			t.Fatalf("catalog missing %q:\n%s", want, q)
		}
	}
	if myISAMQuery("postgres") != "" || myISAMQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb has the MyISAM engine")
	}
}

// TestListMyISAMTables_Unsupported proves unsupported engines get an
// explicit error.
func TestListMyISAMTables_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListMyISAMTables(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
