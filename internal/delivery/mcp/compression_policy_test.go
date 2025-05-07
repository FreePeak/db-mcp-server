package mcp

import (
	"context"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleEnableCompression(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`{"message":"Compression enabled"}`, nil)

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":    "enable_compression",
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

func TestHandleEnableCompressionWithInterval(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`{"message":"Compression enabled"}`, nil).Twice()

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":    "enable_compression",
			"target_table": "test_table",
			"after":        "7 days",
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

func TestHandleDisableCompression(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)

	// First should try to remove any policy
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`[{"job_id": 123}]`, nil).Once()

	// Then remove the policy and disable compression
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`{"message":"Policy removed"}`, nil).Once()
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`{"message":"Compression disabled"}`, nil).Once()

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":    "disable_compression",
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

func TestHandleAddCompressionPolicy(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)

	// Check compression status
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`[{"compress": true}]`, nil).Once()

	// Add compression policy
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`{"message":"Compression policy added"}`, nil).Once()

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":    "add_compression_policy",
			"target_table": "test_table",
			"interval":     "30 days",
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

func TestHandleAddCompressionPolicyWithOptions(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)

	// Check compression status
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`[{"compress": true}]`, nil).Once()

	// Add compression policy with options
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`{"message":"Compression policy added"}`, nil).Once()

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":    "add_compression_policy",
			"target_table": "test_table",
			"interval":     "30 days",
			"segment_by":   "device_id",
			"order_by":     "time DESC",
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

func TestHandleRemoveCompressionPolicy(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)

	// Find policy ID
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`[{"job_id": 123}]`, nil).Once()

	// Remove policy
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`{"message":"Policy removed"}`, nil).Once()

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":    "remove_compression_policy",
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

func TestHandleGetCompressionSettings(t *testing.T) {
	// Create a mock use case
	mockUseCase := new(MockDatabaseUseCase)

	// Set up expectations
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)

	// Check compression enabled
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`[{"compress": true}]`, nil).Once()

	// Get compression settings
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`[{"segmentby": "device_id", "orderby": "time DESC"}]`, nil).Once()

	// Get policy info
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.Anything, mock.Anything).Return(`[{"schedule_interval": "30 days", "chunk_time_interval": "1 day"}]`, nil).Once()

	// Create the tool
	tool := NewTimescaleDBTool()

	// Create a request
	request := server.ToolCallRequest{
		Parameters: map[string]interface{}{
			"operation":    "get_compression_settings",
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
	assert.Contains(t, resultMap, "settings")

	// Verify mock expectations
	mockUseCase.AssertExpectations(t)
}
