package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/FreePeak/db-mcp-server/pkg/dbtools"
)

// TestAnalyzePerformance_SuggestDetectsKnownIssues locks in the suggest
// action against the real SQL issue detector.
func TestAnalyzePerformance_SuggestDetectsKnownIssues(t *testing.T) {
	uc := NewDatabaseUseCase(&fakeRepo{db: &fakeDB{}})

	out, err := uc.AnalyzePerformance(context.Background(), "pg1", "suggest", "SELECT * FROM users WHERE a = 1 OR b = 2", 0, 0)
	if err != nil {
		t.Fatalf("suggest failed: %v", err)
	}
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "select-star") && !strings.Contains(lower, "or-in-where") {
		t.Fatalf("expected known issues in output, got:\n%s", out)
	}
}

// TestAnalyzePerformance_StatsAfterTracking verifies stats reflect real
// tracked query executions.
func TestAnalyzePerformance_StatsAfterTracking(t *testing.T) {
	analyzer := dbtools.GetPerformanceAnalyzer()
	analyzer.Reset()

	_, _ = analyzer.TrackQuery(context.Background(), "SELECT id FROM orders", nil, func() (interface{}, error) {
		time.Sleep(5 * time.Millisecond)
		return nil, nil
	})

	uc := NewDatabaseUseCase(&fakeRepo{db: &fakeDB{}})
	out, err := uc.AnalyzePerformance(context.Background(), "pg1", "stats", "", 0, 0)
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if !strings.Contains(out, "COUNT") || !strings.Contains(out, "orders") {
		t.Fatalf("expected tracked query metrics, got:\n%s", out)
	}
	analyzer.Reset()
}

// TestAnalyzePerformance_SlowQueriesAndReset covers the slow-query listing
// and the reset action end-to-end through the analyzer.
func TestAnalyzePerformance_SlowQueriesAndReset(t *testing.T) {
	analyzer := dbtools.GetPerformanceAnalyzer()
	analyzer.Reset()
	analyzer.SetSlowThreshold(1 * time.Millisecond)

	_, _ = analyzer.TrackQuery(context.Background(), "SELECT slow_one", nil, func() (interface{}, error) {
		time.Sleep(3 * time.Millisecond)
		return nil, nil
	})
	analyzer.TrackQuery(context.Background(), "INSERT INTO t VALUES (1)", nil, func() (interface{}, error) {
		time.Sleep(3 * time.Millisecond) // slow AND failing
		return nil, context.DeadlineExceeded
	})

	uc := NewDatabaseUseCase(&fakeRepo{db: &fakeDB{}})
	out, err := uc.AnalyzePerformance(context.Background(), "pg1", "slow_queries", "", 10, 0)
	if err != nil {
		t.Fatalf("slow_queries failed: %v", err)
	}
	if !strings.Contains(out, "slow_one") {
		t.Fatalf("expected the slow query to be listed, got:\n%s", out)
	}
	if !strings.Contains(out, "error") {
		t.Fatalf("expected recorded error to surface, got:\n%s", out)
	}

	resetOut, err := uc.AnalyzePerformance(context.Background(), "pg1", "reset", "", 0, 0)
	if err != nil {
		t.Fatalf("reset failed: %v", err)
	}
	if !strings.Contains(resetOut, "reset") {
		t.Fatalf("unexpected reset output: %s", resetOut)
	}
	if after, _ := uc.AnalyzePerformance(context.Background(), "pg1", "slow_queries", "", 10, 0); strings.Contains(after, "slow_one") {
		t.Fatalf("history should be empty after reset, got:\n%s", after)
	}
}

// TestAnalyzePerformance_InvalidAction ensures unknown actions fail clearly.
func TestAnalyzePerformance_InvalidAction(t *testing.T) {
	uc := NewDatabaseUseCase(&fakeRepo{db: &fakeDB{}})
	if _, err := uc.AnalyzePerformance(context.Background(), "pg1", "bogus", "", 0, 0); err == nil {
		t.Fatal("expected error for invalid action")
	}
}
