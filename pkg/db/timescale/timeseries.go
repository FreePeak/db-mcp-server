package timescale

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// WindowFunction represents a SQL window function
type WindowFunction struct {
	Function    string // e.g. LAG, LEAD, ROW_NUMBER
	Expression  string // Expression to apply function to
	Alias       string // Result column name
	PartitionBy string // PARTITION BY column(s)
	OrderBy     string // ORDER BY column(s)
	Frame       string // Window frame specification
}

// TimeSeriesQueryOptions encapsulates options for time-series queries
type TimeSeriesQueryOptions struct {
	// Required parameters
	Table          string // The table to query
	TimeColumn     string // The time column
	BucketInterval string // Time bucket interval (e.g., '1 hour', '1 day')

	// Optional parameters
	BucketColumnName string              // Name for the bucket column (defaults to "time_bucket")
	SelectColumns    []string            // Additional columns to select
	Aggregations     []ColumnAggregation // Aggregations to perform
	WindowFunctions  []WindowFunction    // Window functions to apply
	StartTime        time.Time           // Start of time range
	EndTime          time.Time           // End of time range
	WhereCondition   string              // Additional WHERE conditions
	GroupByColumns   []string            // Additional GROUP BY columns
	OrderBy          string              // ORDER BY clause
	Limit            int                 // LIMIT clause
	Offset           int                 // OFFSET clause
}

// TimeSeriesQuery executes a time-series query with the given options
func (t *DB) TimeSeriesQuery(ctx context.Context, options TimeSeriesQueryOptions) ([]map[string]interface{}, error) {
	if !t.isTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB extension not available")
	}

	// Initialize query builder
	builder := NewTimeseriesQueryBuilder(options.Table)

	// Add time bucket
	bucketName := options.BucketColumnName
	if bucketName == "" {
		bucketName = "time_bucket"
	}
	builder.WithTimeBucket(options.BucketInterval, options.TimeColumn, bucketName)

	// Add select columns
	if len(options.SelectColumns) > 0 {
		builder.Select(options.SelectColumns...)
	}

	// Add aggregations
	for _, agg := range options.Aggregations {
		builder.Aggregate(agg.Function, agg.Column, agg.Alias)
	}

	// Add time range if specified
	if !options.StartTime.IsZero() && !options.EndTime.IsZero() {
		builder.WhereTimeRange(options.TimeColumn, options.StartTime, options.EndTime)
	}

	// Add additional WHERE condition if specified
	if options.WhereCondition != "" {
		builder.Where(options.WhereCondition)
	}

	// Add GROUP BY columns
	if len(options.GroupByColumns) > 0 {
		builder.GroupBy(options.GroupByColumns...)
	}

	// Add ORDER BY if specified
	if options.OrderBy != "" {
		orderCols := strings.Split(options.OrderBy, ",")
		for i := range orderCols {
			orderCols[i] = strings.TrimSpace(orderCols[i])
		}
		builder.OrderBy(orderCols...)
	} else {
		// Default sort by time bucket
		builder.OrderBy(bucketName)
	}

	// Add LIMIT if specified
	if options.Limit > 0 {
		builder.Limit(options.Limit)
	}

	// Add OFFSET if specified
	if options.Offset > 0 {
		builder.Offset(options.Offset)
	}

	// Generate the query
	query, args := builder.Build()

	// Add window functions if specified
	if len(options.WindowFunctions) > 0 {
		query = addWindowFunctions(query, options.WindowFunctions)
	}

	// Execute the query
	result, err := t.ExecuteSQL(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute time-series query: %w", err)
	}

	// Convert result to expected format
	rows, ok := result.([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type from database query")
	}

	return rows, nil
}

