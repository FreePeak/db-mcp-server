package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestTempFileLimitProbe proves the probe reads the setting in
// kilobytes and targets PostgreSQL only.
func TestTempFileLimitProbe(t *testing.T) {
	q := tempFileLimitProbe("postgres")
	if !strings.Contains(q, "current_setting('temp_file_limit')") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if tempFileLimitProbe("mysql") != "" || tempFileLimitProbe("sqlite") != "" {
		t.Fatal("only postgres exposes temp_file_limit")
	}
}

// TestTempFileLimitVerdict proves the escalation ladder.
func TestTempFileLimitVerdict(t *testing.T) {
	if got := tempFileLimitVerdict(10 * 1024 * 1024); got != "" {
		t.Fatalf("bounded limit must render empty (audit adds the clean line), got:\n%s", got)
	}
	unlimited := tempFileLimitVerdict(-1)
	if !strings.Contains(unlimited, "WARNING") || !strings.Contains(unlimited, "disk") {
		t.Fatalf("unlimited not escalated:\n%s", unlimited)
	}
	if !strings.Contains(unlimited, "ALTER SYSTEM SET temp_file_limit") {
		t.Fatalf("verdict must name the fix, got:\n%s", unlimited)
	}
	if got := tempFileLimitVerdict(0); !strings.Contains(got, "unlimited") {
		t.Fatalf("zero must render as unlimited, got:\n%s", got)
	}
}

// TestAuditTempFileLimit_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditTempFileLimit_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditTempFileLimit(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
