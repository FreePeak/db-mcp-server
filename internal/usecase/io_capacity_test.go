package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestIOCapacityProbe proves the probe reads both capacity settings,
// MySQL/MariaDB only.
func TestIOCapacityProbe(t *testing.T) {
	q := ioCapacityQuery("mysql")
	if !strings.Contains(q, "innodb_io_capacity") || !strings.Contains(q, "innodb_io_capacity_max") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if ioCapacityQuery("postgres") != "" || ioCapacityQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose innodb_io_capacity")
	}
}

// TestIOCapacityVerdict proves the escalation ladder.
func TestIOCapacityVerdict(t *testing.T) {
	if got := ioCapacityVerdict(8000, 16000); got != "" {
		t.Fatalf("healthy must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := ioCapacityVerdict(200, 2000)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "200") {
		t.Fatalf("default not escalated:\n%s", got)
	}
	if !strings.Contains(strings.ToLower(got), "set global") {
		t.Fatalf("warning must name the live fix, got:\n%s", got)
	}
	if got := ioCapacityVerdict(0, 0); !strings.Contains(got, "unreadable") {
		t.Fatalf("zero misjudged:\n%s", got)
	}
}

// TestAuditIOCapacity_Unsupported proves non-MySQL engines get an
// explicit error.
func TestAuditIOCapacity_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditIOCapacity(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
