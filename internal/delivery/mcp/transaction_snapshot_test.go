package mcp

import (
	"context"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FreePeak/db-mcp-server/internal/usecase"
)

// snapshotBaseProvider satisfies the parts of UseCaseProvider these tests never
// call, so the stubs below only define what the dispatch decision touches.
type snapshotBaseProvider struct{}

func (snapshotBaseProvider) ExecuteQuery(context.Context, string, string, []interface{}) (string, error) {
	return "", nil
}
func (snapshotBaseProvider) ExecuteStatement(context.Context, string, string, []interface{}) (string, error) {
	return "", nil
}
func (snapshotBaseProvider) ExecuteTransaction(context.Context, string, string, string, string, []interface{}, bool) (string, map[string]interface{}, error) {
	return "", nil, nil
}
func (snapshotBaseProvider) GetDatabaseInfo(string) (map[string]interface{}, error) { return nil, nil }
func (snapshotBaseProvider) ListDatabases() []string                                { return nil }
func (snapshotBaseProvider) GetDatabaseType(string) (string, error)                 { return "sqlite", nil }
func (snapshotBaseProvider) IsLazyLoading() bool                                    { return false }
func (snapshotBaseProvider) ExecuteExplain(context.Context, string, string, bool) (string, error) {
	return "", nil
}
func (snapshotBaseProvider) DescribeTable(context.Context, string, string) (map[string]interface{}, error) {
	return nil, nil
}
func (snapshotBaseProvider) AnalyzePerformance(context.Context, string, string, string, int, int) (string, error) {
	return "", nil
}
func (snapshotBaseProvider) HealthCheck(context.Context, string) (map[string]interface{}, error) {
	return nil, nil
}
func (snapshotBaseProvider) RelationshipGraph(context.Context, string) (string, error) {
	return "", nil
}

// stubSnapshotUseCase implements UseCaseProvider plus the snapshot capability,
// recording which snapshot id a rollback was asked to restore.
type stubSnapshotUseCase struct {
	snapshotBaseProvider
	rolledBack string
	txCalls    int
}

func (s *stubSnapshotUseCase) ListSnapshots(string) []usecase.MutationSnapshot {
	return []usecase.MutationSnapshot{
		{ID: "snap_1", Kind: "delete", Table: "users", Rows: []map[string]any{{"id": 1}}},
	}
}

func (s *stubSnapshotUseCase) RollbackSnapshot(_ context.Context, _, snapshotID string) (string, error) {
	s.rolledBack = snapshotID
	return "Restored 1 row(s) in users from snapshot " + snapshotID + ".", nil
}

func (s *stubSnapshotUseCase) ExecuteTransaction(context.Context, string, string, string, string, []interface{}, bool) (string, map[string]interface{}, error) {
	s.txCalls++
	return "transaction handled", nil, nil
}

// TestTransactionTool_ListSnapshots proves the action surfaces registered
// snapshots with their audit context.
func TestTransactionTool_ListSnapshots(t *testing.T) {
	uc := &stubSnapshotUseCase{}
	tool := &TransactionTool{BaseToolType: BaseToolType{name: "transaction"}}

	resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name:       "transaction_db1",
		Parameters: map[string]interface{}{"action": "list_snapshots"},
	}, "db1", uc)
	require.NoError(t, err)

	text := snapshotResponseText(t, resp)
	assert.Contains(t, text, "snap_1")
	assert.Contains(t, text, "delete")
	assert.Contains(t, text, "users")
	assert.Zero(t, uc.txCalls, "a snapshot action must not open a transaction")
}

// TestTransactionTool_RollbackSnapshotRoutesID is the property that matters:
// the action must restore the snapshot the caller named.
func TestTransactionTool_RollbackSnapshotRoutesID(t *testing.T) {
	uc := &stubSnapshotUseCase{}
	tool := &TransactionTool{BaseToolType: BaseToolType{name: "transaction"}}

	resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name: "transaction_db1",
		Parameters: map[string]interface{}{
			"action":      "rollback_snapshot",
			"snapshot_id": "snap_7",
		},
	}, "db1", uc)
	require.NoError(t, err)

	assert.Equal(t, "snap_7", uc.rolledBack)
	assert.Contains(t, snapshotResponseText(t, resp), "snap_7")
	assert.Zero(t, uc.txCalls)
}

// TestTransactionTool_RollbackWithoutIDErrors: a rollback with no snapshot id
// must fail loudly rather than restore an arbitrary snapshot.
func TestTransactionTool_RollbackWithoutIDErrors(t *testing.T) {
	uc := &stubSnapshotUseCase{}
	tool := &TransactionTool{BaseToolType: BaseToolType{name: "transaction"}}

	_, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name:       "transaction_db1",
		Parameters: map[string]interface{}{"action": "rollback_snapshot"},
	}, "db1", uc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "snapshot_id parameter is required")
	assert.Empty(t, uc.rolledBack)
}

// TestTransactionTool_SnapshotUnsupportedProvider: a provider without the
// capability says so instead of silently doing nothing.
func TestTransactionTool_SnapshotUnsupportedProvider(t *testing.T) {
	uc := &plainTransactionUseCase{}
	tool := &TransactionTool{BaseToolType: BaseToolType{name: "transaction"}}

	_, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name:       "transaction_db1",
		Parameters: map[string]interface{}{"action": "list_snapshots"},
	}, "db1", uc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported by this provider")
}

// TestTransactionTool_NormalActionStillWorks pins that the snapshot branch did
// not swallow the ordinary transaction path.
func TestTransactionTool_NormalActionStillWorks(t *testing.T) {
	uc := &stubSnapshotUseCase{}
	tool := &TransactionTool{BaseToolType: BaseToolType{name: "transaction"}}

	resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name:       "transaction_db1",
		Parameters: map[string]interface{}{"action": "begin"},
	}, "db1", uc)
	require.NoError(t, err)
	assert.Equal(t, 1, uc.txCalls)
	assert.Contains(t, snapshotResponseText(t, resp), "transaction handled")
}

type plainTransactionUseCase struct {
	snapshotBaseProvider
}

// snapshotResponseText extracts the text payload from the delivery response.
func snapshotResponseText(t *testing.T, resp interface{}) string {
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
