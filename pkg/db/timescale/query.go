package timescale

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// TimeBucket represents a time bucket for time-series aggregation
type TimeBucket struct {
	Interval string // e.g., '1 hour', '1 day', '1 month'
	Column   string // Time column to bucket
	Alias    string // Optional alias for the bucket column
}

// AggregateFunction represents a common aggregate function
type AggregateFunction string

const (
	// AggrAvg calculates the average value of a column
	AggrAvg AggregateFunction = "AVG"
	// AggrSum calculates the sum of values in a column
	AggrSum AggregateFunction = "SUM"
	// AggrMin finds the minimum value in a column
	AggrMin AggregateFunction = "MIN"
	// AggrMax finds the maximum value in a column
	AggrMax AggregateFunction = "MAX"
	// AggrCount counts the number of rows
	AggrCount AggregateFunction = "COUNT"
	// AggrFirst takes the first value in a window
	AggrFirst AggregateFunction = "FIRST"
	// AggrLast takes the last value in a window
	AggrLast AggregateFunction = "LAST"
)

// ColumnAggregation represents an aggregation operation on a column
type ColumnAggregation struct {
	Function AggregateFunction
	Column   string
	Alias    string
}

// TimeseriesQueryBuilder helps build optimized time-series queries
type TimeseriesQueryBuilder struct {
	table        string
	timeBucket   *TimeBucket
	selectCols   []string
	aggregations []ColumnAggregation
	whereClauses []string
	whereArgs    []interface{}
	groupByCols  []string
	orderByCols  []string
	limit        int
	offset       int
}

// NewTimeseriesQueryBuilder creates a new builder for a specific table
func NewTimeseriesQueryBuilder(table string) *TimeseriesQueryBuilder {
	return &TimeseriesQueryBuilder{
		table:        table,
		selectCols:   make([]string, 0),
		aggregations: make([]ColumnAggregation, 0),
		whereClauses: make([]string, 0),
		whereArgs:    make([]interface{}, 0),
		groupByCols:  make([]string, 0),
		orderByCols:  make([]string, 0),
	}
}

// WithTimeBucket adds a time bucket to the query
func (b *TimeseriesQueryBuilder) WithTimeBucket(interval, column, alias string) *TimeseriesQueryBuilder {
	b.timeBucket = &TimeBucket{
		Interval: interval,
		Column:   column,
		Alias:    alias,
	}
	return b
}

// Select adds columns to the SELECT clause
func (b *TimeseriesQueryBuilder) Select(cols ...string) *TimeseriesQueryBuilder {
	b.selectCols = append(b.selectCols, cols...)
	return b
}

// Aggregate adds an aggregation function to a column
func (b *TimeseriesQueryBuilder) Aggregate(function AggregateFunction, column, alias string) *TimeseriesQueryBuilder {
	b.aggregations = append(b.aggregations, ColumnAggregation{
		Function: function,
		Column:   column,
		Alias:    alias,
	})
	return b
}

// WhereTimeRange adds a time range condition
func (b *TimeseriesQueryBuilder) WhereTimeRange(column string, start, end time.Time) *TimeseriesQueryBuilder {
	clause := fmt.Sprintf("%s BETWEEN $%d AND $%d", column, len(b.whereArgs)+1, len(b.whereArgs)+2)
	b.whereClauses = append(b.whereClauses, clause)
	b.whereArgs = append(b.whereArgs, start, end)
	return b
}

// Where adds a WHERE condition
func (b *TimeseriesQueryBuilder) Where(clause string, args ...interface{}) *TimeseriesQueryBuilder {
	// Adjust the parameter indices in the clause
	paramCount := len(b.whereArgs)
	for i := 1; i <= len(args); i++ {
		oldParam := fmt.Sprintf("$%d", i)
		newParam := fmt.Sprintf("$%d", i+paramCount)
		clause = strings.Replace(clause, oldParam, newParam, -1)
	}

	b.whereClauses = append(b.whereClauses, clause)
	b.whereArgs = append(b.whereArgs, args...)
	return b
}

// GroupBy adds columns to the GROUP BY clause
func (b *TimeseriesQueryBuilder) GroupBy(cols ...string) *TimeseriesQueryBuilder {
	b.groupByCols = append(b.groupByCols, cols...)
	return b
}

// OrderBy adds columns to the ORDER BY clause
func (b *TimeseriesQueryBuilder) OrderBy(cols ...string) *TimeseriesQueryBuilder {
	b.orderByCols = append(b.orderByCols, cols...)
	return b
}

// Limit sets the LIMIT clause
func (b *TimeseriesQueryBuilder) Limit(limit int) *TimeseriesQueryBuilder {
	b.limit = limit
	return b
}

// Offset sets the OFFSET clause
func (b *TimeseriesQueryBuilder) Offset(offset int) *TimeseriesQueryBuilder {
	b.offset = offset
	return b
}

