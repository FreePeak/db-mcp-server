package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FreePeak/db-mcp-server/internal/usecase"
)

// dryRunBaseProvider satisfies the parts of UseCaseProvider this test never
// calls, so the stubs below only define what the dispatch decision touches.
type dryRunBaseProvider struct{}

func (dryRunBaseProvider) ExecuteQuery(context.Context, string, string, []interface{}) (string, error) {
	return "", nil
}
func (dryRunBaseProvider) ExecuteStatement(context.Context, string, string, []interface{}) (string, error) {
	return "", nil
}
func (dryRunBaseProvider) ExecuteTransaction(context.Context, string, string, string, string, []interface{}, bool) (string, map[string]interface{}, error) {
	return "", nil, nil
}
func (dryRunBaseProvider) GetDatabaseInfo(string) (map[string]interface{}, error) { return nil, nil }
func (dryRunBaseProvider) ListDatabases() []string                                { return nil }
func (dryRunBaseProvider) GetDatabaseType(string) (string, error)                 { return "sqlite", nil }
func (dryRunBaseProvider) IsLazyLoading() bool                                    { return false }
func (dryRunBaseProvider) ExecuteExplain(context.Context, string, string, bool) (string, error) {
	return "", nil
}
func (dryRunBaseProvider) DescribeTable(context.Context, string, string) (map[string]interface{}, error) {
	return nil, nil
}
func (dryRunBaseProvider) AnalyzePerformance(context.Context, string, string, string, int, int) (string, error) {
	return "", nil
}
func (dryRunBaseProvider) HealthCheck(context.Context, string) (map[string]interface{}, error) {
	return nil, nil
}
func (dryRunBaseProvider) RelationshipGraph(context.Context, string) (string, error) { return "", nil }

// stubExecuteUseCase implements UseCaseProvider plus the dry-run capability,
// counting which path the execute tool dispatches to.
type stubExecuteUseCase struct {
	dryRunBaseProvider
	execCalls int
	dryCalls  int
}

func (s *stubExecuteUseCase) ExecuteStatement(_ context.Context, _, _ string, _ []interface{}) (string, error) {
	s.execCalls++
	return "Statement executed successfully.\nRows affected: 1", nil
}

func (s *stubExecuteUseCase) ExecuteStatementDryRun(_ context.Context, dbID, statement string) (*usecase.RiskReport, error) {
	s.dryCalls++
	return &usecase.RiskReport{
		Kind:         "destructive",
		Risk:         "critical",
		Statements:   1,
		MissingWhere: false,
		Notes:        []string{"DROP permanently removes table and its data"},
	}, nil
}

// TestExecuteTool_DryRunNeverExecutes is the property that matters: asking for
// a dry run must not reach ExecuteStatement, whatever the statement says.
func TestExecuteTool_DryRunNeverExecutes(t *testing.T) {
	uc := &stubExecuteUseCase{}
	tool := &ExecuteTool{BaseToolType: BaseToolType{name: "execute"}}

	resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name: "execute_db1",
		Parameters: map[string]interface{}{
			"statement": "DROP TABLE users",
			"dry_run":   true,
		},
	}, "db1", uc)
	require.NoError(t, err)

	assert.Equal(t, 1, uc.dryCalls, "dry_run must route to the classifier")
	assert.Zero(t, uc.execCalls, "dry_run must never execute the statement")

	text := dryRunResponseText(t, resp)
	assert.Contains(t, text, "DRY RUN")
	assert.Contains(t, text, "Critical")
	assert.Contains(t, text, "nothing was executed")
	assert.Contains(t, text, "Re-run without dry_run")
}

func TestExecuteTool_WithoutDryRunExecutes(t *testing.T) {
	uc := &stubExecuteUseCase{}
	tool := &ExecuteTool{BaseToolType: BaseToolType{name: "execute"}}

	resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name:       "execute_db1",
		Parameters: map[string]interface{}{"statement": "INSERT INTO t VALUES (1)"},
	}, "db1", uc)
	require.NoError(t, err)

	assert.Equal(t, 1, uc.execCalls)
	assert.Zero(t, uc.dryCalls)
	assert.Contains(t, dryRunResponseText(t, resp), "Statement executed successfully")
}

// TestExecuteTool_DryRunUnsupportedProvider: a provider without the capability
// must say so rather than silently executing the statement.
func TestExecuteTool_DryRunUnsupportedProvider(t *testing.T) {
	uc := &stubPlainExecuteUseCase{}
	tool := &ExecuteTool{BaseToolType: BaseToolType{name: "execute"}}

	_, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name: "execute_db1",
		Parameters: map[string]interface{}{
			"statement": "DROP TABLE users",
			"dry_run":   true,
		},
	}, "db1", uc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dry_run is not supported")
	assert.Zero(t, uc.execCalls, "an unsupported dry_run must not fall through to execution")
}

type stubPlainExecuteUseCase struct {
	dryRunBaseProvider
	execCalls int
}

func (s *stubPlainExecuteUseCase) ExecuteStatement(_ context.Context, _, _ string, _ []interface{}) (string, error) {
	s.execCalls++
	return "executed", nil
}

// dryRunResponseText extracts the text payload from the delivery response.
func dryRunResponseText(t *testing.T, resp interface{}) string {
	t.Helper()
	v, ok := resp.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected response shape %T", resp)
	}
	switch c := v["content"].(type) {
	case []map[string]interface{}:
		if len(c) > 0 {
			if s, ok := c[0]["text"].(string); ok {
				return s
			}
		}
	case []interface{}:
		if len(c) > 0 {
			if m, ok := c[0].(map[string]interface{}); ok {
				if s, ok := m["text"].(string); ok {
					return s
				}
			}
		}
	}
	t.Fatalf("no text payload in response: %#v", resp)
	return ""
}

// TestExecuteTool_DryRunSchemaDeclaresParameter keeps the tool contract honest:
// a parameter the handler reads must be advertised to the model.
func TestExecuteTool_DryRunSchemaDeclaresParameter(t *testing.T) {
	tool := &ExecuteTool{BaseToolType: BaseToolType{name: "execute"}}
	for _, shape := range []interface{}{
		tool.CreateTool("execute_db1", "db1"),
		tool.CreateUnifiedTool("execute", []string{"db1"}),
	} {
		raw, err := json.Marshal(shape)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "dry_run")
	}
}
