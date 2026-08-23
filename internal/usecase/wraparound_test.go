package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestWraparoundCatalog proves the catalog SELECT reads datfrozenxid
// age per database.
func TestWraparoundCatalog(t *testing.T) {
	q := wraparoundQuery("postgres")
	if !strings.Contains(q, "datfrozenxid") || !strings.Contains(q, "pg_database") {
		t.Fatalf("catalog wrong:\n%s", q)
	}
	if wraparoundQuery("mysql") != "" {
		t.Fatal("only postgres has XID wraparound")
	}
}

// TestCheckWraparoundRisk_Unsupported proves unsupported engines get an
// explicit error.
func TestCheckWraparoundRisk_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.CheckWraparoundRisk(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}

// TestWraparoundVerdict proves the age thresholds escalate.
func TestWraparoundVerdict(t *testing.T) {
	if got := wraparoundVerdict("db1", 1_000); !strings.Contains(got, "healthy") {
		t.Fatalf("young age misjudged:\n%s", got)
	}
	if got := wraparoundVerdict("db1", 300_000_000); !strings.Contains(got, "WARNING") {
		t.Fatalf("200M+ not warned:\n%s", got)
	}
	if got := wraparoundVerdict("db1", 900_000_000); !strings.Contains(got, "CRITICAL") {
		t.Fatalf("500M+ not critical:\n%s", got)
	}
}
