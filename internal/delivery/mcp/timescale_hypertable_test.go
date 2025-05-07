package mcp_test

import (
	"context"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/FreePeak/db-mcp-server/internal/delivery/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockUseCase is a mock implementation for testing
type MockUseCase struct {
	mock.Mock
}

func (m *MockUseCase) ExecuteQuery(ctx context.Context, dbID, query string, params []interface{}) (string, error) {
	args := m.Called(ctx, dbID, query, params)
	return args.String(0), args.Error(1)
}

func (m *MockUseCase) ExecuteStatement(ctx context.Context, dbID, statement string, params []interface{}) (string, error) {
	args := m.Called(ctx, dbID, statement, params)
	return args.String(0), args.Error(1)
}

func (m *MockUseCase) ExecuteTransaction(ctx context.Context, dbID, action string, txID string, statement string, params []interface{}, readOnly bool) (string, map[string]interface{}, error) {
	args := m.Called(ctx, dbID, action, txID, statement, params, readOnly)
	return args.String(0), args.Get(1).(map[string]interface{}), args.Error(2)
}

func (m *MockUseCase) GetDatabaseInfo(dbID string) (map[string]interface{}, error) {
	args := m.Called(dbID)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockUseCase) ListDatabases() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockUseCase) GetDatabaseType(dbID string) (string, error) {
	args := m.Called(dbID)
	return args.String(0), args.Error(1)
}

func TestCreateHypertableTool(t *testing.T) {
	// Create a new TimescaleDB tool
	timescaleTool := mcp.NewTimescaleDBTool()
	assert.NotNil(t, timescaleTool)

	// Test creating a hypertable creation tool
	tool := timescaleTool.CreateHypertableTool("create_hypertable_test_db", "test_db")
	assert.NotNil(t, tool, "Should create a hypertable tool")

	// Create mock request with parameters
	mockRequest := server.ToolCallRequest{
		Name: "create_hypertable_test_db",
		Parameters: map[string]interface{}{
			"operation":           "create_hypertable",
			"target_table":        "metrics",
			"time_column":         "timestamp",
			"chunk_time_interval": "1 day",
		},
	}

	// Create mock use case
	mockUseCase := new(MockUseCase)
	mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)
	mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.MatchedBy(func(statement string) bool {
		return assert.Contains(t, statement, "SELECT create_hypertable")
	}), mock.Anything).Return(`{"message":"Hypertable created successfully"}`, nil)

	// Test handling a create_hypertable request
	ctx := context.Background()
	response, err := timescaleTool.HandleRequest(ctx, mockRequest, "test_db", mockUseCase)

	// Verify the response
	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Check the mock was called correctly
	mockUseCase.AssertExpectations(t)
}
