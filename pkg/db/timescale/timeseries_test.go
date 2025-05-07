package timescale

import (
	"context"
	"testing"
	"time"
)

func TestTimeSeriesQuery(t *testing.T) {
	t.Run("should build and execute time series query", func(t *testing.T) {
		// Setup test
		tsdb, mockDB := MockTimescaleDB(t)
		ctx := context.Background()

		// Set mock behavior with non-empty result
		mockDB.SetQueryResult([]map[string]interface{}{
			{
				"time_bucket": time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				"avg_value":   23.5,
				"count":       10,
			},
		})

		// Create a time series query
		result, err := tsdb.TimeSeriesQuery(ctx, TimeSeriesQueryOptions{
			Table:            "metrics",
			TimeColumn:       "time",
			BucketInterval:   "1 hour",
			BucketColumnName: "bucket",
			Aggregations: []ColumnAggregation{
				{Function: AggrAvg, Column: "value", Alias: "avg_value"},
				{Function: AggrCount, Column: "*", Alias: "count"},
			},
			StartTime: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
			Limit:     100,
		})

		// Assert
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if len(result) != 1 {
			t.Fatalf("Expected 1 result, got: %d", len(result))
		}

		// Verify query contains expected elements
		if !mockDB.QueryContains("time_bucket") {
			t.Error("Expected query to contain time_bucket function")
		}
		if !mockDB.QueryContains("FROM metrics") {
			t.Error("Expected query to contain FROM metrics")
		}
		if !mockDB.QueryContains("AVG(value)") {
			t.Error("Expected query to contain AVG(value)")
		}
		if !mockDB.QueryContains("COUNT(*)") {
			t.Error("Expected query to contain COUNT(*)")
		}
	})

	t.Run("should handle additional where conditions", func(t *testing.T) {
		// Setup test
		tsdb, mockDB := MockTimescaleDB(t)
		ctx := context.Background()

		// Set mock behavior
		mockDB.SetQueryResult([]map[string]interface{}{
			{
				"time_bucket": time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				"avg_value":   23.5,
			},
		})

		// Create a time series query with additional where clause
		_, err := tsdb.TimeSeriesQuery(ctx, TimeSeriesQueryOptions{
			Table:            "metrics",
			TimeColumn:       "time",
			BucketInterval:   "1 hour",
			BucketColumnName: "bucket",
			Aggregations: []ColumnAggregation{
				{Function: AggrAvg, Column: "value", Alias: "avg_value"},
			},
			StartTime:      time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			EndTime:        time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
			WhereCondition: "sensor_id = 1",
			GroupByColumns: []string{"sensor_id"},
		})

		// Assert
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify query contains where condition
		if !mockDB.QueryContains("sensor_id = 1") {
			t.Error("Expected query to contain sensor_id = 1")
		}
		if !mockDB.QueryContains("GROUP BY") && !mockDB.QueryContains("sensor_id") {
			t.Error("Expected query to contain GROUP BY with sensor_id")
		}
	})

	t.Run("should error when TimescaleDB not available", func(t *testing.T) {
		// Setup test with TimescaleDB unavailable
		tsdb, mockDB := MockTimescaleDB(t)
		mockDB.SetTimescaleAvailable(false)
		tsdb.isTimescaleDB = false // Explicitly set to false
		ctx := context.Background()

		// Create a time series query
		_, err := tsdb.TimeSeriesQuery(ctx, TimeSeriesQueryOptions{
			Table:          "metrics",
			TimeColumn:     "time",
			BucketInterval: "1 hour",
		})

		// Assert
		if err == nil {
			t.Fatal("Expected an error when TimescaleDB not available, got none")
		}
	})

	t.Run("should handle database errors", func(t *testing.T) {
		// Setup test
		tsdb, mockDB := MockTimescaleDB(t)
		ctx := context.Background()

		// Set mock to return error
		mockDB.SetError("query error")

		// Create a time series query
		_, err := tsdb.TimeSeriesQuery(ctx, TimeSeriesQueryOptions{
			Table:          "metrics",
			TimeColumn:     "time",
			BucketInterval: "1 hour",
		})

		// Assert
		if err == nil {
			t.Fatal("Expected an error, got none")
		}
	})
}

func TestAdvancedTimeSeriesFeatures(t *testing.T) {
	t.Run("should handle time bucketing with different intervals", func(t *testing.T) {
		intervals := []string{"1 minute", "1 hour", "1 day", "1 week", "1 month", "1 year"}

		for _, interval := range intervals {
			t.Run(interval, func(t *testing.T) {
				// Setup test
				tsdb, mockDB := MockTimescaleDB(t)
				ctx := context.Background()

				// Set mock behavior
				mockDB.SetQueryResult([]map[string]interface{}{
					{"time_bucket": time.Now(), "value": 42.0},
				})

				// Create a time series query
				_, err := tsdb.TimeSeriesQuery(ctx, TimeSeriesQueryOptions{
					Table:          "metrics",
					TimeColumn:     "time",
					BucketInterval: interval,
				})

				// Assert
				if err != nil {
					t.Fatalf("Expected no error for interval %s, got: %v", interval, err)
				}

				// Verify query contains the right time bucket interval
				if !mockDB.QueryContains(interval) {
					t.Error("Expected query to contain interval", interval)
				}
			})
		}
	})

	t.Run("should apply window functions when requested", func(t *testing.T) {
		// Setup test
		tsdb, mockDB := MockTimescaleDB(t)
		ctx := context.Background()

		// Set mock behavior
		mockDB.SetQueryResult([]map[string]interface{}{
			{"time_bucket": time.Now(), "avg_value": 42.0, "prev_avg": 40.0},
		})

		// Create a time series query
		_, err := tsdb.TimeSeriesQuery(ctx, TimeSeriesQueryOptions{
			Table:          "metrics",
			TimeColumn:     "time",
			BucketInterval: "1 hour",
			Aggregations: []ColumnAggregation{
				{Function: AggrAvg, Column: "value", Alias: "avg_value"},
			},
			WindowFunctions: []WindowFunction{
				{
					Function:    "LAG",
					Expression:  "avg_value",
					Alias:       "prev_avg",
					PartitionBy: "sensor_id",
					OrderBy:     "time_bucket",
				},
			},
		})

		// Assert
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify query contains window function
		if !mockDB.QueryContains("LAG") {
			t.Error("Expected query to contain LAG window function")
		}
		if !mockDB.QueryContains("PARTITION BY") {
			t.Error("Expected query to contain PARTITION BY clause")
		}
	})
}
