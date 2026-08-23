package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestBinlogRowImageProbe proves the probe targets MySQL only.
func TestBinlogRowImageProbe(t *testing.T) {
	if q := binlogRowImageQuery("mysql"); !strings.Contains(q, "binlog_row_image") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if binlogRowImageQuery("postgres") != "" || binlogRowImageQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose binlog_row_image")
	}
}

// TestBinlogRowImageVerdict proves the escalation ladder.
func TestBinlogRowImageVerdict(t *testing.T) {
	if got := binlogRowImageVerdict("MINIMAL"); got != "" {
		t.Fatalf("MINIMAL must render empty (audit adds the clean line), got:\n%s", got)
	}
	if got := binlogRowImageVerdict("NOBLOB"); got != "" {
		t.Fatalf("NOBLOB is a reasonable middle ground, got:\n%s", got)
	}
	got := binlogRowImageVerdict("FULL")
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "before+after") {
		t.Fatalf("FULL not escalated:\n%s", got)
	}
	if !strings.Contains(got, "SET GLOBAL binlog_row_image='MINIMAL'") ||
		!strings.Contains(got, "flashback") {
		t.Fatalf("warning must name the live fix and its caveat, got:\n%s", got)
	}
	if got := binlogRowImageVerdict(""); !strings.Contains(got, "unreadable") {
		t.Fatalf("empty misjudged:\n%s", got)
	}
}

// TestAuditBinlogRowImage_Unsupported proves non-MySQL engines get
// an explicit error.
func TestAuditBinlogRowImage_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditBinlogRowImage(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
