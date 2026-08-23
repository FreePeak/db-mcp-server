package usecase

import (
	"strings"
	"testing"
	"time"
)

// TestGrowthRate proves the per-day projection helper: positive deltas
// over multi-day windows project rows/day; sub-day windows and
// shrinkage stay unprojected rather than inventing noise.
func TestGrowthRate(t *testing.T) {
	fiveDays := 5 * 24 * time.Hour
	if got := growthRate(400, fiveDays); got != " (+80/day)" {
		t.Fatalf("got %q", got)
	}
	if got := growthRate(-50, fiveDays); got != "" {
		t.Fatalf("shrinkage should not project, got %q", got)
	}
	if got := growthRate(0, fiveDays); got != "" {
		t.Fatalf("zero delta should not project, got %q", got)
	}
	if got := growthRate(100, 2*time.Hour); got != "" {
		t.Fatalf("sub-day window too noisy to project, got %q", got)
	}
}

// TestBaselineHeader proves the compare report states the baseline's
// age so stale baselines are obvious.
func TestBaselineHeader(t *testing.T) {
	got := baselineHeader("db1", 3, 72*time.Hour)
	for _, want := range []string{"db1", "3 day(s)", "3 table(s)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}
