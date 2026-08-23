package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestStatisticsTargetProbe proves the probe targets PostgreSQL only.
func TestStatisticsTargetProbe(t *testing.T) {
	if q := statisticsTargetProbe("postgres"); !strings.Contains(q, "default_statistics_target") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if statisticsTargetProbe("mysql") != "" || statisticsTargetProbe("sqlite") != "" {
		t.Fatal("only postgres exposes default_statistics_target")
	}
}

// TestStatisticsTargetVerdict proves the escalation ladder.
func TestStatisticsTargetVerdict(t *testing.T) {
	if got := statisticsTargetVerdict("500"); got != "" {
		t.Fatalf("tuned value must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := statisticsTargetVerdict("100")
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "default") {
		t.Fatalf("default not escalated:\n%s", got)
	}
	if !strings.Contains(got, "ALTER SYSTEM SET default_statistics_target='250'") || !strings.Contains(got, "ANALYZE") {
		t.Fatalf("verdict must name the fix path including the re-ANALYZE step, got:\n%s", got)
	}
	if got := statisticsTargetVerdict(""); !strings.Contains(got, "unreadable") {
		t.Fatalf("empty/unreadable misjudged:\n%s", got)
	}
	if got := statisticsTargetVerdict("abc"); !strings.Contains(got, "unreadable") {
		t.Fatalf("non-numeric misjudged:\n%s", got)
	}
}

// TestAuditStatisticsTarget_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditStatisticsTarget_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditStatisticsTarget(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
