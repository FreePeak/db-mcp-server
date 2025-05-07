package mcp

import (
	"context"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateRetentionPolicyTool(t *testing.T) {
	tool := NewTimescaleDBTool()
	retentionTool := tool.CreateRetentionPolicyTool("retention_tool", "test_db")

	assert.NotNil(t, retentionTool, "Retention policy tool should be created")

	// Verify tool has the required parameters
	toolMap, ok := retentionTool.(map[string]interface{})
	assert.True(t, ok, "Tool should be a map")

	params, ok := toolMap["parameters"].(map[string]interface{})
	assert.True(t, ok, "Tool should have parameters")

	assert.Contains(t, params, "operation", "Tool should have operation parameter")
	assert.Contains(t, params, "target_table", "Tool should have target_table parameter")
	assert.Contains(t, params, "retention_interval", "Tool should have retention_interval parameter")
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
