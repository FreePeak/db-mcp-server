package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestAutoIncrementQuery proves the SELECT joins COLUMNS' auto_increment
// columns to TABLES' next value.
func TestAutoIncrementQuery(t *testing.T) {
	q := autoIncrementQuery("mysql")
	for _, want := range []string{"information_schema.COLUMNS", "information_schema.TABLES", "auto_increment"} {
		if !strings.Contains(strings.ToLower(q), strings.ToLower(want)) {
			t.Fatalf("catalog missing %q:\n%s", want, q)
		}
	}
	if autoIncrementQuery("postgres") != "" || autoIncrementQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb exposes AUTO_INCREMENT counters")
	}
}

// TestAICeiling proves per-type ceilings including unsigned variants.
func TestAICeiling(t *testing.T) {
	tests := []struct {
		colType string
		want    uint64
	}{
		{"tinyint", 127},
		{"smallint", 32767},
		{"mediumint", 8388607},
		{"int", 2147483647},
		{"bigint", 9223372036854775807},
		{"int unsigned", 4294967295},
		{"bigint unsigned", 18446744073709551615},
		{"varchar(20)", 0}, // unknown types report zero ceiling → skipped
	}
	for _, tt := range tests {
		if got := aiCeiling(tt.colType); got != tt.want {
			t.Fatalf("aiCeiling(%q) = %d, want %d", tt.colType, got, tt.want)
		}
	}
}

// TestAIRiskLine proves the at-ceiling escalation and comfortable skip.
func TestAIRiskLine(t *testing.T) {
	// 2150000000 is above the 2147483647 int ceiling.
	if got := aiRiskLine("db.t", "int", 2150000000); !strings.Contains(got, "AT CEILING") || !strings.Contains(got, "inserts will fail") {
		t.Fatalf("at-ceiling not escalated:\n%s", got)
	}
	// 2000000000 is ~93% of the 2147483647 int ceiling.
	if got := aiRiskLine("db.t", "int", 2000000000); !strings.Contains(got, "WARNING") {
		t.Fatalf("near-ceiling not warned:\n%s", got)
	}
	if got := aiRiskLine("db.t", "bigint", 1000); got != "" {
		t.Fatalf("comfortable table should render no line: %q", got)
	}
}

// TestAuditAutoIncrement_Unsupported proves unsupported engines get an
// explicit error.
func TestAuditAutoIncrement_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditAutoIncrement(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
