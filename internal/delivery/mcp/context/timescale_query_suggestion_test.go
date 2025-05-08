package context_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/FreePeak/db-mcp-server/internal/delivery/mcp"
)

func TestTimescaleDBQuerySuggestions(t *testing.T) {
	// Create a mock use case provider
	mockUseCase := new(MockDatabaseUseCase)

	// Create a context for testing
	ctx := context.Background()

	t.Run("get_query_suggestions_with_hypertables", func(t *testing.T) {
		// Set up expectations for the mock
		mockUseCase.On("GetDatabaseType", "timescale_db").Return("postgres", nil).Once()
		mockUseCase.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return sql == "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(`[{"extversion": "2.8.0"}]`, nil).Once()

		// Mock the hypertable query
		mockUseCase.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return sql != "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(`[{"table_name": "metrics", "time_column": "timestamp", "chunk_interval": "604800000000"}]`, nil).Once()

		// Create the completion provider
		provider := mcp.NewTimescaleDBCompletionProvider()

		// Call the method to get query suggestions
		suggestions, err := provider.GetQuerySuggestions(ctx, "timescale_db", mockUseCase)

		// Verify the result
		assert.NoError(t, err)
		assert.NotNil(t, suggestions)
		assert.NotEmpty(t, suggestions)

		// Check for generic suggestions
		var foundGenericTimeBucket, foundGenericCompression, foundGenericDiagnostics bool
		// Check for schema-specific suggestions
		var foundSpecificTimeBucket, foundSpecificRetention, foundSpecificQuery bool

		for _, suggestion := range suggestions {
			// Check generic suggestions
			if suggestion.Title == "Basic Time Bucket Aggregation" {
				foundGenericTimeBucket = true
				assert.Contains(t, suggestion.Query, "time_bucket")
				assert.Equal(t, "Time Buckets", suggestion.Category)
			}
			if suggestion.Title == "Add Compression Policy" {
				foundGenericCompression = true
				assert.Contains(t, suggestion.Query, "add_compression_policy")
			}
			if suggestion.Title == "Job Stats" {
				foundGenericDiagnostics = true
				assert.Contains(t, suggestion.Query, "timescaledb_information.jobs")
			}

			// Check schema-specific suggestions
			if suggestion.Title == "Time Bucket Aggregation for metrics" {
				foundSpecificTimeBucket = true
				assert.Contains(t, suggestion.Query, "metrics")
				assert.Contains(t, suggestion.Query, "timestamp")
			}
			if suggestion.Title == "Retention Policy for metrics" {
				foundSpecificRetention = true
				assert.Contains(t, suggestion.Query, "metrics")
			}
			if suggestion.Title == "Recent Data from metrics" {
				foundSpecificQuery = true
				assert.Contains(t, suggestion.Query, "metrics")
				assert.Contains(t, suggestion.Query, "timestamp")
			}
		}

		// Verify we found all the expected suggestion types
		assert.True(t, foundGenericTimeBucket, "generic time bucket suggestion not found")
		assert.True(t, foundGenericCompression, "generic compression policy suggestion not found")
		assert.True(t, foundGenericDiagnostics, "generic diagnostics suggestion not found")

		assert.True(t, foundSpecificTimeBucket, "specific time bucket suggestion not found")
		assert.True(t, foundSpecificRetention, "specific retention policy suggestion not found")
		assert.True(t, foundSpecificQuery, "specific data query suggestion not found")

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	t.Run("get_query_suggestions_without_hypertables", func(t *testing.T) {
		// Create a separate mock for this test
		localMock := new(MockDatabaseUseCase)

		// Set up expectations for the mock
		localMock.On("GetDatabaseType", "timescale_db").Return("postgres", nil).Once()
		localMock.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return sql == "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(`[{"extversion": "2.8.0"}]`, nil).Once()

		// Mock the hypertable query with empty results
		localMock.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return sql != "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(`[]`, nil).Once()

		// Create the completion provider
		provider := mcp.NewTimescaleDBCompletionProvider()

		// Call the method to get query suggestions
		suggestions, err := provider.GetQuerySuggestions(ctx, "timescale_db", localMock)

		// Verify the result
		assert.NoError(t, err)
		assert.NotNil(t, suggestions)
		assert.NotEmpty(t, suggestions)

		// We should only get generic suggestions, no schema-specific ones
		for _, suggestion := range suggestions {
			assert.NotContains(t, suggestion.Title, "metrics", "should not contain schema-specific suggestions")
		}

		// Check generic suggestion count (should be 11 as defined in the function)
		assert.Len(t, suggestions, 11, "should have 11 generic suggestions")

		// Verify the mock expectations
		localMock.AssertExpectations(t)
	})

	t.Run("get_query_suggestions_with_non_timescaledb", func(t *testing.T) {
		// Create a separate mock for this test
		localMock := new(MockDatabaseUseCase)

		// Set up expectations for the mock
		localMock.On("GetDatabaseType", "postgres_db").Return("postgres", nil).Once()
		localMock.On("ExecuteStatement", mock.Anything, "postgres_db", mock.MatchedBy(func(sql string) bool {
			return sql == "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(`[]`, nil).Once()

		// Create the completion provider
		provider := mcp.NewTimescaleDBCompletionProvider()

		// Call the method to get query suggestions
		suggestions, err := provider.GetQuerySuggestions(ctx, "postgres_db", localMock)

		// Verify the result
		assert.Error(t, err)
		assert.Nil(t, suggestions)
		assert.Contains(t, err.Error(), "TimescaleDB is not available")

		// Verify the mock expectations
		localMock.AssertExpectations(t)
	})

	t.Run("get_query_suggestions_with_non_postgres", func(t *testing.T) {
		// Create a separate mock for this test
		localMock := new(MockDatabaseUseCase)

		// Set up expectations for the mock
		localMock.On("GetDatabaseType", "mysql_db").Return("mysql", nil).Once()

		// Create the completion provider
		provider := mcp.NewTimescaleDBCompletionProvider()

		// Call the method to get query suggestions
		suggestions, err := provider.GetQuerySuggestions(ctx, "mysql_db", localMock)

		// Verify the result
		assert.Error(t, err)
		assert.Nil(t, suggestions)
		assert.Contains(t, err.Error(), "not available")

		// Verify the mock expectations
		localMock.AssertExpectations(t)
	})
}
