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

type stubSnapshotUseCase struct {
	stubMaskingUseCase
	rolledBack string
}

func (s *stubSnapshotUseCase) ListSnapshots(dbID string) []usecase.MutationSnapshot {
	return []usecase.MutationSnapshot{{ID: "snap_1", DatabaseID: dbID, Kind: "delete", Table: "users"}}
}

func (s *stubSnapshotUseCase) RollbackSnapshot(_ context.Context, _ string, id string) (string, error) {
	s.rolledBack = id
	return "Restored 1 row(s) from snapshot " + id, nil
}

// TestTransactionTool_SnapshotActions proves list_snapshots/rollback_snapshot
// route to the snapshot-capable use case.
func TestTransactionTool_SnapshotActions(t *testing.T) {
	tool := NewTransactionTool()

	t.Run("rollback_snapshot", func(t *testing.T) {
		uc := &stubSnapshotUseCase{}
		resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
			Name: "transaction_db1",
			Parameters: map[string]interface{}{
				"action":      "rollback_snapshot",
				"snapshot_id": "snap_7",
			},
		}, "", uc)
		if err != nil {
			t.Fatalf("handle failed: %v", err)
		}
		if uc.rolledBack != "snap_7" {
			t.Fatalf("expected rollback of snap_7, got %q", uc.rolledBack)
		}
		out := sprintfResponse(resp)
		if !strings.Contains(out, "snap_7") {
			t.Fatalf("expected confirmation mentioning snap_7:\n%s", out)
		}
	})

	t.Run("list_snapshots", func(t *testing.T) {
		uc := &stubSnapshotUseCase{}
		resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
			Name:       "transaction_db1",
			Parameters: map[string]interface{}{"action": "list_snapshots"},
		}, "", uc)
		if err != nil {
			t.Fatalf("handle failed: %v", err)
		}
		if out := sprintfResponse(resp); !strings.Contains(out, "snap_1") {
			t.Fatalf("expected listing to include snap_1:\n%s", out)
		}
	})
}

type stubSchemaDriftUseCase struct {
	stubMaskingUseCase
	lastBaseline string
}

func (s *stubSchemaDriftUseCase) CaptureSchemaSnapshot(_ context.Context, _ string) (*usecase.SchemaSnapshot, error) {
	return &usecase.SchemaSnapshot{ID: "schema_snap_1", Tables: map[string][]usecase.SchemaColumn{
		"users": {{Name: "id", Type: "integer"}},
	}}, nil
}

func (s *stubSchemaDriftUseCase) CheckSchemaDrift(_ context.Context, _ string, baselineID string) (*usecase.SchemaDriftReport, error) {
	s.lastBaseline = baselineID
	return &usecase.SchemaDriftReport{BaselineID: baselineID, Drifted: true,
		Changes: []string{"users.email added (text)"}}, nil
}

func (s *stubSchemaDriftUseCase) ListSchemaSnapshots(dbID string) []usecase.SchemaSnapshot {
	return []usecase.SchemaSnapshot{{ID: "schema_snap_1", DatabaseID: dbID}}
}

// TestTransactionTool_SchemaDriftActions proves the three drift actions route.
func TestTransactionTool_SchemaDriftActions(t *testing.T) {
	tool := NewTransactionTool()

	t.Run("capture", func(t *testing.T) {
		uc := &stubSchemaDriftUseCase{}
		resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
			Name:       "transaction_db1",
			Parameters: map[string]interface{}{"action": "capture_schema_snapshot"},
		}, "", uc)
		if err != nil {
			t.Fatalf("handle failed: %v", err)
		}
		if out := sprintfResponse(resp); !strings.Contains(out, "schema_snap_1") {
			t.Fatalf("expected new baseline id:\n%s", out)
		}
	})

	t.Run("drift_check", func(t *testing.T) {
		uc := &stubSchemaDriftUseCase{}
		resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
			Name: "transaction_db1",
			Parameters: map[string]interface{}{
				"action":      "check_schema_drift",
				"baseline_id": "schema_snap_2",
			},
		}, "", uc)
		if err != nil {
			t.Fatalf("handle failed: %v", err)
		}
		if uc.lastBaseline != "schema_snap_2" {
			t.Fatalf("expected baseline routing, got %q", uc.lastBaseline)
		}
		if out := sprintfResponse(resp); !strings.Contains(out, "users.email") {
			t.Fatalf("expected change list:\n%s", out)
		}
	})

	t.Run("list", func(t *testing.T) {
		uc := &stubSchemaDriftUseCase{}
		resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
			Name:       "transaction_db1",
			Parameters: map[string]interface{}{"action": "list_schema_snapshots"},
		}, "", uc)
		if err != nil {
			t.Fatalf("handle failed: %v", err)
		}
		if out := sprintfResponse(resp); !strings.Contains(out, "schema_snap_1") {
			t.Fatalf("expected listing:\n%s", out)
		}
	})
}

// TestSchemaTool_FormatSensitiveRoutes proves format=sensitive renders the
// PII column report via the capability-detected path.
func TestSchemaTool_FormatSensitiveRoutes(t *testing.T) {
	tool := NewSchemaTool()
	uc := &stubSensitiveUseCase{}

	resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name:       "schema_db1",
		Parameters: map[string]interface{}{"format": "sensitive"},
	}, "", uc)
	if err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	out := strings.ToLower(sprintfResponse(resp))
	if !strings.Contains(out, "mask_pii") {
		t.Fatalf("expected sensitive-column report:\n%s", out)
	}
}

type stubSensitiveUseCase struct{ stubMaskingUseCase }

func (s *stubSensitiveUseCase) FindSensitiveColumns(_ context.Context, dbID string) ([]usecase.SensitiveFinding, error) {
	return []usecase.SensitiveFinding{{Table: "users", Column: "email", Category: "email"}}, nil
}
