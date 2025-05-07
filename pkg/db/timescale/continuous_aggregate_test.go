package timescale

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestCreateContinuousAggregate(t *testing.T) {
	t.Run("should create a continuous aggregate view", func(t *testing.T) {
		// Setup test with a custom mock DB
		mockDB := NewMockDB()
		mockDB.SetTimescaleAvailable(true)
		tsdb := &DB{
			Database:      mockDB,
			isTimescaleDB: true,
			config: DBConfig{
				UseTimescaleDB: true,
			},
		}
		ctx := context.Background()

		// Create a continuous aggregate
		err := tsdb.CreateContinuousAggregate(ctx, ContinuousAggregateOptions{
			ViewName:       "daily_metrics",
			SourceTable:    "raw_metrics",
			TimeColumn:     "time",
			BucketInterval: "1 day",
			Aggregations: []ColumnAggregation{
				{Function: AggrAvg, Column: "temperature", Alias: "avg_temp"},
				{Function: AggrMax, Column: "temperature", Alias: "max_temp"},
				{Function: AggrMin, Column: "temperature", Alias: "min_temp"},
				{Function: AggrCount, Column: "*", Alias: "count"},
			},
			WithData:      true,
			RefreshPolicy: false, // Set to false to avoid additional queries
		})

		// Assert
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify query contains required elements - since we're checking the last query directly
		lastQuery := mockDB.LastQuery()
		requiredElements := []string{
			"CREATE MATERIALIZED VIEW",
			"daily_metrics",
			"time_bucket",
			"1 day",
			"AVG",
			"MAX",
			"MIN",
			"COUNT",
			"raw_metrics",
			"WITH DATA",
		}

		for _, element := range requiredElements {
			if !strings.Contains(lastQuery, element) {
				t.Errorf("Expected query to contain '%s', but got: %s", element, lastQuery)
			}
		}
	})

	t.Run("should error when TimescaleDB not available", func(t *testing.T) {
		// Setup test with TimescaleDB unavailable
		tsdb, mockDB := MockTimescaleDB(t)
		mockDB.SetTimescaleAvailable(false)
		tsdb.isTimescaleDB = false // Explicitly set this to false
		ctx := context.Background()

		// Create a continuous aggregate
		err := tsdb.CreateContinuousAggregate(ctx, ContinuousAggregateOptions{
			ViewName:       "daily_metrics",
			SourceTable:    "raw_metrics",
			TimeColumn:     "time",
			BucketInterval: "1 day",
			Aggregations: []ColumnAggregation{
				{Function: AggrAvg, Column: "temperature", Alias: "avg_temp"},
			},
		})

		// Assert
		if err == nil {
			t.Fatal("Expected an error when TimescaleDB not available, got none")
		}
	})

	t.Run("should handle database errors", func(t *testing.T) {
		// Setup test with a custom mock DB
		mockDB := NewMockDB()
		mockDB.SetTimescaleAvailable(true)
		tsdb := &DB{
			Database:      mockDB,
			isTimescaleDB: true,
			config: DBConfig{
				UseTimescaleDB: true,
			},
		}
		ctx := context.Background()

		// Register a query result with an error
		mockDB.RegisterQueryResult("CREATE MATERIALIZED VIEW", nil, fmt.Errorf("query error"))

		// Create a continuous aggregate
		err := tsdb.CreateContinuousAggregate(ctx, ContinuousAggregateOptions{
			ViewName:       "daily_metrics",
			SourceTable:    "raw_metrics",
			TimeColumn:     "time",
			BucketInterval: "1 day",
			RefreshPolicy:  false, // Disable to avoid additional queries
		})

		// Assert
		if err == nil {
			t.Fatal("Expected an error, got none")
		}
	})
}

func TestRefreshContinuousAggregate(t *testing.T) {
	t.Run("should refresh a continuous aggregate view", func(t *testing.T) {
		// Setup test
		tsdb, mockDB := MockTimescaleDB(t)
		ctx := context.Background()

		// Set mock behavior
		mockDB.SetQueryResult([]map[string]interface{}{
			{"refreshed": true},
		})

		// Refresh a continuous aggregate with time range
		err := tsdb.RefreshContinuousAggregate(ctx, "daily_metrics", "2023-01-01", "2023-01-31")

		// Assert
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify SQL contains proper calls
		if !mockDB.QueryContains("CALL") || !mockDB.QueryContains("refresh_continuous_aggregate") {
			t.Error("Expected query to call refresh_continuous_aggregate")
		}
	})

	t.Run("should refresh without time range", func(t *testing.T) {
		// Setup test
		tsdb, mockDB := MockTimescaleDB(t)
		ctx := context.Background()

		// Refresh a continuous aggregate without time range
		err := tsdb.RefreshContinuousAggregate(ctx, "daily_metrics", "", "")

		// Assert
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify SQL contains proper calls but no time range
		if !mockDB.QueryContains("CALL") || !mockDB.QueryContains("refresh_continuous_aggregate") {
			t.Error("Expected query to call refresh_continuous_aggregate")
		}

		if !mockDB.QueryContains("NULL, NULL") {
			t.Error("Expected query to use NULL for undefined time ranges")
		}
	})
}

func TestManageContinuousAggregatePolicy(t *testing.T) {
	t.Run("should add a refresh policy", func(t *testing.T) {
		// Setup test
		tsdb, mockDB := MockTimescaleDB(t)
		ctx := context.Background()

		// Add a refresh policy
		err := tsdb.AddContinuousAggregatePolicy(ctx, ContinuousAggregatePolicyOptions{
			ViewName:         "daily_metrics",
			Start:            "-2 days",
			End:              "now()",
			ScheduleInterval: "1 hour",
		})

		// Assert
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify SQL contains proper calls
		if !mockDB.QueryContains("add_continuous_aggregate_policy") {
			t.Error("Expected query to contain add_continuous_aggregate_policy")
		}
	})

	t.Run("should remove a refresh policy", func(t *testing.T) {
		// Setup test
		tsdb, mockDB := MockTimescaleDB(t)
		ctx := context.Background()

		// Remove a refresh policy
		err := tsdb.RemoveContinuousAggregatePolicy(ctx, "daily_metrics")

		// Assert
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify SQL contains proper calls
		if !mockDB.QueryContains("remove_continuous_aggregate_policy") {
			t.Error("Expected query to contain remove_continuous_aggregate_policy")
		}
	})
}

func TestDropContinuousAggregate(t *testing.T) {
	t.Run("should drop a continuous aggregate", func(t *testing.T) {
		// Setup test
		tsdb, mockDB := MockTimescaleDB(t)
		ctx := context.Background()

		// Drop a continuous aggregate
		err := tsdb.DropContinuousAggregate(ctx, "daily_metrics", false)

		// Assert
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify SQL contains proper calls
		if !mockDB.QueryContains("DROP MATERIALIZED VIEW") {
			t.Error("Expected query to contain DROP MATERIALIZED VIEW")
		}
	})

	t.Run("should drop a continuous aggregate with cascade", func(t *testing.T) {
		// Setup test
		tsdb, mockDB := MockTimescaleDB(t)
		ctx := context.Background()

		// Drop a continuous aggregate with cascade
		err := tsdb.DropContinuousAggregate(ctx, "daily_metrics", true)

		// Assert
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify SQL contains proper calls
		if !mockDB.QueryContains("DROP MATERIALIZED VIEW") || !mockDB.QueryContains("CASCADE") {
			t.Error("Expected query to contain DROP MATERIALIZED VIEW with CASCADE")
		}
	})
}