// addWindowFunctions modifies a query to include window functions
func addWindowFunctions(query string, functions []WindowFunction) string {
	// If no window functions, return original query
	if len(functions) == 0 {
		return query
	}

	// Split query at FROM to insert window functions
	parts := strings.SplitN(query, "FROM", 2)
	if len(parts) != 2 {
		return query // Can't modify query structure
	}

	// Build window function part
	var windowPart strings.Builder
	windowPart.WriteString(parts[0])

	// Add comma after existing selections
	trimmedSelect := strings.TrimSpace(parts[0][7:]) // Remove "SELECT " prefix
	if trimmedSelect != "" && len(trimmedSelect) > 0 && !strings.HasSuffix(trimmedSelect, ",") {
		windowPart.WriteString(", ")
	}

	// Add each window function
	for i, fn := range functions {
		windowPart.WriteString(fmt.Sprintf("%s(%s) OVER (", fn.Function, fn.Expression))

		// Add PARTITION BY if specified
		if fn.PartitionBy != "" {
			windowPart.WriteString(fmt.Sprintf("PARTITION BY %s ", fn.PartitionBy))
		}

		// Add ORDER BY if specified
		if fn.OrderBy != "" {
			windowPart.WriteString(fmt.Sprintf("ORDER BY %s ", fn.OrderBy))
		}

		// Add window frame if specified
		if fn.Frame != "" {
			windowPart.WriteString(fn.Frame)
		}

		windowPart.WriteString(")")

		// Add alias if specified
		if fn.Alias != "" {
			windowPart.WriteString(fmt.Sprintf(" AS %s", fn.Alias))
		}

		// Add comma if not last function
		if i < len(functions)-1 {
			windowPart.WriteString(", ")
		}
	}

	// Reconstruct query
	windowPart.WriteString(" FROM")
	windowPart.WriteString(parts[1])

	return windowPart.String()
}

// GetCommonTimeIntervals returns a list of supported time bucket intervals
func (t *DB) GetCommonTimeIntervals() []string {
	return []string{
		"1 minute", "5 minutes", "10 minutes", "15 minutes", "30 minutes",
		"1 hour", "2 hours", "3 hours", "6 hours", "12 hours",
		"1 day", "1 week", "1 month", "3 months", "6 months", "1 year",
	}
}

// AnalyzeTimeSeries performs analysis on time-series data
func (t *DB) AnalyzeTimeSeries(ctx context.Context, table, timeColumn string,
	startTime, endTime time.Time) (map[string]interface{}, error) {

	if !t.isTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB extension not available")
	}

	// Get basic statistics about the time range
	statsQuery := fmt.Sprintf(`
		SELECT 
			COUNT(*) as row_count,
			MIN(%s) as min_time,
			MAX(%s) as max_time,
			(MAX(%s) - MIN(%s)) as time_span,
			COUNT(DISTINCT date_trunc('day', %s)) as unique_days
		FROM %s
		WHERE %s BETWEEN $1 AND $2
	`, timeColumn, timeColumn, timeColumn, timeColumn, timeColumn, table, timeColumn)

	statsResult, err := t.ExecuteSQL(ctx, statsQuery, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get time-series statistics: %w", err)
	}

	statsRows, ok := statsResult.([]map[string]interface{})
	if !ok || len(statsRows) == 0 {
		return nil, fmt.Errorf("unexpected result type from database query")
	}

	// Build result
	result := statsRows[0]

	// Add suggested bucket intervals based on data characteristics
	if rowCount, ok := result["row_count"].(int64); ok && rowCount > 0 {
		// Get time span in hours
		var timeSpanHours float64
		if timeSpan, ok := result["time_span"].(string); ok {
			timeSpanHours = parseTimeInterval(timeSpan)
		}

		if timeSpanHours > 0 {
			// Suggest reasonable intervals based on amount of data and time span
			if timeSpanHours <= 24 {
				result["suggested_interval"] = "5 minutes"
			} else if timeSpanHours <= 168 { // 1 week
				result["suggested_interval"] = "1 hour"
			} else if timeSpanHours <= 720 { // 1 month
				result["suggested_interval"] = "6 hours"
			} else if timeSpanHours <= 2160 { // 3 months
				result["suggested_interval"] = "1 day"
			} else {
				result["suggested_interval"] = "1 week"
			}
		}
	}

	return result, nil
}

// parseTimeInterval converts a PostgreSQL interval string to hours
func parseTimeInterval(interval string) float64 {
	// This is a simplistic parser for time intervals
	// Real implementation would need to handle more formats
	if strings.Contains(interval, "days") {
		parts := strings.Split(interval, "days")
		if len(parts) > 0 {
			var days float64
			fmt.Sscanf(parts[0], "%f", &days)
			return days * 24
		}
	}
	return 0
}
