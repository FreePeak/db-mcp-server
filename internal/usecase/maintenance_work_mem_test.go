package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestMaintenanceWorkMemProbe proves the probe reads the setting and
// targets PostgreSQL only.
func TestMaintenanceWorkMemProbe(t *testing.T) {
	q := maintenanceWorkMemProbe("postgres")
	if !strings.Contains(q, "current_setting('maintenance_work_mem')") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if maintenanceWorkMemProbe("mysql") != "" || maintenanceWorkMemProbe("sqlite") != "" {
		t.Fatal("only postgres exposes maintenance_work_mem")
	}
}

// TestMaintenanceWorkMemVerdict proves the escalation ladder.
func TestMaintenanceWorkMemVerdict(t *testing.T) {
	if got := maintenanceWorkMemVerdict(256 * 1024 * 1024); got != "" {
		t.Fatalf("256MB must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := maintenanceWorkMemVerdict(64 * 1024 * 1024) // the default
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "64.0 MB") {
		t.Fatalf("default 64MB not escalated:\n%s", got)
	}
	for _, want := range []string{"VACUUM", "CREATE INDEX", "ALTER SYSTEM SET maintenance_work_mem"} {
		if !strings.Contains(got, want) {
			t.Fatalf("verdict missing %q:\n%s", want, got)
		}
	}
	blank := maintenanceWorkMemVerdict(0)
	if !strings.Contains(blank, "unreadable") {
		t.Fatalf("zero misjudged:\n%s", blank)
	}
}

// TestAuditMaintenanceWorkMem_Unsupported proves non-PG engines get
// an explicit error.
func TestAuditMaintenanceWorkMem_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditMaintenanceWorkMem(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
