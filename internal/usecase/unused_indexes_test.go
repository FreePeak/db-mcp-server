package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestListUnusedIndexes_Unsupported proves cycle 97: engines without
// usage statistics get an explicit message, never fabricated output.
func TestListUnusedIndexes_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	_, err := uc.ListUnusedIndexes(context.Background(), "db1", 100)
	if err == nil || !strings.Contains(err.Error(), "usage statistic") {
		t.Fatalf("expected unsupported-engine error, got %v", err)
	}
}

// TestUnusedIndexQueries proves the per-engine catalog SELECTs exist and
// parameterize the minimum-scan threshold.
func TestUnusedIndexQueries(t *testing.T) {
	pg := unusedIndexQuery("postgres")
	if !strings.Contains(pg, "pg_stat_user_indexes") || !strings.Contains(pg, "$1") {
		t.Fatalf("pg query wrong:\n%s", pg)
	}
	my := unusedIndexQuery("mysql")
	if !strings.Contains(my, "schema_unused_indexes") {
		t.Fatalf("mysql query wrong:\n%s", my)
	}
	if unusedIndexQuery("sqlite") != "" {
		t.Fatal("sqlite should have no usage-stat query")
	}
}
