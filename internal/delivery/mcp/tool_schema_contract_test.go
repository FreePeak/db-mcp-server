package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestToolSchemas_DocumentedActionsLocked locks the advertised tool surface
// to what README/CHANGELOG document: suggest_indexes must appear on the
// performance tools and mask_pii on the query tools (per-db + unified).
func TestToolSchemas_DocumentedActionsLocked(t *testing.T) {
	cases := []struct {
		name     string
		tool     interface{}
		contains []string
	}{
		{"performance_perdb", NewPerformanceTool().CreateTool("perf_db1", "db1"), []string{"suggest_indexes"}},
		{"performance_unified", NewPerformanceTool().CreateUnifiedTool("performance", []string{"db1"}), []string{"suggest_indexes"}},
		{"execute_perdb", NewExecuteTool().CreateTool("execute_db1", "db1"), []string{"dry_run"}},
		{"execute_unified", NewExecuteTool().CreateUnifiedTool("execute", []string{"db1"}), []string{"dry_run"}},
		{"query_perdb", NewQueryTool().CreateTool("query_db1", "db1"), []string{"mask_pii", "verbosity"}},
		{"query_unified", NewQueryTool().CreateUnifiedTool("query", []string{"db1"}), []string{"mask_pii", "verbosity"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.tool)
			if err != nil {
				t.Fatalf("marshal schema failed: %v", err)
			}
			schema := string(raw)
			for _, want := range tc.contains {
				if !strings.Contains(schema, want) {
					t.Fatalf("schema missing %q — update the tool definition or the docs together:\n%s", want, schema)
				}
			}
		})
	}
}
