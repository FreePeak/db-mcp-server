package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestDurabilityProbe proves the probe reads the flush mode.
func TestDurabilityProbe(t *testing.T) {
	q := flushLogQuery("mysql")
	if !strings.Contains(q, "innodb_flush_log_at_trx_commit") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if flushLogQuery("postgres") != "" || flushLogQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose innodb_flush_log_at_trx_commit")
	}
}

// TestFlushLogVerdict proves durability escalations.
func TestFlushLogVerdict(t *testing.T) {
	if got := flushLogVerdict(1); !strings.Contains(got, "healthy") {
		t.Fatalf("mode 1 misjudged:\n%s", got)
	}
	if got := flushLogVerdict(2); !strings.Contains(got, "WARNING") || !strings.Contains(got, "committed") {
		t.Fatalf("mode 2 not escalated:\n%s", got)
	}
	if got := flushLogVerdict(0); !strings.Contains(got, "WARNING") || !strings.Contains(got, "SET GLOBAL") {
		t.Fatalf("mode 0 not escalated:\n%s", got)
	}
}

// TestAuditDurability_Unsupported proves unsupported engines get an
// explicit error.
func TestAuditDurability_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditDurability(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
