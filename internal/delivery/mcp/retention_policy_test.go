package mcp

import (
	"context"
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

	// Get retention policy
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`[{"hypertable_name":"test_table","retention_interval":"30 days","retention_enabled":true}]`, nil).Once()

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

	// No retention policy found
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`[]`, nil).Once()

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
	assert.Contains(t, resultMap["message"].(string), "No retention policy found")

	// Verify mock expectations
	mockUseCase.AssertExpectations(t)
}
