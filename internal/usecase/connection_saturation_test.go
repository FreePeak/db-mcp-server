package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestConnectionSaturationCatalog proves per-engine saturation SELECTs
// exist and report both current usage and the ceiling.
func TestConnectionSaturationCatalog(t *testing.T) {
	pg := saturationQuery("postgres")
	if !strings.Contains(pg, "pg_stat_activity") || !strings.Contains(pg, "max_connections") {
		t.Fatalf("pg catalog wrong:\n%s", pg)
	}
	my := saturationQuery("mysql")
	if !strings.Contains(my, "Threads_connected") || !strings.Contains(my, "max_connections") {
		t.Fatalf("mysql catalog wrong:\n%s", my)
	}
	if saturationQuery("sqlite") != "" {
		t.Fatal("sqlite should have no saturation catalog")
	}
}

// TestCheckConnectionSaturation_Unsupported proves unsupported engines
// get an explicit error.
func TestCheckConnectionSaturation_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.CheckConnectionSaturation(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
