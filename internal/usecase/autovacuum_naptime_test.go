package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestAutovacuumNaptimeProbe proves the probe reads the setting and
// targets PostgreSQL only.
func TestAutovacuumNaptimeProbe(t *testing.T) {
	q := autovacuumNaptimeProbe("postgres")
	if !strings.Contains(q, "current_setting('autovacuum_naptime')") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if autovacuumNaptimeProbe("mysql") != "" || autovacuumNaptimeProbe("sqlite") != "" {
		t.Fatal("only postgres exposes autovacuum_naptime")
	}
}

// TestAutovacuumNaptimeVerdict proves the escalation ladder.
func TestAutovacuumNaptimeVerdict(t *testing.T) {
	if got := autovacuumNaptimeVerdict(60); got != "" {
		t.Fatalf("default 60s must render empty (audit adds the clean line), got:\n%s", got)
	}
	if got := autovacuumNaptimeVerdict(300); got != "" {
		t.Fatalf("exactly 300s must stay quiet, got:\n%s", got)
	}
	got := autovacuumNaptimeVerdict(600)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "10m0s") {
		t.Fatalf("600s not escalated:\n%s", got)
	}
	for _, want := range []string{"bloat", "ALTER SYSTEM SET autovacuum_naptime", "per-database"} {
		if !strings.Contains(got, want) {
			t.Fatalf("verdict missing %q:\n%s", want, got)
		}
	}
	blank := autovacuumNaptimeVerdict(0)
	if !strings.Contains(blank, "unreadable") {
		t.Fatalf("zero misjudged:\n%s", blank)
	}
}

// TestAuditAutovacuumNaptime_Unsupported proves non-PG engines get
// an explicit error.
func TestAuditAutovacuumNaptime_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditAutovacuumNaptime(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}

// TestParseSecondsSetting proves the seconds parser handles both
// bare numbers and suffixed intervals.
func TestParseSecondsSetting(t *testing.T) {
	cases := map[string]int{"60": 60, "1min": 60, "2 min": 120, "90s": 90, "": 0, "junk": 0}
	for in, want := range cases {
		if got := parseSecondsSetting(in); got != want {
			t.Fatalf("parseSecondsSetting(%q) = %d, want %d", in, got, want)
		}
	}
}