// Build constructs the SQL query and args
func (b *TimeseriesQueryBuilder) Build() (string, []interface{}) {
	var selectClause strings.Builder
	selectClause.WriteString("SELECT ")

	var selects []string

	// Add time bucket if specified
	if b.timeBucket != nil {
		alias := b.timeBucket.Alias
		if alias == "" {
			alias = "time_bucket"
		}

		bucketStr := fmt.Sprintf(
			"time_bucket('%s', %s) AS %s",
			b.timeBucket.Interval,
			b.timeBucket.Column,
			alias,
		)
		selects = append(selects, bucketStr)

		// Add time bucket to group by if not already included
		bucketFound := false
		for _, col := range b.groupByCols {
			if col == alias {
				bucketFound = true
				break
			}
		}

		if !bucketFound {
			b.groupByCols = append([]string{alias}, b.groupByCols...)
		}
	}

	// Add selected columns
	selects = append(selects, b.selectCols...)

	// Add aggregations
	for _, agg := range b.aggregations {
		alias := agg.Alias
		if alias == "" {
			alias = strings.ToLower(string(agg.Function)) + "_" + agg.Column
		}

		aggStr := fmt.Sprintf(
			"%s(%s) AS %s",
			agg.Function,
			agg.Column,
			alias,
		)
		selects = append(selects, aggStr)
	}

	// If no columns or aggregations selected, use *
	if len(selects) == 0 {
		selectClause.WriteString("*")
	} else {
		selectClause.WriteString(strings.Join(selects, ", "))
	}

	// Build query
	query := fmt.Sprintf("%s FROM %s", selectClause.String(), b.table)

	// Add WHERE clause
	if len(b.whereClauses) > 0 {
		query += " WHERE " + strings.Join(b.whereClauses, " AND ")
	}

	// Add GROUP BY clause
	if len(b.groupByCols) > 0 {
		query += " GROUP BY " + strings.Join(b.groupByCols, ", ")
	}

	// Add ORDER BY clause
	if len(b.orderByCols) > 0 {
		query += " ORDER BY " + strings.Join(b.orderByCols, ", ")
	}

	// Add LIMIT clause
	if b.limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", b.limit)
	}

	// Add OFFSET clause
	if b.offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", b.offset)
	}

	return query, b.whereArgs
}

