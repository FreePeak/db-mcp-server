package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestListLongQueries_Unsupported proves cycle 98: engines without an
// activity catalog get an explicit error.
func TestListLongQueries_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	_, err := uc.ListLongQueries(context.Background(), "db1", 30)
	if err == nil || !strings.Contains(err.Error(), "activity") {
		t.Fatalf("expected unsupported-engine error, got %v", err)
	}
}

// TestLongQueryCatalogs proves the per-engine activity SELECTs exist
// and parameterize the age threshold.
func TestLongQueryCatalogs(t *testing.T) {
	pg := longQueriesCatalog("postgres")
	if !strings.Contains(pg, "pg_stat_activity") || !strings.Contains(pg, "$1") {
		t.Fatalf("pg catalog wrong:\n%s", pg)
	}
	my := longQueriesCatalog("mysql")
	if !strings.Contains(my, "processlist") || !strings.Contains(my, "?") {
		t.Fatalf("mysql catalog wrong:\n%s", my)
	}
	if longQueriesCatalog("sqlite") != "" {
		t.Fatal("sqlite should have no activity catalog")
	}
}
