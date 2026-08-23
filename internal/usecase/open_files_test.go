package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestOpenFilesLimitProbe proves the probe reads both settings in one
// round trip, MySQL/MariaDB only.
func TestOpenFilesLimitProbe(t *testing.T) {
	q := openFilesLimitQuery("mysql")
	if !strings.Contains(q, "open_files_limit") || !strings.Contains(q, "table_open_cache") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if openFilesLimitQuery("postgres") != "" || openFilesLimitQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose open_files_limit")
	}
}

// TestOpenFilesLimitVerdict proves the escalation ladder.
func TestOpenFilesLimitVerdict(t *testing.T) {
	if got := openFilesLimitVerdict(100000, 4000); got != "" {
		t.Fatalf("healthy must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := openFilesLimitVerdict(1024, 4000)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "open_files_limit") {
		t.Fatalf("capped cache not escalated:\n%s", got)
	}
	if !strings.Contains(strings.ToLower(got), "restart") && !strings.Contains(strings.ToLower(got), "config") {
		t.Fatalf("warning must note this needs restart/config, got:\n%s", got)
	}
	if got := openFilesLimitVerdict(0, 4000); !strings.Contains(got, "unreadable") {
		t.Fatalf("zero misjudged:\n%s", got)
	}
}

// TestAuditOpenFilesLimit_Unsupported proves non-MySQL engines get an
// explicit error.
func TestAuditOpenFilesLimit_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditOpenFilesLimit(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
