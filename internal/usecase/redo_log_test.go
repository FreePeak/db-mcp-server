package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestRedoLogProbe proves the probe prefers the modern capacity
// setting and falls back to legacy file-size math.
func TestRedoLogProbe(t *testing.T) {
	probes := redoLogQueries("mysql")
	if len(probes) != 2 ||
		!strings.Contains(probes[0], "innodb_redo_log_capacity") ||
		!strings.Contains(probes[1], "innodb_log_file_size") {
		t.Fatalf("probe ladder wrong:\n%s", strings.Join(probes, "\n"))
	}
	if len(redoLogQueries("postgres")) != 0 || len(redoLogQueries("sqlite")) != 0 {
		t.Fatal("only mysql/mariadb expose redo-log sizing")
	}
}

// TestRedoLogVerdict proves the escalation ladder.
func TestRedoLogVerdict(t *testing.T) {
	if got := redoLogVerdict(2 << 30); got != "" {
		t.Fatalf("healthy must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := redoLogVerdict(48 * 1024 * 1024)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "checkpointing") {
		t.Fatalf("small not escalated:\n%s", got)
	}
	if !strings.Contains(got, "innodb_redo_log_capacity") || !strings.Contains(got, "SET GLOBAL") {
		t.Fatalf("warning must name the modern live fix, got:\n%s", got)
	}
}

// TestAuditRedoLog_Unsupported proves non-MySQL engines get an
// explicit error.
func TestAuditRedoLog_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditRedoLog(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
