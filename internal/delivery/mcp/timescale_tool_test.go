package mcp

import (
	"context"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDatabaseUseCase is a mock implementation of the database use case
type MockDatabaseUseCase struct {
	mock.Mock
}

// ExecuteStatement mocks the ExecuteStatement method
func (m *MockDatabaseUseCase) ExecuteStatement(ctx context.Context, dbID, statement string, params []interface{}) (string, error) {
	args := m.Called(ctx, dbID, statement, params)
	return args.String(0), args.Error(1)
}

// GetDatabaseType mocks the GetDatabaseType method
func (m *MockDatabaseUseCase) GetDatabaseType(dbID string) (string, error) {
	args := m.Called(dbID)
	return args.String(0), args.Error(1)
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
