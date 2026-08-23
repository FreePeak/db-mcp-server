package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestFKRulesCatalog proves per-engine delete-rule SELECTs return the
// edge plus its ON DELETE / ON UPDATE behavior.
func TestFKRulesCatalog(t *testing.T) {
	pg := fkRulesQuery("postgres")
	for _, want := range []string{"referential_constraints", "delete_rule", "update_rule"} {
		if !strings.Contains(pg, want) {
			t.Fatalf("pg catalog missing %q:\n%s", want, pg)
		}
	}
	my := fkRulesQuery("mysql")
	for _, want := range []string{"REFERENTIAL_CONSTRAINTS", "DELETE_RULE", "UPDATE_RULE"} {
		if !strings.Contains(my, want) {
			t.Fatalf("mysql catalog missing %q:\n%s", want, my)
		}
	}
	if fkRulesQuery("sqlite") != "" {
		t.Fatal("sqlite should have no fk-rules catalog")
	}
}

// TestListFKRules_Unsupported proves unsupported engines get an
// explicit error.
func TestListFKRules_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListFKRules(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
