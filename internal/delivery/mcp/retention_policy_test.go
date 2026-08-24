package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleAddRetentionPolicyFull(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`{"message":"Retention policy added"}`, nil)

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":          "add_retention_policy",
			"target_table":       "test_table",
			"retention_interval": "30 days",
		},
	}

	// Call the handler
	result, err := tool.HandleRequest(context.Background(), request, "test_db", mockUseCase)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check the result
	resultMap, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, resultMap, "message")

	// Verify mock expectations
	mockUseCase.AssertExpectations(t)
}

func TestHandleRemoveRetentionPolicyFull(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)

	// First should try to find any policy
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`[{"job_id": 123}]`, nil).Once()

	// Then remove the policy
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`{"message":"Policy removed"}`, nil).Once()

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":    "remove_retention_policy",
			"target_table": "test_table",
		},
	}

	// Call the handler
	result, err := tool.HandleRequest(context.Background(), request, "test_db", mockUseCase)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check the result
	resultMap, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, resultMap, "message")

	// Verify mock expectations
	mockUseCase.AssertExpectations(t)
}

func TestHandleRemoveRetentionPolicyNoPolicy(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)

	// Find policy ID - no policy found
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`[]`, nil).Once()

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":    "remove_retention_policy",
			"target_table": "test_table",
		},
	}

	// Call the handler
	result, err := tool.HandleRequest(context.Background(), request, "test_db", mockUseCase)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check the result
	resultMap, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, resultMap, "message")
	assert.Contains(t, resultMap["message"].(string), "No retention policy found")

	// Verify mock expectations
	mockUseCase.AssertExpectations(t)
}

func TestHandleGetRetentionPolicyFull(t *testing.T) {
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
test_table	30 days	true

Total rows: 1`, nil).Once()

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":    "get_retention_policy",
			"target_table": "test_table",
		},
	}

	// Call the handler
	result, err := tool.HandleRequest(context.Background(), request, "test_db", mockUseCase)

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

func TestHandleGetRetentionPolicyNoPolicy(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)

	// Extension probe from ensureTimescaleExtension
	mockUseCase.On("ExecuteQuery", mock.Anything, "test_db", mock.MatchedBy(func(sql string) bool {
		return strings.Contains(sql, "pg_extension")
	}), mock.Anything).Return("Results:\nn\n 1\n", nil).Once()

	// No retention policy found (read_only-safe ExecuteQuery path)
	mockUseCase.On("ExecuteQuery", mock.Anything, "test_db", mock.MatchedBy(func(sql string) bool {
		return strings.Contains(sql, "policy_retention")
	}), mock.Anything).Return(`Results:
hypertable_name	retention_interval	retention_enabled

Total rows: 0`, nil).Once()

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":    "get_retention_policy",
			"target_table": "test_table",
		},
	}

	// Call the handler
	result, err := tool.HandleRequest(context.Background(), request, "test_db", mockUseCase)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check the result
	resultMap, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, resultMap, "message")
	assert.Contains(t, resultMap["message"].(string), "empty result means none is configured")

	// Verify mock expectations
	mockUseCase.AssertExpectations(t)
}
