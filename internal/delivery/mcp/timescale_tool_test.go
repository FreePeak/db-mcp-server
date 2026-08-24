package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTimescaleDBTool_CreateTool(t *testing.T) {
	tool := NewTimescaleDBTool()
	assert.Equal(t, "timescaledb", tool.GetName())
	assert.Contains(t, tool.GetDescription("test_db"), "on test_db")

	// Test standard tool creation
	baseTool := tool.CreateTool("test_tool", "test_db")
	assert.NotNil(t, baseTool)
}

func TestTimescaleDBTool_CreateHypertableTool(t *testing.T) {
	tool := NewTimescaleDBTool()
	hypertableTool := tool.CreateHypertableTool("hypertable_tool", "test_db")
	assert.NotNil(t, hypertableTool)
}

func TestTimescaleDBTool_CreateListHypertablesTool(t *testing.T) {
	tool := NewTimescaleDBTool()
	listTool := tool.CreateListHypertablesTool("list_tool", "test_db")
	assert.NotNil(t, listTool)
}

func TestTimescaleDBTool_CreateRetentionPolicyTool(t *testing.T) {
	tool := NewTimescaleDBTool()
	retentionTool := tool.CreateRetentionPolicyTool("retention_tool", "test_db")

	assert.NotNil(t, retentionTool, "Retention policy tool should be created")
}

func TestHandleCreateHypertable(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.MatchedBy(func(_ string) bool {
		return true // Accept any SQL for now
	}), mock.Anything).Return(`{"result": "success"}`, nil)

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":    "create_hypertable",
			"target_table": "metrics",
			"time_column":  "timestamp",
		},
	}

	// Call the handler
	result, err := tool.HandleRequest(context.Background(), request, "test_db", mockUseCase)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify mock expectations
	mockUseCase.AssertExpectations(t)
}

func TestHandleListHypertables(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)
	// The runtime extension guard runs before the catalog read.
	mockUseCase.On("ExecuteQuery", mock.Anything, "test_db", mock.MatchedBy(func(q string) bool {
		return strings.Contains(q, "pg_extension")
	}), mock.Anything).Return(`[{"n": 1}]`, nil)
	// Listing is a pure SELECT and must go through ExecuteQuery so it stays
	// available on read_only databases (ExecuteStatement is blocked there).
	mockUseCase.On("ExecuteQuery", mock.Anything, "test_db", mock.MatchedBy(func(q string) bool {
		return !strings.Contains(q, "pg_extension")
	}), mock.Anything).Return(`[{"table_name":"metrics","schema_name":"public","time_column":"time"}]`, nil)

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation": "list_hypertables",
		},
	}

	// Call the handler
	result, err := tool.handleListHypertables(context.Background(), request, "test_db", mockUseCase)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check the result
	resultMap, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, resultMap, "message")
	assert.Contains(t, resultMap, "details")

	// Verify mock expectations
	mockUseCase.AssertExpectations(t)
}

// TestEnsureTimescaleExtension_Absent locks in lazy-loading's call-time
// guard: when the extension probe returns zero rows the caller gets
// actionable guidance instead of a raw catalog error.
func TestEnsureTimescaleExtension_Absent(t *testing.T) {
	mockUseCase := new(MockDatabaseUseCase)
	mockUseCase.On("ExecuteQuery", mock.Anything, "plain_pg", mock.AnythingOfType("string"), mock.Anything).
		Return(`[{"n":0}]`, nil)

	err := ensureTimescaleExtension(context.Background(), "plain_pg", mockUseCase)
	if err == nil {
		t.Fatal("expected error for database without timescaledb")
	}
	if !strings.Contains(err.Error(), "CREATE EXTENSION timescaledb") {
		t.Errorf("expected actionable guidance, got: %v", err)
	}
}

// TestEnsureTimescaleExtension_QueryError covers the fail-closed path:
// a failed probe must abort the operation rather than assume absence.
func TestEnsureTimescaleExtension_QueryError(t *testing.T) {
	mockUseCase := new(MockDatabaseUseCase)
	mockUseCase.On("ExecuteQuery", mock.Anything, "broken_pg", mock.AnythingOfType("string"), mock.Anything).
		Return("", context.DeadlineExceeded)

	err := ensureTimescaleExtension(context.Background(), "broken_pg", mockUseCase)
	if err == nil || !strings.Contains(err.Error(), "cannot verify") {
		t.Fatalf("expected verification failure, got: %v", err)
	}
}

func TestHandleListHypertablesNonPostgresDB(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations for a non-PostgreSQL database
	mockUseCase.On("GetDatabaseType", "test_db").Return("mysql", nil)

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation": "list_hypertables",
		},
	}

	// Call the handler
	_, err := tool.handleListHypertables(context.Background(), request, "test_db", mockUseCase)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TimescaleDB operations are only supported on PostgreSQL databases")

	// Verify mock expectations
	mockUseCase.AssertExpectations(t)
}

func TestHandleAddRetentionPolicy(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.MatchedBy(func(_ string) bool {
		return true // Accept any SQL for now
	}), mock.Anything).Return(`{"result": "success"}`, nil)

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":          "add_retention_policy",
			"target_table":       "metrics",
			"retention_interval": "30 days",
		},
	}

	// Call the handler
	result, err := tool.HandleRequest(context.Background(), request, "test_db", mockUseCase)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify mock expectations
	mockUseCase.AssertExpectations(t)
}

func TestHandleRemoveRetentionPolicy(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.MatchedBy(func(_ string) bool {
		return true // Accept any SQL for now
	}), mock.Anything).Return(`{"result": "success"}`, nil)

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":    "remove_retention_policy",
			"target_table": "metrics",
		},
	}

	// Call the handler
	result, err := tool.HandleRequest(context.Background(), request, "test_db", mockUseCase)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify mock expectations
	mockUseCase.AssertExpectations(t)
}

func TestHandleGetRetentionPolicy(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)
	// Extension probe from ensureTimescaleExtension
	mockUseCase.On("ExecuteQuery", mock.Anything, "test_db", mock.MatchedBy(func(sql string) bool {
		return strings.Contains(sql, "pg_extension")
	}), mock.Anything).Return("Results:\nn\n 1\n", nil).Once()
	// Get retention policy (read_only-safe ExecuteQuery path)
	mockUseCase.On("ExecuteQuery", mock.Anything, "test_db", mock.MatchedBy(func(sql string) bool {
		return strings.Contains(sql, "policy_retention")
	}), mock.Anything).Return(`Results:
hypertable_name	retention_interval	retention_enabled
metrics	30 days	true

Total rows: 1`, nil).Once()

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":    "get_retention_policy",
			"target_table": "metrics",
		},
	}

	// Call the handler
	result, err := tool.HandleRequest(context.Background(), request, "test_db", mockUseCase)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify mock expectations
	mockUseCase.AssertExpectations(t)
}

func TestHandleNonPostgresDB(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations for a non-PostgreSQL database
	mockUseCase.On("GetDatabaseType", "test_db").Return("mysql", nil)

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":          "add_retention_policy",
			"target_table":       "metrics",
			"retention_interval": "30 days",
		},
	}

	// Call the handler
	_, err := tool.HandleRequest(context.Background(), request, "test_db", mockUseCase)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TimescaleDB operations are only supported on PostgreSQL databases")

	// Verify mock expectations
	mockUseCase.AssertExpectations(t)
}
