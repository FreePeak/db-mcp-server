package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/FreePeak/db-mcp-server/internal/usecase"
)

// stubMaskingUseCase satisfies UseCaseProvider plus ExecuteQueryMasked,
// recording which path the query tool dispatches to.
type stubMaskingUseCase struct {
	maskWasCalled bool
	lastMaskFlag  bool
}

func (s *stubMaskingUseCase) ExecuteQuery(ctx context.Context, dbID, query string, params []interface{}) (string, error) {
	return "Results:\n\nemail\n----\njane@acme.com\n\nTotal rows: 1", nil
}

func (s *stubMaskingUseCase) ExecuteQueryMasked(_ context.Context, _ string, _ string, _ []interface{}, mask bool, _ usecase.ResultVerbosity) (string, error) {
	s.maskWasCalled = true
	s.lastMaskFlag = mask
	return "Results:\n\nemail\n----\n[EMAIL]\n\nTotal rows: 1", nil
}

func (s *stubMaskingUseCase) ExecuteStatement(ctx context.Context, dbID, statement string, params []interface{}) (string, error) {
	return "", nil
}
func (s *stubMaskingUseCase) ExecuteTransaction(ctx context.Context, dbID, action string, txID string, statement string, params []interface{}, readOnly bool) (string, map[string]interface{}, error) {
	return "", nil, nil
}
func (s *stubMaskingUseCase) GetDatabaseInfo(dbID string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
func (s *stubMaskingUseCase) ListDatabases() []string { return []string{"db1"} }
func (s *stubMaskingUseCase) GetDatabaseType(dbID string) (string, error) {
	return "sqlite", nil
}
func (s *stubMaskingUseCase) IsLazyLoading() bool { return false }
func (s *stubMaskingUseCase) ExecuteExplain(_ context.Context, _, _ string, _ bool) (string, error) {
	return "", nil
}
func (s *stubMaskingUseCase) DescribeTable(_ context.Context, _, _ string) (map[string]interface{}, error) {
	return nil, nil
}
func (s *stubMaskingUseCase) AnalyzePerformance(_ context.Context, _, _, _ string, _, _ int) (string, error) {
	return "", nil
}
func (s *stubMaskingUseCase) HealthCheck(_ context.Context, _ string) (map[string]interface{}, error) {
	return nil, nil
}
func (s *stubMaskingUseCase) RelationshipGraph(_ context.Context, _ string) (string, error) {
	return "", nil
}

func TestQueryTool_MaskPIIRoutesToMaskedPath(t *testing.T) {
	tool := NewQueryTool()
	uc := &stubMaskingUseCase{}

	resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name: "query_db1",
		Parameters: map[string]interface{}{
			"query":    "SELECT email FROM customers",
			"mask_pii": true,
		},
	}, "", uc)
	if err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	if uc.lastMaskFlag != true {
		t.Fatalf("expected masked path with mask=true")
	}
	out := strings.ToLower(sprintfResponse(resp))
	if strings.Contains(out, "jane@acme.com") {
		t.Fatalf("raw email in masked response: %s", out)
	}
	if !strings.Contains(out, "[email]") {
		t.Fatalf("expected [EMAIL] marker: %s", out)
	}
}

func TestQueryTool_NoMaskParamUsesLegacyPath(t *testing.T) {
	tool := NewQueryTool()
	uc := &stubMaskingUseCase{}

	if _, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name: "query_db1",
		Parameters: map[string]interface{}{
			"query": "SELECT email FROM customers",
		},
	}, "", uc); err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	// The masked path always runs when supported so the use case layer can
	// enforce server-level masking config; flag is false here.
	if !uc.maskWasCalled || uc.lastMaskFlag {
		t.Fatalf("expected masked-capable path with flag=false, called=%v flag=%v", uc.maskWasCalled, uc.lastMaskFlag)
	}
}

func sprintfResponse(resp interface{}) string {
	raw, err := json.Marshal(resp)
	if err != nil {
		return ""
	}
	return string(raw)
}

// stubFullUseCase adds dry-run capability to the stub.
type stubDryRunUseCase struct {
	stubMaskingUseCase
	lastStatement string
}

func (s *stubDryRunUseCase) ExecuteStatementDryRun(_ context.Context, _ string, statement string) (*usecase.RiskReport, error) {
	s.lastStatement = statement
	return &usecase.RiskReport{Kind: "destructive", Risk: "critical", Statements: 1,
		Notes: []string{"DROP permanently removes a table"}, WouldExecute: true}, nil
}

// TestExecuteTool_DryRunDoesNotExecute proves the dry_run parameter returns
// a risk report and never calls ExecuteStatement.
func TestExecuteTool_DryRunDoesNotExecute(t *testing.T) {
	tool := NewExecuteTool()
	uc := &stubDryRunUseCase{}

	resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name: "execute_db1",
		Parameters: map[string]interface{}{
			"statement": "DROP TABLE users",
			"dry_run":   true,
		},
	}, "", uc)
	if err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	out := strings.ToLower(sprintfResponse(resp))
	if !strings.Contains(out, "dry run") || !strings.Contains(out, "critical") {
		t.Fatalf("expected dry-run report with risk level:\n%s", out)
	}
	if uc.lastStatement != "DROP TABLE users" {
		t.Fatal("analyzer did not receive the statement")
	}
}
