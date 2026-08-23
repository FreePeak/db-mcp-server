package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestLogBufferProbe proves the probe targets MySQL only and pulls
// both the setting and its wait-counter evidence.
func TestLogBufferProbe(t *testing.T) {
	q := logBufferProbe("mysql")
	if !strings.Contains(q, "innodb_log_buffer_size") || !strings.Contains(q, "Innodb_log_waits") {
		t.Fatalf("probe must fetch setting + evidence:\n%s", q)
	}
	if logBufferProbe("postgres") != "" || logBufferProbe("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose innodb_log_buffer_size")
	}
}

// TestLogBufferSizeVerdict proves the escalation ladder.
func TestLogBufferSizeVerdict(t *testing.T) {
	if got := logBufferVerdict(128*1024*1024, 0); got != "" {
		t.Fatalf("healthy config must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := logBufferVerdict(16*1024*1024, 42)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "Innodb_log_waits=42") {
		t.Fatalf("waits not escalated:\n%s", got)
	}
	if !strings.Contains(got, "innodb_log_buffer_size='64M'") {
		t.Fatalf("warning must name the live fix, got:\n%s", got)
	}
	if got := logBufferVerdict(8*1024*1024, 0); !strings.Contains(got, "below the 16M default") {
		t.Fatalf("shrunken-but-clean config misjudged:\n%s", got)
	}
	if got := logBufferVerdict(0, 5); !strings.Contains(got, "unreadable") {
		t.Fatalf("zero/unreadable misjudged:\n%s", got)
	}
}

// TestAuditLogBuffer_Unsupported proves non-MySQL engines get an
// explicit error.
func TestAuditLogBuffer_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditLogBuffer(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
