package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestAutovacuumOffCatalog proves the SELECT scans reloptions for
// explicitly disabled autovacuum.
func TestAutovacuumOffCatalog(t *testing.T) {
	q := autovacuumDisabledQuery("postgres")
	for _, want := range []string{"reloptions", "autovacuum_enabled=false"} {
		if !strings.Contains(q, want) {
			t.Fatalf("catalog missing %q:\n%s", want, q)
		}
	}
	if autovacuumDisabledQuery("mysql") != "" || autovacuumDisabledQuery("sqlite") != "" {
		t.Fatal("only postgres has per-table autovacuum switches")
	}
}

// TestListAutovacuumDisabled_Unsupported proves unsupported engines get
// an explicit error.
func TestListAutovacuumDisabled_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListAutovacuumDisabled(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
