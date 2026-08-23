package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestTableCacheProbe proves the probe reads the cache size plus both
// status counters.
func TestTableCacheProbe(t *testing.T) {
	q := tableOpenCacheQuery("mysql")
	for _, want := range []string{"table_open_cache", "Open_tables", "Opened_tables"} {
		if !strings.Contains(q, want) {
			t.Fatalf("probe missing %q:\n%s", want, q)
		}
	}
	if tableOpenCacheQuery("postgres") != "" || tableOpenCacheQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose table_open_cache")
	}
}

// TestTableCacheVerdict proves the pressure escalation.
func TestTableCacheVerdict(t *testing.T) {
	if got := tableCacheVerdict(4000, 4000, 50000); !strings.Contains(got, "WARNING") {
		t.Fatalf("saturated cache not escalated:\n%s", got)
	}
	if got := tableCacheVerdict(4000, 1200, 300); got == "" || strings.Contains(got, "WARNING") {
		t.Fatalf("healthy cache misjudged:\n%s", got)
	}
}

// TestAuditTableCache_Unsupported proves unsupported engines get an
// explicit error.
func TestAuditTableCache_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditTableCache(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
