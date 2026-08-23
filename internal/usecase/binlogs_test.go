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
