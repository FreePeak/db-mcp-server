package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestBackLogProbe proves the probe pairs the setting with
// max_connections and targets MySQL only.
func TestBackLogProbe(t *testing.T) {
	q := backLogProbe("mysql")
	for _, frag := range []string{"back_log", "max_connections"} {
		if !strings.Contains(q, frag) {
			t.Fatalf("probe missing %s:\n%s", frag, q)
		}
	}
	if backLogProbe("postgres") != "" || backLogProbe("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose back_log")
	}
}

// TestBackLogVerdict proves the escalation ladder.
func TestBackLogVerdict(t *testing.T) {
	if got := backLogVerdict(500); got != "" {
		t.Fatalf("healthy must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := backLogVerdict(-1)
	if !strings.Contains(got, "autosized") || strings.Contains(got, "WARNING") {
		t.Fatalf("autosize sentinel misjudged:\n%s", got)
	}
	low := backLogVerdict(10)
	if !strings.Contains(low, "WARNING") || !strings.Contains(low, "before authentication") {
		t.Fatalf("low value not escalated:\n%s", low)
	}
	if !strings.Contains(low, "restart") {
		t.Fatalf("verdict must name the restart-required fix, got:\n%s", low)
	}
	if got := backLogVerdict(0); !strings.Contains(got, "unreadable") {
		t.Fatalf("zero/unreadable misjudged:\n%s", got)
	}
}

// TestAuditBackLog_Unsupported proves non-MySQL engines get an
// explicit error.
func TestAuditBackLog_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditBackLog(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
