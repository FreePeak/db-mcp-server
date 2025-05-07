package mcp

import (
	"context"
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
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.MatchedBy(func(sql string) bool {
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
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.MatchedBy(func(sql string) bool {
		return true // Any SQL that contains the relevant query
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
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.MatchedBy(func(sql string) bool {
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
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.MatchedBy(func(sql string) bool {
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
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.MatchedBy(func(sql string) bool {
		return true // Accept any SQL for now
	}), mock.Anything).Return(`[{"hypertable_name":"metrics","retention_interval":"30 days","retention_enabled":true}]`, nil)

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
