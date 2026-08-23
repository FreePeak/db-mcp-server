package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestGrantsCatalog proves per-engine privilege SELECTs exist.
func TestGrantsCatalog(t *testing.T) {
	pg := grantsCatalog("postgres")
	if !strings.Contains(pg, "role_table_grants") {
		t.Fatalf("pg catalog wrong:\n%s", pg)
	}
	my := grantsCatalog("mysql")
	if !strings.Contains(my, "TABLE_PRIVILEGES") || !strings.Contains(my, "DATABASE()") {
		t.Fatalf("mysql catalog wrong:\n%s", my)
	}
	if grantsCatalog("sqlite") != "" {
		t.Fatal("sqlite should have no grants catalog")
	}
}

// TestListGrants_Unsupported proves engines without privilege catalogs
// get an explicit error.
func TestListGrants_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListGrants(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
