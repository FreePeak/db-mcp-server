package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
)

// stubReadModes satisfies UseCaseProvider plus the cycle 75-88 capability
// interfaces; it records which route fired.
type stubReadModes struct {
	stubMaskingUseCase
	route string
}

func (s *stubReadModes) ExecuteQueryAcross(_ context.Context, _ string, dbIDs []string) (string, error) {
	s.route = "across:" + strings.Join(dbIDs, ",")
	return "across-out", nil
}
func (s *stubReadModes) ExecuteQuerySample(_ context.Context, _, _ string, _ []interface{}, n int) (string, error) {
	s.route = "sample"
	return "sample-out", nil
}
func (s *stubReadModes) ExecuteQueryPage(_ context.Context, _, _ string, _ []interface{}, page, size int) (string, int64, error) {
	s.route = "page"
	return "page-out", 0, nil
}

func runQuery(t *testing.T, params map[string]interface{}) *stubReadModes {
	t.Helper()
	uc := &stubReadModes{}
	tool := NewQueryTool()
	if _, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name:       "query_db1",
		Parameters: params,
	}, "", uc); err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	return uc
}

// TestQueryToolRouting proves cycles 75-88 wiring: each read-mode param
// routes to its capability method, and plain queries stay on ExecuteQuery.
func TestQueryToolRouting(t *testing.T) {
	if uc := runQuery(t, map[string]interface{}{"query": "SELECT 1"}); uc.route != "" {
		t.Fatalf("plain query misrouted to %q", uc.route)
	}

	if uc := runQuery(t, map[string]interface{}{"query": "SELECT 1", "databases": "a,b"}); uc.route != "across:a,b" {
		t.Fatalf("databases= not routed: %q", uc.route)
	}

	if uc := runQuery(t, map[string]interface{}{"query": "SELECT 1", "sample_rows": float64(5)}); uc.route != "sample" {
		t.Fatalf("sample_rows not routed: %q", uc.route)
	}

	if uc := runQuery(t, map[string]interface{}{"query": "SELECT 1", "page": float64(2), "page_size": float64(10)}); uc.route != "page" {
		t.Fatalf("page not routed: %q", uc.route)
	}

	// Single database in databases= must NOT trigger fan-out.
	if uc := runQuery(t, map[string]interface{}{"query": "SELECT 1", "databases": "only"}); uc.route != "" {
		t.Fatalf("single-database fan-out triggered: %q", uc.route)
	}
}
