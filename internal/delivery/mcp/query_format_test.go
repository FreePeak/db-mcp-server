package mcp

import (
	"context"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/FreePeak/db-mcp-server/internal/usecase"
)

type stubExportUseCase struct {
	UseCaseProvider
	gotFormat string
}

func (s *stubExportUseCase) ExecuteQueryMasked(_ context.Context, _, _ string, _ []interface{}, _ bool, _ usecase.ResultVerbosity) (string, error) {
	return "TEXT", nil
}

func (s *stubExportUseCase) ExecuteQueryFormat(_ context.Context, _, _ string, _ []interface{}, format string) (string, error) {
	s.gotFormat = format
	return "EXPORTED", nil
}

func TestQueryTool_FormatRouting(t *testing.T) {
	tool := NewQueryTool()
	uc := &stubExportUseCase{}

	req := server.ToolCallRequest{
		Name: "query_db1",
		Parameters: map[string]interface{}{
			"query":  "SELECT 1",
			"format": "csv",
		},
	}
	resp, err := tool.HandleRequest(context.Background(), req, "db1", uc)
	if err != nil {
		t.Fatalf("csv request failed: %v", err)
	}
	if uc.gotFormat != "csv" {
		t.Fatalf("csv not routed to export path (gotFormat=%q)", uc.gotFormat)
	}
	if txt, ok := resp.(map[string]interface{}); ok {
		_ = txt // shape asserted elsewhere; routing is the contract here
	}

	uc.gotFormat = ""
	req.Parameters["format"] = ""
	if _, err := tool.HandleRequest(context.Background(), req, "db1", uc); err != nil {
		t.Fatalf("text request failed: %v", err)
	}
	if uc.gotFormat != "" {
		t.Fatalf("empty format must stay on text path (gotFormat=%q)", uc.gotFormat)
	}
}
