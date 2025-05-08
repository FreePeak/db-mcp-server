package context_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/FreePeak/db-mcp-server/internal/delivery/mcp"
)

func TestHypertableSchemaProvider(t *testing.T) {
	// Create a mock use case provider
	mockUseCase := new(MockDatabaseUseCase)

	// Create a context for testing
	ctx := context.Background()

	t.Run("get_hypertable_schema", func(t *testing.T) {
		// Sample results for hypertable metadata queries
		sampleMetadataResult := `[{
			"table_name": "temperature_readings",
			"schema_name": "public",
			"owner": "postgres",
			"time_dimension": "timestamp",
			"time_dimension_type": "TIMESTAMP",
			"chunk_time_interval": "1 day",
			"total_size": "24 MB",
			"chunks": 30,
			"total_rows": 1000000,
			"compression_enabled": true
		}]`

		sampleColumnsResult := `[
			{
				"column_name": "timestamp",
				"data_type": "timestamp without time zone",
				"is_nullable": false,
				"is_primary_key": false,
				"is_indexed": true,
				"description": "Time when reading was taken"
			},
			{
				"column_name": "device_id",
				"data_type": "text",
				"is_nullable": false,
				"is_primary_key": false,
				"is_indexed": true,
				"description": "Device identifier"
			},
			{
				"column_name": "temperature",
				"data_type": "double precision",
				"is_nullable": false,
				"is_primary_key": false,
				"is_indexed": false,
				"description": "Temperature in Celsius"
			},
			{
				"column_name": "humidity",
				"data_type": "double precision",
				"is_nullable": true,
				"is_primary_key": false,
				"is_indexed": false,
				"description": "Relative humidity percentage"
			},
			{
				"column_name": "id",
				"data_type": "integer",
				"is_nullable": false,
				"is_primary_key": true,
				"is_indexed": true,
				"description": "Primary key"
			}
		]`

		sampleCompressionResult := `[{
			"segmentby": "device_id",
			"orderby": "timestamp",
			"compression_interval": "7 days"
		}]`

		sampleRetentionResult := `[{
			"hypertable_name": "temperature_readings",
			"retention_interval": "90 days",
			"retention_enabled": true
		}]`

		// Set up expectations for the mock
		mockUseCase.On("GetDatabaseType", "timescale_db").Return("postgres", nil).Once()
		mockUseCase.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return sql == "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(`[{"extversion": "2.8.0"}]`, nil).Once()

		// Metadata query
		mockUseCase.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return sql != "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'" &&
				strings.Contains(sql, "hypertable")
		}), mock.Anything).Return(sampleMetadataResult, nil).Once()

		// Columns query
		mockUseCase.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "information_schema.columns") &&
				strings.Contains(sql, "temperature_readings")
		}), mock.Anything).Return(sampleColumnsResult, nil).Once()

		// Compression query
		mockUseCase.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "compression_settings") &&
				strings.Contains(sql, "temperature_readings")
		}), mock.Anything).Return(sampleCompressionResult, nil).Once()

		// Retention query
		mockUseCase.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "retention")
		}), mock.Anything).Return(sampleRetentionResult, nil).Once()

		// Create the schema provider
		provider := mcp.NewHypertableSchemaProvider()

		// Call the method
		schemaInfo, err := provider.GetHypertableSchema(ctx, "timescale_db", "temperature_readings", mockUseCase)

		// Verify the result
		assert.NoError(t, err)
		assert.NotNil(t, schemaInfo)
		assert.Equal(t, "temperature_readings", schemaInfo.TableName)
		assert.Equal(t, "public", schemaInfo.SchemaName)
		assert.Equal(t, "timestamp", schemaInfo.TimeColumn)
		assert.Equal(t, "1 day", schemaInfo.ChunkTimeInterval)
		assert.Equal(t, "24 MB", schemaInfo.Size)
		assert.Equal(t, 30, schemaInfo.ChunkCount)
		assert.Equal(t, int64(1000000), schemaInfo.RowCount)
		assert.True(t, schemaInfo.CompressionEnabled)
		assert.Equal(t, "device_id", schemaInfo.CompressionConfig.SegmentBy)
		assert.Equal(t, "timestamp", schemaInfo.CompressionConfig.OrderBy)
		assert.Equal(t, "7 days", schemaInfo.CompressionConfig.Interval)
		assert.True(t, schemaInfo.RetentionEnabled)
		assert.Equal(t, "90 days", schemaInfo.RetentionInterval)

		// Check columns
		assert.Len(t, schemaInfo.Columns, 5)

		// Check time column
		timeCol := findColumn(schemaInfo.Columns, "timestamp")
		assert.NotNil(t, timeCol)
		assert.Equal(t, "timestamp without time zone", timeCol.Type)
		assert.Equal(t, "Time when reading was taken", timeCol.Description)
		assert.False(t, timeCol.Nullable)
		assert.True(t, timeCol.Indexed)

		// Check primary key
		idCol := findColumn(schemaInfo.Columns, "id")
		assert.NotNil(t, idCol)
		assert.True(t, idCol.PrimaryKey)

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	t.Run("get_hypertable_schema_with_non_timescaledb", func(t *testing.T) {
		// Set up expectations for the mock
		mockUseCase.On("GetDatabaseType", "postgres_db").Return("postgres", nil).Once()
		mockUseCase.On("ExecuteStatement", mock.Anything, "postgres_db", mock.MatchedBy(func(sql string) bool {
			return sql == "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(`[]`, nil).Once()

		// Create the schema provider
		provider := mcp.NewHypertableSchemaProvider()

		// Call the method
		schemaInfo, err := provider.GetHypertableSchema(ctx, "postgres_db", "some_table", mockUseCase)

		// Verify the result
		assert.Error(t, err)
		assert.Nil(t, schemaInfo)
		assert.Contains(t, err.Error(), "TimescaleDB is not available")

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	t.Run("get_hypertable_schema_with_not_a_hypertable", func(t *testing.T) {
		// Set up expectations for the mock
		mockUseCase.On("GetDatabaseType", "timescale_db").Return("postgres", nil).Once()
		mockUseCase.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return sql == "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(`[{"extversion": "2.8.0"}]`, nil).Once()

		// Empty result for metadata query indicates it's not a hypertable
		mockUseCase.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return sql != "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(`[]`, nil).Once()

		// Create the schema provider
		provider := mcp.NewHypertableSchemaProvider()

		// Call the method
		schemaInfo, err := provider.GetHypertableSchema(ctx, "timescale_db", "normal_table", mockUseCase)

		// Verify the result
		assert.Error(t, err)
		assert.Nil(t, schemaInfo)
		assert.Contains(t, err.Error(), "is not a hypertable")

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})
}

// Helper function to find a column by name
func findColumn(columns []mcp.HypertableColumnInfo, name string) *mcp.HypertableColumnInfo {
	for i, col := range columns {
		if col.Name == name {
			return &columns[i]
		}
	}
	return nil
}
