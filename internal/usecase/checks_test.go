package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestChecksCatalog proves per-engine CHECK-constraint SELECTs exist.
func TestChecksCatalog(t *testing.T) {
	pg := checksCatalog("postgres")
	if !strings.Contains(pg, "check_constraints") || !strings.Contains(pg, "check_clause") {
		t.Fatalf("pg catalog wrong:\n%s", pg)
	}
	my := checksCatalog("mysql")
	if !strings.Contains(my, "CHECK_CONSTRAINTS") {
		t.Fatalf("mysql catalog wrong:\n%s", my)
	}
	if checksCatalog("sqlite") != "" {
		t.Fatal("sqlite should have no checks catalog")
	}
}

// TestListCheckConstraints_Unsupported proves unsupported engines get
// an explicit error.
func TestListCheckConstraints_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListCheckConstraints(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
