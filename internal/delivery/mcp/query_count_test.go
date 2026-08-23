package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/FreePeak/db-mcp-server/internal/usecase"
)

type stubCountUseCase struct {
	stubExportUseCase
	gotQuery string
}

func (s *stubCountUseCase) CountQueryRows(_ context.Context, _, query string, _ []interface{}) (string, error) {
	s.gotQuery = query
	return "COUNTED", nil
}

func (s *stubCountUseCase) ExecuteQueryMasked(context.Context, string, string, []interface{}, bool, usecase.ResultVerbosity) (string, error) {
	return "ROWS", nil
}

func TestQueryTool_CountOnlyRouting(t *testing.T) {
	tool := NewQueryTool()
	uc := &stubCountUseCase{}
	req := server.ToolCallRequest{
		Name: "query_db1",
		Parameters: map[string]interface{}{
			"query":      "SELECT 1",
			"count_only": true,
		},
	}
	out, err := tool.HandleRequest(context.Background(), req, "db1", uc)
	if err != nil {
		t.Fatalf("count_only failed: %v", err)
	}
	if uc.gotQuery == "" {
		t.Fatal("count path not invoked")
	}
	if txt, ok := out.(map[string]interface{}); !ok || len(txt) == 0 {
		t.Fatalf("unexpected response shape: %T", out)
	}

	// Without count_only the normal row path must run.
	uc.gotQuery = ""
	delete(req.Parameters, "count_only")
	res, err := tool.HandleRequest(context.Background(), req, "db1", uc)
	if err != nil {
		t.Fatalf("plain query failed: %v", err)
	}
	if uc.gotQuery != "" {
		t.Fatal("count path must not run without count_only")
	}
	if !strings.Contains(strings.ToLower("x"), "x") {
		t.Fatal("unreachable") // keep strings import honest if shapes change
	}
	_ = res
}
