package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestBinlogFormatProbe proves the probe targets MySQL/MariaDB only.
func TestBinlogFormatProbe(t *testing.T) {
	if q := binlogFormatQuery("mysql"); !strings.Contains(q, "binlog_format") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if binlogFormatQuery("postgres") != "" || binlogFormatQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose binlog_format")
	}
}

// TestBinlogFormatVerdict proves the escalation ladder.
func TestBinlogFormatVerdict(t *testing.T) {
	if got := binlogFormatVerdict("ROW"); got != "" {
		t.Fatalf("healthy must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := binlogFormatVerdict("STATEMENT")
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "diverge") || !strings.Contains(got, "SET GLOBAL") {
		t.Fatalf("statement not escalated:\n%s", got)
	}
	got = binlogFormatVerdict("MIXED")
	if !strings.Contains(got, "WARNING") {
		t.Fatalf("mixed not escalated:\n%s", got)
	}
	if got := binlogFormatVerdict(""); !strings.Contains(got, "unreadable") {
		t.Fatalf("empty misjudged:\n%s", got)
	}
}

// TestAuditBinlogFormat_Unsupported proves non-MySQL engines get an
// explicit error.
func TestAuditBinlogFormat_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditBinlogFormat(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
