package usecase

import (
	"strings"
	"testing"
)

// TestAVWorkersProbe covers cycle 200: the probe reads
// autovacuum_max_workers on PostgreSQL only.
func TestAVWorkersProbe(t *testing.T) {
	if !strings.Contains(avWorkersProbe("postgres"), "autovacuum_max_workers") {
		t.Fatal("postgres probe must read autovacuum_max_workers")
	}
	if avWorkersProbe("mysql") != "" || avWorkersProbe("sqlite") != "" {
		t.Fatal("only postgres exposes autovacuum_max_workers")
	}
}

// TestAVWorkersVerdict proves the escalation ladder: the PostgreSQL
// default of 3 workers starves vacuum on busy clusters (each database
// waits its turn), so it warns; 5+ stays quiet.
func TestAVWorkersVerdict(t *testing.T) {
	tests := []struct {
		workers int
		want    string // substring required; "" = must be silent
	}{
		{3, "WARNING"},
		{1, "WARNING"},
		{2, "WARNING"},
		{4, ""},
		{8, ""},
	}
	for _, tt := range tests {
		got := avWorkersVerdict(tt.workers)
		if tt.want == "" && got != "" {
			t.Fatalf("%d workers must stay quiet, got:\n%s", tt.workers, got)
		}
		if tt.want != "" && !strings.Contains(got, tt.want) {
			t.Fatalf("%d workers not escalated:\n%s", tt.workers, got)
		}
	}

	got := avWorkersVerdict(3)
	for _, want := range []string{"autovacuum_max_workers", "ALTER SYSTEM SET", "max_worker_processes"} {
		if !strings.Contains(got, want) {
			t.Fatalf("verdict missing %q:\n%s", want, got)
		}
	}

	if blank := avWorkersVerdict(0); !strings.Contains(blank, "unreadable") {
		t.Fatalf("zero/unreadable misjudged:\n%s", blank)
	}
}
