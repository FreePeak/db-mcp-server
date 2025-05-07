package timescale

import (
	"context"
	"fmt"
	"strings"
)

// ContinuousAggregateOptions encapsulates options for creating a continuous aggregate
type ContinuousAggregateOptions struct {
	// Required parameters
	ViewName       string // Name of the continuous aggregate view to create
	SourceTable    string // Source table with raw data
	TimeColumn     string // Time column to bucket
	BucketInterval string // Time bucket interval (e.g., '1 hour', '1 day')

	// Optional parameters
	Aggregations      []ColumnAggregation // Aggregations to include in the view
	WhereCondition    string              // WHERE condition to filter source data
	WithData          bool                // Whether to materialize data immediately (WITH DATA)
	RefreshPolicy     bool                // Whether to add a refresh policy
	RefreshInterval   string              // Refresh interval (default: '1 day')
	RefreshLookback   string              // How far back to look when refreshing (default: '1 week')
	MaterializedOnly  bool                // Whether to materialize only (no real-time)
	CreateIfNotExists bool                // Whether to use IF NOT EXISTS
}

// ContinuousAggregatePolicyOptions encapsulates options for refresh policies
type ContinuousAggregatePolicyOptions struct {
	ViewName         string // Name of the continuous aggregate view
	Start            string // Start offset (e.g., '-2 days')
	End              string // End offset (e.g., 'now()')
	ScheduleInterval string // Execution interval (e.g., '1 hour')
}

// CreateContinuousAggregate creates a new continuous aggregate view
func (t *DB) CreateContinuousAggregate(ctx context.Context, options ContinuousAggregateOptions) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	var builder strings.Builder

	// Build CREATE MATERIALIZED VIEW statement
	builder.WriteString("CREATE MATERIALIZED VIEW ")

	// Add IF NOT EXISTS clause if requested
	if options.CreateIfNotExists {
		builder.WriteString("IF NOT EXISTS ")
	}

	// Add view name
	builder.WriteString(options.ViewName)
	builder.WriteString("\n")

	// Add WITH clause for materialized_only if requested
	if options.MaterializedOnly {
		builder.WriteString("WITH (timescaledb.materialized_only=true)\n")
	}

	// Start SELECT statement
	builder.WriteString("AS SELECT\n    ")

	// Add time bucket
	builder.WriteString(fmt.Sprintf("time_bucket('%s', %s) as time_bucket",
		options.BucketInterval, options.TimeColumn))

	// Add aggregations
	if len(options.Aggregations) > 0 {
		for _, agg := range options.Aggregations {
			colName := agg.Alias
			if colName == "" {
				colName = strings.ToLower(string(agg.Function)) + "_" + agg.Column
			}

			builder.WriteString(fmt.Sprintf(",\n    %s(%s) as %s",
				agg.Function, agg.Column, colName))
		}
	} else {
		// Default to count(*) if no aggregations specified
		builder.WriteString(",\n    COUNT(*) as count")
	}

	// Add FROM clause
	builder.WriteString(fmt.Sprintf("\nFROM %s\n", options.SourceTable))

	// Add WHERE clause if specified
	if options.WhereCondition != "" {
		builder.WriteString(fmt.Sprintf("WHERE %s\n", options.WhereCondition))
	}

	// Add GROUP BY clause
	builder.WriteString("GROUP BY time_bucket\n")

	// Add WITH DATA or WITH NO DATA
	if options.WithData {
		builder.WriteString("WITH DATA")
	} else {
		builder.WriteString("WITH NO DATA")
	}

	// Execute the statement
	_, err := t.ExecuteSQLWithoutParams(ctx, builder.String())
	if err != nil {
		return fmt.Errorf("failed to create continuous aggregate: %w", err)
	}

	// Add refresh policy if requested
	if options.RefreshPolicy {
		refreshInterval := options.RefreshInterval
		if refreshInterval == "" {
			refreshInterval = "1 day"
		}

		refreshLookback := options.RefreshLookback
		if refreshLookback == "" {
			refreshLookback = "1 week"
		}

		err = t.AddContinuousAggregatePolicy(ctx, ContinuousAggregatePolicyOptions{
			ViewName:         options.ViewName,
			Start:            fmt.Sprintf("-%s", refreshLookback),
			End:              "now()",
			ScheduleInterval: refreshInterval,
		})

		if err != nil {
			return fmt.Errorf("created continuous aggregate but failed to add refresh policy: %w", err)
		}
	}

	return nil
}

