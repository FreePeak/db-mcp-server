package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestTempSpillCatalog proves per-engine spill SELECTs read the right
// counters.
func TestTempSpillCatalog(t *testing.T) {
	pg := tempSpillQuery("postgres")
	if !strings.Contains(pg, "pg_stat_database") || !strings.Contains(pg, "temp_files") {
		t.Fatalf("pg catalog wrong:\n%s", pg)
	}
	my := tempSpillQuery("mysql")
	if !strings.Contains(my, "Created_tmp_disk_tables") || !strings.Contains(my, "Created_tmp_tables") {
		t.Fatalf("mysql catalog wrong:\n%s", my)
	}
	if tempSpillQuery("sqlite") != "" {
		t.Fatal("sqlite should have no spill catalog")
	}
}

// TestCheckTempSpills_Unsupported proves unsupported engines get an
// explicit error.
func TestCheckTempSpills_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.CheckTempSpills(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
