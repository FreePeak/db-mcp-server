package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
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

func (s *stubMaskingUseCase) ExecuteQueryMasked(_ context.Context, _ string, _ string, _ []interface{}, mask bool) (string, error) {
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
	if uc.maskWasCalled {
		t.Fatal("legacy path should be used when mask_pii is absent")
	}
}

func sprintfResponse(resp interface{}) string {
	raw, err := json.Marshal(resp)
	if err != nil {
		return ""
	}
	return string(raw)
}
