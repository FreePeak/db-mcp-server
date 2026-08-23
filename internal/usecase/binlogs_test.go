package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestBinlogRetentionQuery proves the retention probe reads
// binlog_expire_logs_seconds and only fires on mysql/mariadb.
func TestBinlogRetentionQuery(t *testing.T) {
	q := binlogRetentionQuery("mysql")
	if !strings.Contains(q, "binlog_expire_logs_seconds") {
		t.Fatalf("retention probe wrong:\n%s", q)
	}
	if binlogRetentionQuery("postgres") != "" || binlogRetentionQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb has binary logs")
	}
}

// TestBinlogVerdict proves the retention escalation.
func TestBinlogVerdict(t *testing.T) {
	if got := binlogVerdict(3, 0, 100); !strings.Contains(got, "never expire") || !strings.Contains(got, "binlog_expire_logs_seconds") {
		t.Fatalf("no-retention case misjudged:\n%s", got)
	}
	if got := binlogVerdict(2, 86400*7, 5<<30); got == "" || strings.Contains(got, "never expire") {
		t.Fatalf("healthy case misjudged:\n%s", got)
	}
}

// TestAuditBinaryLogs_Unsupported proves unsupported engines get an
// explicit error.
func TestAuditBinaryLogs_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditBinaryLogs(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}

// TestBinlogDisabledVerdict proves cycle 202: when binary logging is
// explicitly disabled (no binlog files AND expire_secs reads as 0), the
// verdict must say so — a replica source or point-in-time recovery
// target with binlogs silently off is a data-loss risk, not a healthy
// state. The ambiguous case (files present but retention unreadable,
// i.e. no privileges) keeps the old warning wording.
func TestBinlogDisabledVerdict(t *testing.T) {
	got := binlogVerdict(0, 0, 0)
	if !strings.Contains(got, "disabled") || !strings.Contains(got, "replication") ||
		strings.Contains(got, "WARNING") {
		t.Fatalf("disabled-logging misjudged:\n%s", got)
	}
	// Ambiguous: files exist but retention is unreadable → still warn.
	if got := binlogVerdict(2, 0, 4096); !strings.Contains(got, "never expire") {
		t.Fatalf("ambiguous case lost its warning:\n%s", got)
	}
	// Disabled but with a stale file still present (rotation lag) → note
	// it rather than claiming a clean disable.
	if got := binlogVerdict(1, 0, 1024); !strings.Contains(got, "disabled") {
		t.Fatalf("disabled-with-files misjudged:\n%s", got)
	}
}
