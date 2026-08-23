package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestInvalidIndexCatalog proves the SELECT reads indisvalid over user
// schemas.
func TestInvalidIndexCatalog(t *testing.T) {
	q := invalidIndexQuery("postgres")
	for _, want := range []string{"indisvalid", "pg_index", "pg_catalog"} {
		if !strings.Contains(q, want) {
			t.Fatalf("catalog missing %q:\n%s", want, q)
		}
	}
	if invalidIndexQuery("mysql") != "" || invalidIndexQuery("sqlite") != "" {
		t.Fatal("only postgres tracks index validity in the catalog")
	}
}

// TestListInvalidIndexes_Unsupported proves unsupported engines get an
// explicit error.
func TestListInvalidIndexes_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListInvalidIndexes(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
