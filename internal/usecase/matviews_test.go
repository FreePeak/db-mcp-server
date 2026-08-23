package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestMatviewCatalog proves the SELECT reads pg_matviews for
// unpopulated views only.
func TestMatviewCatalog(t *testing.T) {
	q := unpopulatedMatviewQuery("postgres")
	for _, want := range []string{"pg_matviews", "ispopulated"} {
		if !strings.Contains(q, want) {
			t.Fatalf("catalog missing %q:\n%s", want, q)
		}
	}
	if unpopulatedMatviewQuery("mysql") != "" || unpopulatedMatviewQuery("sqlite") != "" {
		t.Fatal("only postgres has materialized views in the catalog")
	}
}

// TestListUnpopulatedMatviews_Unsupported proves unsupported engines
// get an explicit error.
func TestListUnpopulatedMatviews_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListUnpopulatedMatviews(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
