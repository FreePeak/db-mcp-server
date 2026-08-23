package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestPartitionCatalog proves per-engine partition SELECTs read the
// right catalogs and bind the table safely.
func TestPartitionCatalog(t *testing.T) {
	pg := partitionQuery("postgres")
	for _, want := range []string{"pg_inherits", "$1", "relname"} {
		if !strings.Contains(pg, want) {
			t.Fatalf("pg catalog missing %q:\n%s", want, pg)
		}
	}
	my := partitionQuery("mysql")
	for _, want := range []string{"information_schema.PARTITIONS", "?", "PARTITION_DESCRIPTION"} {
		if !strings.Contains(my, want) {
			t.Fatalf("mysql catalog missing %q:\n%s", want, my)
		}
	}
	if partitionQuery("sqlite") != "" {
		t.Fatal("sqlite should have no partition catalog")
	}
}

// TestListPartitions_Unsupported proves unsupported engines get an
// explicit error.
func TestListPartitions_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListPartitions(context.Background(), "db1", "events"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