// RefreshContinuousAggregate refreshes a continuous aggregate for a specific time range
func (t *DB) RefreshContinuousAggregate(ctx context.Context, viewName, startTime, endTime string) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	var builder strings.Builder

	// Build CALL statement
	builder.WriteString("CALL refresh_continuous_aggregate(")

	// Add view name
	builder.WriteString(fmt.Sprintf("'%s'", viewName))

	// Add time range if specified
	if startTime != "" && endTime != "" {
		builder.WriteString(fmt.Sprintf(", '%s'::timestamptz, '%s'::timestamptz",
			startTime, endTime))
	} else {
		builder.WriteString(", NULL, NULL")
	}

	builder.WriteString(")")

	// Execute the statement
	_, err := t.ExecuteSQLWithoutParams(ctx, builder.String())
	if err != nil {
		return fmt.Errorf("failed to refresh continuous aggregate: %w", err)
	}

	return nil
}

// AddContinuousAggregatePolicy adds a refresh policy to a continuous aggregate
func (t *DB) AddContinuousAggregatePolicy(ctx context.Context, options ContinuousAggregatePolicyOptions) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	// Build policy creation SQL
	sql := fmt.Sprintf(
		"SELECT add_continuous_aggregate_policy('%s', start_offset => INTERVAL '%s', "+
			"end_offset => INTERVAL '%s', schedule_interval => INTERVAL '%s')",
		options.ViewName,
		options.Start,
		options.End,
		options.ScheduleInterval,
	)

	// Execute the statement
	_, err := t.ExecuteSQLWithoutParams(ctx, sql)
	if err != nil {
		return fmt.Errorf("failed to add continuous aggregate policy: %w", err)
	}

	return nil
}

// RemoveContinuousAggregatePolicy removes a refresh policy from a continuous aggregate
func (t *DB) RemoveContinuousAggregatePolicy(ctx context.Context, viewName string) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	// Build policy removal SQL
	sql := fmt.Sprintf(
		"SELECT remove_continuous_aggregate_policy('%s')",
		viewName,
	)

	// Execute the statement
	_, err := t.ExecuteSQLWithoutParams(ctx, sql)
	if err != nil {
		return fmt.Errorf("failed to remove continuous aggregate policy: %w", err)
	}

	return nil
}

// DropContinuousAggregate drops a continuous aggregate
func (t *DB) DropContinuousAggregate(ctx context.Context, viewName string, cascade bool) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	var builder strings.Builder

	// Build DROP statement
	builder.WriteString(fmt.Sprintf("DROP MATERIALIZED VIEW %s", viewName))

	// Add CASCADE if requested
	if cascade {
		builder.WriteString(" CASCADE")
	}

	// Execute the statement
	_, err := t.ExecuteSQLWithoutParams(ctx, builder.String())
	if err != nil {
		return fmt.Errorf("failed to drop continuous aggregate: %w", err)
	}

	return nil
}

// GetContinuousAggregateInfo gets detailed information about a continuous aggregate
func (t *DB) GetContinuousAggregateInfo(ctx context.Context, viewName string) (map[string]interface{}, error) {
	if !t.isTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB extension not available")
	}

	// Query for continuous aggregate information
	query := fmt.Sprintf(`
		WITH policy_info AS (
			SELECT 
				ca.user_view_name,
				p.schedule_interval,
				p.start_offset,
				p.end_offset
			FROM timescaledb_information.continuous_aggregates ca
			LEFT JOIN timescaledb_information.jobs j ON j.hypertable_name = ca.user_view_name
			LEFT JOIN timescaledb_information.policies p ON p.job_id = j.job_id
			WHERE p.proc_name = 'policy_refresh_continuous_aggregate'
			AND ca.view_name = '%s'
		),
		size_info AS (
			SELECT 
				pg_size_pretty(pg_total_relation_size(format('%%I.%%I', schemaname, tablename)))
				as view_size
			FROM pg_tables
			WHERE tablename = '%s'
		)
		SELECT 
			ca.view_name,
			ca.view_schema,
			ca.materialized_only,
			ca.view_definition,
			ca.refresh_lag,
			ca.refresh_interval,
			ca.hypertable_name,
			ca.hypertable_schema,
			pi.schedule_interval,
			pi.start_offset,
			pi.end_offset,
			si.view_size,
			(
				SELECT min(time_bucket) 
				FROM %s
			) as min_time,
			(
				SELECT max(time_bucket) 
				FROM %s
			) as max_time
		FROM timescaledb_information.continuous_aggregates ca
		LEFT JOIN policy_info pi ON pi.user_view_name = ca.user_view_name
		CROSS JOIN size_info si
		WHERE ca.view_name = '%s'
	`, viewName, viewName, viewName, viewName, viewName)

	// Execute query
	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get continuous aggregate info: %w", err)
	}

	// Convert result to map
	rows, ok := result.([]map[string]interface{})
	if !ok || len(rows) == 0 {
		return nil, fmt.Errorf("continuous aggregate '%s' not found", viewName)
	}

	// Extract the first row
	info := rows[0]

	// Add computed fields
	info["has_policy"] = info["schedule_interval"] != nil

	return info, nil
}
