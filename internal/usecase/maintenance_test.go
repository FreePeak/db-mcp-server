package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestListMaintenance_Unsupported proves engines without stats catalogs
// get an explicit error rather than fabricated advice.
func TestListMaintenance_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListMaintenance(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}

// TestMaintenanceCatalog proves per-engine maintenance SELECTs exist.
func TestMaintenanceCatalog(t *testing.T) {
	pg := maintenanceCatalog("postgres")
	if !strings.Contains(pg, "pg_stat_user_tables") || !strings.Contains(pg, "n_dead_tup") {
		t.Fatalf("pg catalog wrong:\n%s", pg)
	}
	my := maintenanceCatalog("mysql")
	if !strings.Contains(my, "DATA_FREE") {
		t.Fatalf("mysql catalog wrong:\n%s", my)
	}
	if maintenanceCatalog("sqlite") != "" {
		t.Fatal("sqlite should have no maintenance catalog")
	}
}
