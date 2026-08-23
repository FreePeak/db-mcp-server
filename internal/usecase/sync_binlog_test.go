package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSyncBinlogProbe proves the probe targets MySQL only.
func TestSyncBinlogProbe(t *testing.T) {
	if q := syncBinlogProbe("mysql"); !strings.Contains(q, "sync_binlog") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if syncBinlogProbe("postgres") != "" || syncBinlogProbe("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose sync_binlog")
	}
}

// TestSyncBinlogVerdict proves the escalation ladder.
func TestSyncBinlogVerdict(t *testing.T) {
	if got := syncBinlogVerdict(1); got != "" {
		t.Fatalf("durable value must render empty (audit adds the clean line), got:\n%s", got)
	}
	if got := syncBinlogVerdict(1000); got != "" {
		t.Fatalf("group-commit N>0 is a deliberate tradeoff, must render empty, got:\n%s", got)
	}
	got := syncBinlogVerdict(0)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "sync_binlog=0") {
		t.Fatalf("disabled not escalated:\n%s", got)
	}
	if !strings.Contains(got, "lose committed transactions") || !strings.Contains(got, "SET GLOBAL sync_binlog=1") {
		t.Fatalf("verdict must name the loss mode and fix, got:\n%s", got)
	}
	if got := syncBinlogVerdict(-1); !strings.Contains(got, "unreadable") {
		t.Fatalf("negative/unreadable misjudged:\n%s", got)
	}
}

// TestAuditSyncBinlog_Unsupported proves non-MySQL engines get an
// explicit error.
func TestAuditSyncBinlog_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditSyncBinlog(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
