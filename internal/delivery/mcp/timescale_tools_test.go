package mcp_test

import (
	"context"
	"strings"
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

// TestContinuousAggregateTool tests the continuous aggregate operations
func TestContinuousAggregateTool(t *testing.T) {
	// Create a context for testing
	ctx := context.Background()

	// Test case for create_continuous_aggregate operation
	t.Run("create_continuous_aggregate", func(t *testing.T) {
		// Create a new mock for this test case
		mockUseCase := new(MockDatabaseUseCase)

		// Set up the TimescaleDB tool
		tool := mcp.NewTimescaleDBTool()

		// Set up expectations
		// Removed GetDatabaseType expectation as it's not called in this handler

		// Add mock expectation for the SQL containing CREATE MATERIALIZED VIEW
		mockUseCase.On("ExecuteStatement",
			mock.Anything,
			"test_db",
			mock.MatchedBy(func(sql string) bool {
				return strings.Contains(sql, "CREATE MATERIALIZED VIEW")
			}),
			mock.Anything).Return(`{"result": "success"}`, nil)

		// Add separate mock expectation for policy SQL if needed
		mockUseCase.On("ExecuteStatement",
			mock.Anything,
			"test_db",
			mock.MatchedBy(func(sql string) bool {
				return strings.Contains(sql, "add_continuous_aggregate_policy")
			}),
			mock.Anything).Return(`{"result": "success"}`, nil)

		// Create a request
		request := server.ToolCallRequest{
			Name: "timescaledb_create_continuous_aggregate_test_db",
			Parameters: map[string]interface{}{
				"operation":        "create_continuous_aggregate",
				"view_name":        "daily_metrics",
				"source_table":     "sensor_data",
				"time_column":      "timestamp",
				"bucket_interval":  "1 day",
				"aggregations":     "AVG(temperature) as avg_temp, MIN(temperature) as min_temp, MAX(temperature) as max_temp",
				"with_data":        true,
				"refresh_policy":   true,
				"refresh_interval": "1 hour",
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
		assert.Contains(t, resultMap, "sql")

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	// Test case for refresh_continuous_aggregate operation
	t.Run("refresh_continuous_aggregate", func(t *testing.T) {
		// Create a new mock for this test case
		mockUseCase := new(MockDatabaseUseCase)

		// Set up the TimescaleDB tool
		tool := mcp.NewTimescaleDBTool()

		// Set up expectations
		// Removed GetDatabaseType expectation as it's not called in this handler
		mockUseCase.On("ExecuteStatement",
			mock.Anything,
			"test_db",
			mock.MatchedBy(func(sql string) bool {
				return strings.Contains(sql, "CALL refresh_continuous_aggregate")
			}),
			mock.Anything).Return(`{"result": "success"}`, nil)

		// Create a request
		request := server.ToolCallRequest{
			Name: "timescaledb_refresh_continuous_aggregate_test_db",
			Parameters: map[string]interface{}{
				"operation":  "refresh_continuous_aggregate",
				"view_name":  "daily_metrics",
				"start_time": "2023-01-01",
				"end_time":   "2023-01-31",
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

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	// Test case for drop_continuous_aggregate operation
	t.Run("drop_continuous_aggregate", func(t *testing.T) {
		// Create a new mock for this test case
		mockUseCase := new(MockDatabaseUseCase)

		// Set up the TimescaleDB tool
		tool := mcp.NewTimescaleDBTool()

		// Set up expectations
		// Removed GetDatabaseType expectation as it's not called in this handler
		mockUseCase.On("ExecuteStatement",
			mock.Anything,
			"test_db",
			mock.MatchedBy(func(sql string) bool {
				return strings.Contains(sql, "DROP MATERIALIZED VIEW")
			}),
			mock.Anything).Return(`{"result": "success"}`, nil)

		// Create a request
		request := server.ToolCallRequest{
			Name: "timescaledb_drop_continuous_aggregate_test_db",
			Parameters: map[string]interface{}{
				"operation": "drop_continuous_aggregate",
				"view_name": "daily_metrics",
				"cascade":   true,
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

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	// Test case for list_continuous_aggregates operation
	t.Run("list_continuous_aggregates", func(t *testing.T) {
		// Create a new mock for this test case
		mockUseCase := new(MockDatabaseUseCase)

		// Set up the TimescaleDB tool
		tool := mcp.NewTimescaleDBTool()

		// Set up expectations
		mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)
		mockUseCase.On("ExecuteStatement",
			mock.Anything,
			"test_db",
			mock.MatchedBy(func(sql string) bool {
				return strings.Contains(sql, "SELECT") && strings.Contains(sql, "continuous_aggregates")
			}),
			mock.Anything).Return(`[{"view_name": "daily_metrics", "source_table": "sensor_data"}]`, nil)

		// Create a request
		request := server.ToolCallRequest{
			Name: "timescaledb_list_continuous_aggregates_test_db",
			Parameters: map[string]interface{}{
				"operation": "list_continuous_aggregates",
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

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	// Test case for get_continuous_aggregate_info operation
	t.Run("get_continuous_aggregate_info", func(t *testing.T) {
		// Create a new mock for this test case
		mockUseCase := new(MockDatabaseUseCase)

		// Set up the TimescaleDB tool
		tool := mcp.NewTimescaleDBTool()

		// Set up expectations
		mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)
		mockUseCase.On("ExecuteStatement",
			mock.Anything,
			"test_db",
			mock.MatchedBy(func(sql string) bool {
				return strings.Contains(sql, "SELECT") && strings.Contains(sql, "continuous_aggregates") && strings.Contains(sql, "WHERE")
			}),
			mock.Anything).Return(`[{"view_name": "daily_metrics", "source_table": "sensor_data", "bucket_interval": "1 day"}]`, nil)

		// Create a request
		request := server.ToolCallRequest{
			Name: "timescaledb_get_continuous_aggregate_info_test_db",
			Parameters: map[string]interface{}{
				"operation": "get_continuous_aggregate_info",
				"view_name": "daily_metrics",
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

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	// Test case for add_continuous_aggregate_policy operation
	t.Run("add_continuous_aggregate_policy", func(t *testing.T) {
		// Create a new mock for this test case
		mockUseCase := new(MockDatabaseUseCase)

		// Set up the TimescaleDB tool
		tool := mcp.NewTimescaleDBTool()

		// Set up expectations
		mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)
		mockUseCase.On("ExecuteStatement",
			mock.Anything,
			"test_db",
			mock.MatchedBy(func(sql string) bool {
				return strings.Contains(sql, "add_continuous_aggregate_policy")
			}),
			mock.Anything).Return(`{"result": "success"}`, nil)

		// Create a request
		request := server.ToolCallRequest{
			Name: "timescaledb_add_continuous_aggregate_policy_test_db",
			Parameters: map[string]interface{}{
				"operation":         "add_continuous_aggregate_policy",
				"view_name":         "daily_metrics",
				"start_offset":      "1 month",
				"end_offset":        "2 hours",
				"schedule_interval": "6 hours",
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

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	// Test case for remove_continuous_aggregate_policy operation
	t.Run("remove_continuous_aggregate_policy", func(t *testing.T) {
		// Create a new mock for this test case
		mockUseCase := new(MockDatabaseUseCase)

		// Set up the TimescaleDB tool
		tool := mcp.NewTimescaleDBTool()

		// Set up expectations
		mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil)
		mockUseCase.On("ExecuteStatement",
			mock.Anything,
			"test_db",
			mock.MatchedBy(func(sql string) bool {
				return strings.Contains(sql, "remove_continuous_aggregate_policy")
			}),
			mock.Anything).Return(`{"result": "success"}`, nil)

		// Create a request
		request := server.ToolCallRequest{
			Name: "timescaledb_remove_continuous_aggregate_policy_test_db",
			Parameters: map[string]interface{}{
				"operation": "remove_continuous_aggregate_policy",
				"view_name": "daily_metrics",
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

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})
}
