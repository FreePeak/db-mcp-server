package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/FreePeak/cortex/pkg/types"
)

// estimateToolTokens approximates the context-window cost of a tool
// definition using the industry-standard ~4 characters per token heuristic.
func estimateToolTokens(t *testing.T, tool interface{}) int {
	t.Helper()
	typed, ok := tool.(*types.Tool)
	if !ok {
		t.Fatalf("expected *types.Tool, got %T", tool)
	}
	payload := map[string]interface{}{
		"name":        typed.Name,
		"description": typed.Description,
		"parameters":  typed.Parameters,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	return len(b) / 4
}

// TestToolTokenBenchmark measures the agent-context cost of the two
// registration modes across database counts. Unified mode must stay cheap
// regardless of how many databases are connected; per-database mode scales
// linearly. Numbers feed docs/cycles/cycle-12.
func TestToolTokenBenchmark(t *testing.T) {
	toolTypes := []string{"query", "execute", "transaction", "performance", "explain", "describe", "schema", "filter_tables"}
	factory := NewToolTypeFactory()

	for _, n := range []int{1, 3, 10} {
		dbList := make([]string, n)
		for i := range dbList {
			dbList[i] = fmt.Sprintf("db%d", i+1)
		}

		unifiedTokens := 0
		for _, typeName := range toolTypes {
			toolType, ok := factory.GetToolType(typeName)
			if !ok {
				continue
			}
			unifiedTokens += estimateToolTokens(t, toolType.CreateUnifiedTool(strings.ReplaceAll(typeName, "_", "-"), dbList))
		}

		perDBTokens := 0
		for _, dbID := range dbList {
			for _, typeName := range toolTypes {
				toolType, ok := factory.GetToolType(typeName)
				if !ok {
					continue
				}
				name := fmt.Sprintf("%s_%s", typeName, dbID)
				perDBTokens += estimateToolTokens(t, toolType.CreateTool(name, dbID))
			}
		}

		ratio := 0.0
		if unifiedTokens > 0 {
			ratio = float64(perDBTokens) / float64(unifiedTokens)
		}
		t.Logf("databases=%d unified=%dtokens per_db=%dtokens ratio=%.1fx", n, unifiedTokens, perDBTokens, ratio)

		// Guardrail: unified surface stays lean no matter the fleet size,
		// and per-database mode must cost meaningfully more at scale.
		if unifiedTokens > 8000 {
			t.Errorf("unified surface too expensive: %d tokens", unifiedTokens)
		}
		if n >= 10 && ratio < 4.0 {
			t.Errorf("per-db mode expected to cost >=4x unified at %d databases; got %.1fx", n, ratio)
		}
	}
}
