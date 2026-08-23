package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestPreparedXactCatalog proves the SELECT reads pg_prepared_xacts
// with age rendering.
func TestPreparedXactCatalog(t *testing.T) {
	q := preparedXactQuery("postgres")
	for _, want := range []string{"pg_prepared_xacts", "now() - prepared"} {
		if !strings.Contains(q, want) {
			t.Fatalf("catalog missing %q:\n%s", want, q)
		}
	}
	if preparedXactQuery("mysql") != "" || preparedXactQuery("sqlite") != "" {
		t.Fatal("only postgres surfaces prepared transactions this way")
	}
}

// TestListPreparedTransactions_Unsupported proves unsupported engines
// get an explicit error.
func TestListPreparedTransactions_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListPreparedTransactions(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