// Execute runs the query against the database
func (b *TimeseriesQueryBuilder) Execute(ctx context.Context, db *DB) ([]map[string]interface{}, error) {
	query, args := b.Build()
	result, err := db.ExecuteSQL(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute time-series query: %w", err)
	}

	rows, ok := result.([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type from database query")
	}

	return rows, nil
}

// DownsampleOptions describes options for downsampling time-series data
type DownsampleOptions struct {
	SourceTable       string
	DestTable         string
	TimeColumn        string
	BucketInterval    string
	Aggregations      []ColumnAggregation
	WhereCondition    string
	CreateTable       bool
	ChunkTimeInterval string
}

// DownsampleTimeSeries creates downsampled time-series data
func (t *DB) DownsampleTimeSeries(ctx context.Context, options DownsampleOptions) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	// Create the destination table if requested
	if options.CreateTable {
		// Get source table columns
		schemaQuery := fmt.Sprintf("SELECT column_name, data_type FROM information_schema.columns WHERE table_name = '%s'", options.SourceTable)
		result, err := t.ExecuteSQLWithoutParams(ctx, schemaQuery)
		if err != nil {
			return fmt.Errorf("failed to get source table schema: %w", err)
		}

		columns, ok := result.([]map[string]interface{})
		if !ok {
			return fmt.Errorf("unexpected result from schema query")
		}

		// Build CREATE TABLE statement
		var createStmt strings.Builder
		createStmt.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (", options.DestTable))

		// Add time bucket column
		createStmt.WriteString("time_bucket timestamptz, ")

		// Add aggregation columns
		for i, agg := range options.Aggregations {
			colName := agg.Alias
			if colName == "" {
				colName = strings.ToLower(string(agg.Function)) + "_" + agg.Column
			}

			// Find the data type of the source column
			var dataType string
			for _, col := range columns {
				if fmt.Sprintf("%v", col["column_name"]) == agg.Column {
					dataType = fmt.Sprintf("%v", col["data_type"])
					break
				}
			}

			if dataType == "" {
				dataType = "double precision" // Default for numeric aggregations
			}

			createStmt.WriteString(fmt.Sprintf("%s %s", colName, dataType))

			if i < len(options.Aggregations)-1 {
				createStmt.WriteString(", ")
			}
		}

		createStmt.WriteString(", PRIMARY KEY (time_bucket)")
		createStmt.WriteString(")")

		// Create the table
		_, err = t.ExecuteSQLWithoutParams(ctx, createStmt.String())
		if err != nil {
			return fmt.Errorf("failed to create destination table: %w", err)
		}

		// Make it a hypertable
		if options.ChunkTimeInterval == "" {
			options.ChunkTimeInterval = options.BucketInterval
		}

		err = t.CreateHypertable(ctx, HypertableConfig{
			TableName:         options.DestTable,
			TimeColumn:        "time_bucket",
			ChunkTimeInterval: options.ChunkTimeInterval,
			IfNotExists:       true,
		})
		if err != nil {
			return fmt.Errorf("failed to create hypertable: %w", err)
		}
	}

	// Build the INSERT statement with aggregations
	var insertStmt strings.Builder
	insertStmt.WriteString(fmt.Sprintf("INSERT INTO %s (time_bucket, ", options.DestTable))

	// Add aggregation column names
	for i, agg := range options.Aggregations {
		colName := agg.Alias
		if colName == "" {
			colName = strings.ToLower(string(agg.Function)) + "_" + agg.Column
		}

		insertStmt.WriteString(colName)

		if i < len(options.Aggregations)-1 {
			insertStmt.WriteString(", ")
		}
	}

	insertStmt.WriteString(") SELECT time_bucket('")
	insertStmt.WriteString(options.BucketInterval)
	insertStmt.WriteString("', ")
	insertStmt.WriteString(options.TimeColumn)
	insertStmt.WriteString(") AS time_bucket, ")

	// Add aggregation functions
	for i, agg := range options.Aggregations {
		insertStmt.WriteString(fmt.Sprintf("%s(%s)", agg.Function, agg.Column))

		if i < len(options.Aggregations)-1 {
			insertStmt.WriteString(", ")
		}
	}

	insertStmt.WriteString(fmt.Sprintf(" FROM %s", options.SourceTable))

	// Add WHERE clause if specified
	if options.WhereCondition != "" {
		insertStmt.WriteString(" WHERE ")
		insertStmt.WriteString(options.WhereCondition)
	}

	// Group by time bucket
	insertStmt.WriteString(" GROUP BY time_bucket")

	// Order by time bucket
	insertStmt.WriteString(" ORDER BY time_bucket")

	// Execute the INSERT statement
	_, err := t.ExecuteSQLWithoutParams(ctx, insertStmt.String())
	if err != nil {
		return fmt.Errorf("failed to downsample data: %w", err)
	}

	return nil
}

// TimeRange represents a common time range for queries
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// PredefinedTimeRange returns a TimeRange for common time ranges
func PredefinedTimeRange(name string) (*TimeRange, error) {
	now := time.Now()

	switch strings.ToLower(name) {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return &TimeRange{Start: start, End: now}, nil

	case "yesterday":
		end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		start := end.Add(-24 * time.Hour)
		return &TimeRange{Start: start, End: end}, nil

	case "last24hours", "last_24_hours":
		start := now.Add(-24 * time.Hour)
		return &TimeRange{Start: start, End: now}, nil

	case "thisweek", "this_week":
		// Calculate the beginning of the week (Sunday/Monday depending on locale, using Sunday here)
		weekday := int(now.Weekday())
		start := now.Add(-time.Duration(weekday) * 24 * time.Hour)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, now.Location())
		return &TimeRange{Start: start, End: now}, nil

	case "lastweek", "last_week":
		// Calculate the beginning of this week
		weekday := int(now.Weekday())
		thisWeekStart := now.Add(-time.Duration(weekday) * 24 * time.Hour)
		thisWeekStart = time.Date(thisWeekStart.Year(), thisWeekStart.Month(), thisWeekStart.Day(), 0, 0, 0, 0, now.Location())

		// Last week is 7 days before the beginning of this week
		lastWeekStart := thisWeekStart.Add(-7 * 24 * time.Hour)
		lastWeekEnd := thisWeekStart

		return &TimeRange{Start: lastWeekStart, End: lastWeekEnd}, nil

	case "last7days", "last_7_days":
		start := now.Add(-7 * 24 * time.Hour)
		return &TimeRange{Start: start, End: now}, nil

	case "thismonth", "this_month":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return &TimeRange{Start: start, End: now}, nil

	case "lastmonth", "last_month":
		thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

		var lastMonthStart time.Time
		if now.Month() == 1 {
			// January, so last month is December of previous year
			lastMonthStart = time.Date(now.Year()-1, 12, 1, 0, 0, 0, 0, now.Location())
		} else {
			// Any other month
			lastMonthStart = time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
		}

		return &TimeRange{Start: lastMonthStart, End: thisMonthStart}, nil

	case "last30days", "last_30_days":
		start := now.Add(-30 * 24 * time.Hour)
		return &TimeRange{Start: start, End: now}, nil

	case "thisyear", "this_year":
		start := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		return &TimeRange{Start: start, End: now}, nil

	case "lastyear", "last_year":
		thisYearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		lastYearStart := time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location())

		return &TimeRange{Start: lastYearStart, End: thisYearStart}, nil

	case "last365days", "last_365_days":
		start := now.Add(-365 * 24 * time.Hour)
		return &TimeRange{Start: start, End: now}, nil

	default:
		return nil, fmt.Errorf("unknown time range: %s", name)
	}
}
