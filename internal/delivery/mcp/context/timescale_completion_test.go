package context_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/FreePeak/db-mcp-server/internal/delivery/mcp"
)

func TestTimescaleDBCompletionProvider(t *testing.T) {
	// Create a mock use case provider
	mockUseCase := new(MockDatabaseUseCase)

	// Create a context for testing
	ctx := context.Background()

	t.Run("get_time_bucket_completions", func(t *testing.T) {
		// Set up expectations for the mock
		mockUseCase.On("GetDatabaseType", "timescale_db").Return("postgres", nil).Once()
		mockUseCase.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return sql == "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(`[{"extversion": "2.8.0"}]`, nil).Once()

		// Create the completion provider
		provider := mcp.NewTimescaleDBCompletionProvider()

		// Call the method to get time bucket function completions
		completions, err := provider.GetTimeBucketCompletions(ctx, "timescale_db", mockUseCase)

		// Verify the result
		assert.NoError(t, err)
		assert.NotNil(t, completions)
		assert.NotEmpty(t, completions)

		// Check for essential time_bucket functions
		var foundBasicTimeBucket, foundGapfill, foundTzTimeBucket bool
		for _, completion := range completions {
			if completion.Name == "time_bucket" && completion.Type == "function" {
				foundBasicTimeBucket = true
				assert.Contains(t, completion.Documentation, "buckets")
				assert.Contains(t, completion.InsertText, "time_bucket")
			}
			if completion.Name == "time_bucket_gapfill" && completion.Type == "function" {
				foundGapfill = true
				assert.Contains(t, completion.Documentation, "gap")
			}
			if completion.Name == "time_bucket_ng" && completion.Type == "function" {
				foundTzTimeBucket = true
				assert.Contains(t, completion.Documentation, "timezone")
			}
		}

		assert.True(t, foundBasicTimeBucket, "time_bucket function completion not found")
		assert.True(t, foundGapfill, "time_bucket_gapfill function completion not found")
		assert.True(t, foundTzTimeBucket, "time_bucket_ng function completion not found")

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	t.Run("get_hypertable_function_completions", func(t *testing.T) {
		// Set up expectations for the mock
		mockUseCase.On("GetDatabaseType", "timescale_db").Return("postgres", nil).Once()
		mockUseCase.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return sql == "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(`[{"extversion": "2.8.0"}]`, nil).Once()

		// Create the completion provider
		provider := mcp.NewTimescaleDBCompletionProvider()

		// Call the method to get hypertable function completions
		completions, err := provider.GetHypertableFunctionCompletions(ctx, "timescale_db", mockUseCase)

		// Verify the result
		assert.NoError(t, err)
		assert.NotNil(t, completions)
		assert.NotEmpty(t, completions)

		// Check for essential hypertable functions
		var foundCreate, foundCompression, foundRetention bool
		for _, completion := range completions {
			if completion.Name == "create_hypertable" && completion.Type == "function" {
				foundCreate = true
				assert.Contains(t, completion.Documentation, "hypertable")
				assert.Contains(t, completion.InsertText, "create_hypertable")
			}
			if completion.Name == "add_compression_policy" && completion.Type == "function" {
				foundCompression = true
				assert.Contains(t, completion.Documentation, "compression")
			}
			if completion.Name == "add_retention_policy" && completion.Type == "function" {
				foundRetention = true
				assert.Contains(t, completion.Documentation, "retention")
			}
		}

		assert.True(t, foundCreate, "create_hypertable function completion not found")
		assert.True(t, foundCompression, "add_compression_policy function completion not found")
		assert.True(t, foundRetention, "add_retention_policy function completion not found")

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	t.Run("get_all_function_completions", func(t *testing.T) {
		// Create a separate mock for this test to avoid issues with expectations
		localMock := new(MockDatabaseUseCase)

		// The new implementation makes fewer calls to GetDatabaseType
		localMock.On("GetDatabaseType", "timescale_db").Return("postgres", nil).Once()

		// It also calls ExecuteStatement once through DetectTimescaleDB
		localMock.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return sql == "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(`[{"extversion": "2.8.0"}]`, nil).Once()

		// Create the completion provider
		provider := mcp.NewTimescaleDBCompletionProvider()

		// Call the method to get all function completions
		completions, err := provider.GetAllFunctionCompletions(ctx, "timescale_db", localMock)

		// Verify the result
		assert.NoError(t, err)
		assert.NotNil(t, completions)
		assert.NotEmpty(t, completions)

		// Check for categories of functions
		var foundTimeBucket, foundHypertable, foundContinuousAggregates, foundAnalytics bool
		for _, completion := range completions {
			if completion.Name == "time_bucket" && completion.Type == "function" {
				foundTimeBucket = true
			}
			if completion.Name == "create_hypertable" && completion.Type == "function" {
				foundHypertable = true
			}
			if completion.Name == "create_materialized_view" && completion.Type == "function" {
				foundContinuousAggregates = true
				// Special case - materialized view does not include parentheses
				assert.Contains(t, completion.InsertText, "CREATE MATERIALIZED VIEW")
			}
			if completion.Name == "first" || completion.Name == "last" || completion.Name == "time_weight" {
				foundAnalytics = true
			}
		}

		assert.True(t, foundTimeBucket, "time_bucket function completion not found")
		assert.True(t, foundHypertable, "hypertable function completion not found")
		assert.True(t, foundContinuousAggregates, "continuous aggregate function completion not found")
		assert.True(t, foundAnalytics, "analytics function completion not found")

		// Check that returned completions have properly formatted insert text
		for _, completion := range completions {
			if completion.Type == "function" && completion.Name != "create_materialized_view" {
				assert.Contains(t, completion.InsertText, completion.Name+"(")
				assert.Contains(t, completion.Documentation, "TimescaleDB")
			}
		}

		// Verify the mock expectations
		localMock.AssertExpectations(t)
	})

	t.Run("get_function_completions_with_non_timescaledb", func(t *testing.T) {
		// Create a separate mock for this test to avoid issues with expectations
		localMock := new(MockDatabaseUseCase)

		// With the new implementation, we only need one GetDatabaseType call
		localMock.On("GetDatabaseType", "postgres_db").Return("postgres", nil).Once()

		// It also calls ExecuteStatement through DetectTimescaleDB
		localMock.On("ExecuteStatement", mock.Anything, "postgres_db", mock.MatchedBy(func(sql string) bool {
			return sql == "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(`[]`, nil).Once()

		// Create the completion provider
		provider := mcp.NewTimescaleDBCompletionProvider()

		// Call the method to get function completions
		completions, err := provider.GetAllFunctionCompletions(ctx, "postgres_db", localMock)

		// Verify the result
		assert.Error(t, err)
		assert.Nil(t, completions)
		assert.Contains(t, err.Error(), "TimescaleDB is not available")

		// Verify the mock expectations
		localMock.AssertExpectations(t)
	})

	t.Run("get_function_completions_with_non_postgres", func(t *testing.T) {
		// Create a separate mock for this test
		localMock := new(MockDatabaseUseCase)

		// Set up expectations for the mock
		localMock.On("GetDatabaseType", "mysql_db").Return("mysql", nil).Once()

		// Create the completion provider
		provider := mcp.NewTimescaleDBCompletionProvider()

		// Call the method to get function completions
		completions, err := provider.GetAllFunctionCompletions(ctx, "mysql_db", localMock)

		// Verify the result
		assert.Error(t, err)
		assert.Nil(t, completions)
		// The error message is now "not available" instead of "not a PostgreSQL database"
		assert.Contains(t, err.Error(), "not available")

		// Verify the mock expectations
		localMock.AssertExpectations(t)
	})
}
