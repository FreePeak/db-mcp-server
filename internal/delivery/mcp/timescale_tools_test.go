package mcp_test

import (
	"context"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/FreePeak/db-mcp-server/internal/delivery/mcp"
)

// MockDatabaseUseCase is a mock implementation of the UseCaseProvider interface
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

// ExecuteQuery mocks the ExecuteQuery method
func (m *MockDatabaseUseCase) ExecuteQuery(ctx context.Context, dbID, query string, params []interface{}) (string, error) {
	args := m.Called(ctx, dbID, query, params)
	return args.String(0), args.Error(1)
}

// ExecuteTransaction mocks the ExecuteTransaction method
func (m *MockDatabaseUseCase) ExecuteTransaction(ctx context.Context, dbID, action string, txID string, statement string, params []interface{}, readOnly bool) (string, map[string]interface{}, error) {
	args := m.Called(ctx, dbID, action, txID, statement, params, readOnly)
	return args.String(0), args.Get(1).(map[string]interface{}), args.Error(2)
}

// GetDatabaseInfo mocks the GetDatabaseInfo method
func (m *MockDatabaseUseCase) GetDatabaseInfo(dbID string) (map[string]interface{}, error) {
	args := m.Called(dbID)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

// ListDatabases mocks the ListDatabases method
func (m *MockDatabaseUseCase) ListDatabases() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func TestTimescaleDBTool(t *testing.T) {
	tool := mcp.NewTimescaleDBTool()
	assert.Equal(t, "timescaledb", tool.GetName())
}

func TestTimeSeriesQueryTool(t *testing.T) {
	// Create a mock use case provider
	mockUseCase := new(MockDatabaseUseCase)

	// Set up the TimescaleDB tool
	tool := mcp.NewTimescaleDBTool()

	// Create a context for testing
	ctx := context.Background()

	// Test case for time_series_query operation
	t.Run("time_series_query with basic parameters", func(t *testing.T) {
		// Sample result that would be returned by the database
		sampleResult := `[
			{"time_bucket": "2023-01-01T00:00:00Z", "avg_temp": 22.5, "count": 10},
			{"time_bucket": "2023-01-02T00:00:00Z", "avg_temp": 23.1, "count": 12}
		]`

		// Set up expectations for the mock
		mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.AnythingOfType("string"), mock.Anything).
			Return(sampleResult, nil).Once()

		// Create a request with time_series_query operation
		request := server.ToolCallRequest{
			Name: "timescaledb_timeseries_query_test_db",
			Parameters: map[string]interface{}{
				"operation":       "time_series_query",
				"target_table":    "sensor_data",
				"time_column":     "timestamp",
				"bucket_interval": "1 day",
				"start_time":      "2023-01-01",
				"end_time":        "2023-01-31",
				"aggregations":    "AVG(temperature) as avg_temp, COUNT(*) as count",
			},
		}

		// Call the handler
		result, err := tool.HandleRequest(ctx, request, "test_db", mockUseCase)

		// Verify the result
		assert.NoError(t, err)
		assert.NotNil(t, result)

		// Check the result contains expected fields
		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, resultMap, "message")
		assert.Contains(t, resultMap, "details")
		assert.Equal(t, sampleResult, resultMap["details"])

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	t.Run("time_series_query with window functions", func(t *testing.T) {
		// Sample result that would be returned by the database
		sampleResult := `[
			{"time_bucket": "2023-01-01T00:00:00Z", "avg_temp": 22.5, "prev_avg": null},
			{"time_bucket": "2023-01-02T00:00:00Z", "avg_temp": 23.1, "prev_avg": 22.5}
		]`

		// Set up expectations for the mock
		mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.AnythingOfType("string"), mock.Anything).
			Return(sampleResult, nil).Once()

		// Create a request with time_series_query operation
		request := server.ToolCallRequest{
			Name: "timescaledb_timeseries_query_test_db",
			Parameters: map[string]interface{}{
				"operation":        "time_series_query",
				"target_table":     "sensor_data",
				"time_column":      "timestamp",
				"bucket_interval":  "1 day",
				"aggregations":     "AVG(temperature) as avg_temp",
				"window_functions": "LAG(avg_temp) OVER (ORDER BY time_bucket) AS prev_avg",
				"format_pretty":    true,
			},
		}

		// Call the handler
		result, err := tool.HandleRequest(ctx, request, "test_db", mockUseCase)

		// Verify the result
		assert.NoError(t, err)
		assert.NotNil(t, result)

		// Check the result contains expected fields
		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, resultMap, "message")
		assert.Contains(t, resultMap, "details")
		assert.Contains(t, resultMap, "metadata")

		// Check metadata contains expected fields for pretty formatting
		metadata, ok := resultMap["metadata"].(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, metadata, "num_rows")
		assert.Contains(t, metadata, "time_bucket_interval")

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})
}
