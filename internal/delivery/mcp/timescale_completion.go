package mcp

import (
	"context"
	"fmt"
)

// CompletionItem represents a code completion item
type CompletionItem struct {
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	Documentation    string   `json:"documentation"`
	InsertText       string   `json:"insertText"`
	Parameters       []string `json:"parameters,omitempty"`
	ReturnType       string   `json:"returnType,omitempty"`
	Category         string   `json:"category,omitempty"`
	SortText         string   `json:"sortText,omitempty"`
	FilterText       string   `json:"filterText,omitempty"`
	CommitCharacters []string `json:"commitCharacters,omitempty"`
}

// QuerySuggestion represents a suggested query template for TimescaleDB
type QuerySuggestion struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Query       string `json:"query"`
	Category    string `json:"category"`
}

// TimescaleDBCompletionProvider provides code completion for TimescaleDB functions
type TimescaleDBCompletionProvider struct {
	contextProvider *TimescaleDBContextProvider
}

// NewTimescaleDBCompletionProvider creates a new TimescaleDB completion provider
func NewTimescaleDBCompletionProvider() *TimescaleDBCompletionProvider {
	return &TimescaleDBCompletionProvider{
		contextProvider: NewTimescaleDBContextProvider(),
	}
}

// GetTimeBucketCompletions returns completions for time_bucket functions
func (p *TimescaleDBCompletionProvider) GetTimeBucketCompletions(ctx context.Context, dbID string, useCase UseCaseProvider) ([]CompletionItem, error) {
	// First check if TimescaleDB is available
	tsdbContext, err := p.contextProvider.DetectTimescaleDB(ctx, dbID, useCase)
	if err != nil {
		return nil, fmt.Errorf("failed to detect TimescaleDB: %w", err)
	}

	if !tsdbContext.IsTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB is not available in the database %s", dbID)
	}

	// Define time bucket function completions
	completions := []CompletionItem{
		{
			Name:             "time_bucket",
			Type:             "function",
			Documentation:    "TimescaleDB function that groups time into buckets. Useful for downsampling time-series data.",
			InsertText:       "time_bucket($1, $2)",
			Parameters:       []string{"interval", "timestamp"},
			ReturnType:       "timestamp",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "time_bucket_gapfill",
			Type:             "function",
			Documentation:    "TimescaleDB function similar to time_bucket but fills in missing values (gaps) in the result.",
			InsertText:       "time_bucket_gapfill($1, $2)",
			Parameters:       []string{"interval", "timestamp"},
			ReturnType:       "timestamp",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "time_bucket_ng",
			Type:             "function",
			Documentation:    "TimescaleDB next-generation time bucket function that supports timezone-aware bucketing.",
			InsertText:       "time_bucket_ng('$1', $2)",
			Parameters:       []string{"interval", "timestamp", "timezone"},
			ReturnType:       "timestamp with time zone",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "time_bucket",
			Type:             "function",
			Documentation:    "TimescaleDB function that groups time into buckets with an offset.",
			InsertText:       "time_bucket($1, $2, $3)",
			Parameters:       []string{"interval", "timestamp", "offset"},
			ReturnType:       "timestamp",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
	}

	// Add version information to documentation
	for i := range completions {
		completions[i].Documentation = fmt.Sprintf("TimescaleDB v%s: %s", tsdbContext.Version, completions[i].Documentation)
	}

	return completions, nil
}

// GetHypertableFunctionCompletions returns completions for hypertable management functions
func (p *TimescaleDBCompletionProvider) GetHypertableFunctionCompletions(ctx context.Context, dbID string, useCase UseCaseProvider) ([]CompletionItem, error) {
	// First check if TimescaleDB is available
	tsdbContext, err := p.contextProvider.DetectTimescaleDB(ctx, dbID, useCase)
	if err != nil {
		return nil, fmt.Errorf("failed to detect TimescaleDB: %w", err)
	}

	if !tsdbContext.IsTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB is not available in the database %s", dbID)
	}

	// Define hypertable function completions
	completions := []CompletionItem{
		{
			Name:             "create_hypertable",
			Type:             "function",
			Documentation:    "TimescaleDB function that converts a standard PostgreSQL table into a hypertable partitioned by time.",
			InsertText:       "create_hypertable('$1', '$2')",
			Parameters:       []string{"table_name", "time_column_name"},
			ReturnType:       "void",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "add_dimension",
			Type:             "function",
			Documentation:    "TimescaleDB function that adds another dimension to a hypertable for partitioning.",
			InsertText:       "add_dimension('$1', '$2')",
			Parameters:       []string{"hypertable", "column_name"},
			ReturnType:       "void",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "add_compression_policy",
			Type:             "function",
			Documentation:    "TimescaleDB function that adds an automatic compression policy to a hypertable.",
			InsertText:       "add_compression_policy('$1', INTERVAL '$2')",
			Parameters:       []string{"hypertable", "older_than"},
			ReturnType:       "integer",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "add_retention_policy",
			Type:             "function",
			Documentation:    "TimescaleDB function that adds an automatic data retention policy to a hypertable.",
			InsertText:       "add_retention_policy('$1', INTERVAL '$2')",
			Parameters:       []string{"hypertable", "drop_after"},
			ReturnType:       "integer",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "alter_job",
			Type:             "function",
			Documentation:    "TimescaleDB function that alters a policy job's schedule or configuration.",
			InsertText:       "alter_job($1, scheduled => true)",
			Parameters:       []string{"job_id"},
			ReturnType:       "integer",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "hypertable_size",
			Type:             "function",
			Documentation:    "TimescaleDB function that shows the size of a hypertable, including all chunks.",
			InsertText:       "hypertable_size('$1')",
			Parameters:       []string{"hypertable"},
			ReturnType:       "bigint",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "hypertable_detailed_size",
			Type:             "function",
			Documentation:    "TimescaleDB function that shows detailed size information for a hypertable.",
			InsertText:       "hypertable_detailed_size('$1')",
			Parameters:       []string{"hypertable"},
			ReturnType:       "table",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
	}

	// Add version information to documentation
	for i := range completions {
		completions[i].Documentation = fmt.Sprintf("TimescaleDB v%s: %s", tsdbContext.Version, completions[i].Documentation)
	}

	return completions, nil
}

// GetContinuousAggregateFunctionCompletions returns completions for continuous aggregate functions
func (p *TimescaleDBCompletionProvider) GetContinuousAggregateFunctionCompletions(ctx context.Context, dbID string, useCase UseCaseProvider) ([]CompletionItem, error) {
	// First check if TimescaleDB is available
	tsdbContext, err := p.contextProvider.DetectTimescaleDB(ctx, dbID, useCase)
	if err != nil {
		return nil, fmt.Errorf("failed to detect TimescaleDB: %w", err)
	}

	if !tsdbContext.IsTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB is not available in the database %s", dbID)
	}

	// Define continuous aggregate function completions
	completions := []CompletionItem{
		{
			Name:             "create_materialized_view",
			Type:             "function",
			Documentation:    "TimescaleDB function that creates a continuous aggregate view.",
			InsertText:       "CREATE MATERIALIZED VIEW $1 WITH (timescaledb.continuous) AS SELECT $2 FROM $3 GROUP BY $4;",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "add_continuous_aggregate_policy",
			Type:             "function",
			Documentation:    "TimescaleDB function that adds a refresh policy to a continuous aggregate.",
			InsertText:       "add_continuous_aggregate_policy('$1', start_offset => INTERVAL '$2', end_offset => INTERVAL '$3', schedule_interval => INTERVAL '$4')",
			Parameters:       []string{"continuous_aggregate", "start_offset", "end_offset", "schedule_interval"},
			ReturnType:       "integer",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "refresh_continuous_aggregate",
			Type:             "function",
			Documentation:    "TimescaleDB function that manually refreshes a continuous aggregate for a specific time range.",
			InsertText:       "refresh_continuous_aggregate('$1', '$2', '$3')",
			Parameters:       []string{"continuous_aggregate", "start_time", "end_time"},
			ReturnType:       "void",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
	}

	// Add version information to documentation
	for i := range completions {
		completions[i].Documentation = fmt.Sprintf("TimescaleDB v%s: %s", tsdbContext.Version, completions[i].Documentation)
	}

	return completions, nil
}

// GetAnalyticsFunctionCompletions returns completions for TimescaleDB's analytics functions
func (p *TimescaleDBCompletionProvider) GetAnalyticsFunctionCompletions(ctx context.Context, dbID string, useCase UseCaseProvider) ([]CompletionItem, error) {
	// First check if TimescaleDB is available
	tsdbContext, err := p.contextProvider.DetectTimescaleDB(ctx, dbID, useCase)
	if err != nil {
		return nil, fmt.Errorf("failed to detect TimescaleDB: %w", err)
	}

	if !tsdbContext.IsTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB is not available in the database %s", dbID)
	}

	// Define analytics function completions
	completions := []CompletionItem{
		{
			Name:             "first",
			Type:             "function",
			Documentation:    "TimescaleDB function that returns the value of the specified column at the first time ordered by time within each group.",
			InsertText:       "first($1, $2)",
			Parameters:       []string{"value", "time"},
			ReturnType:       "same as value",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "last",
			Type:             "function",
			Documentation:    "TimescaleDB function that returns the value of the specified column at the last time ordered by time within each group.",
			InsertText:       "last($1, $2)",
			Parameters:       []string{"value", "time"},
			ReturnType:       "same as value",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "time_weight",
			Type:             "function",
			Documentation:    "TimescaleDB function that returns the time-weighted average of a value over time.",
			InsertText:       "time_weight($1, $2)",
			Parameters:       []string{"value", "time"},
			ReturnType:       "double precision",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "histogram",
			Type:             "function",
			Documentation:    "TimescaleDB function that buckets values and returns a histogram showing the distribution.",
			InsertText:       "histogram($1, $2, $3, $4)",
			Parameters:       []string{"value", "min", "max", "num_buckets"},
			ReturnType:       "histogram",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "approx_percentile",
			Type:             "function",
			Documentation:    "TimescaleDB function that calculates approximate percentiles using the t-digest method.",
			InsertText:       "approx_percentile($1, $2)",
			Parameters:       []string{"value", "percentile"},
			ReturnType:       "double precision",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
	}

	// Add version information to documentation
	for i := range completions {
		completions[i].Documentation = fmt.Sprintf("TimescaleDB v%s: %s", tsdbContext.Version, completions[i].Documentation)
	}

	return completions, nil
}

// GetAllFunctionCompletions returns completions for all TimescaleDB functions
func (p *TimescaleDBCompletionProvider) GetAllFunctionCompletions(ctx context.Context, dbID string, useCase UseCaseProvider) ([]CompletionItem, error) {
	// Check if TimescaleDB is available by using the DetectTimescaleDB method
	// which already checks the database type
	tsdbContext, err := p.contextProvider.DetectTimescaleDB(ctx, dbID, useCase)
	if err != nil {
		return nil, fmt.Errorf("failed to detect TimescaleDB: %w", err)
	}

	if !tsdbContext.IsTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB is not available in the database %s", dbID)
	}

	// Define predefined completions for all categories
	var allCompletions []CompletionItem

	// Time bucket functions
	allCompletions = append(allCompletions, []CompletionItem{
		{
			Name:             "time_bucket",
			Type:             "function",
			Documentation:    "TimescaleDB function that groups time into buckets. Useful for downsampling time-series data.",
			InsertText:       "time_bucket($1, $2)",
			Parameters:       []string{"interval", "timestamp"},
			ReturnType:       "timestamp",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "time_bucket_gapfill",
			Type:             "function",
			Documentation:    "TimescaleDB function similar to time_bucket but fills in missing values (gaps) in the result.",
			InsertText:       "time_bucket_gapfill($1, $2)",
			Parameters:       []string{"interval", "timestamp"},
			ReturnType:       "timestamp",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "time_bucket_ng",
			Type:             "function",
			Documentation:    "TimescaleDB next-generation time bucket function that supports timezone-aware bucketing.",
			InsertText:       "time_bucket_ng('$1', $2)",
			Parameters:       []string{"interval", "timestamp", "timezone"},
			ReturnType:       "timestamp with time zone",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
	}...)

	// Hypertable functions
	allCompletions = append(allCompletions, []CompletionItem{
		{
			Name:             "create_hypertable",
			Type:             "function",
			Documentation:    "TimescaleDB function that converts a standard PostgreSQL table into a hypertable partitioned by time.",
			InsertText:       "create_hypertable('$1', '$2')",
			Parameters:       []string{"table_name", "time_column_name"},
			ReturnType:       "void",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "add_dimension",
			Type:             "function",
			Documentation:    "TimescaleDB function that adds another dimension to a hypertable for partitioning.",
			InsertText:       "add_dimension('$1', '$2')",
			Parameters:       []string{"hypertable", "column_name"},
			ReturnType:       "void",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "add_compression_policy",
			Type:             "function",
			Documentation:    "TimescaleDB function that adds an automatic compression policy to a hypertable.",
			InsertText:       "add_compression_policy('$1', INTERVAL '$2')",
			Parameters:       []string{"hypertable", "older_than"},
			ReturnType:       "integer",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "add_retention_policy",
			Type:             "function",
			Documentation:    "TimescaleDB function that adds an automatic data retention policy to a hypertable.",
			InsertText:       "add_retention_policy('$1', INTERVAL '$2')",
			Parameters:       []string{"hypertable", "drop_after"},
			ReturnType:       "integer",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
	}...)

	// Continuous aggregate functions
	allCompletions = append(allCompletions, []CompletionItem{
		{
			Name:             "create_materialized_view",
			Type:             "function",
			Documentation:    "TimescaleDB function that creates a continuous aggregate view.",
			InsertText:       "CREATE MATERIALIZED VIEW $1 WITH (timescaledb.continuous) AS SELECT $2 FROM $3 GROUP BY $4;",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "add_continuous_aggregate_policy",
			Type:             "function",
			Documentation:    "TimescaleDB function that adds a refresh policy to a continuous aggregate.",
			InsertText:       "add_continuous_aggregate_policy('$1', start_offset => INTERVAL '$2', end_offset => INTERVAL '$3', schedule_interval => INTERVAL '$4')",
			Parameters:       []string{"continuous_aggregate", "start_offset", "end_offset", "schedule_interval"},
			ReturnType:       "integer",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
	}...)

	// Analytics functions
	allCompletions = append(allCompletions, []CompletionItem{
		{
			Name:             "first",
			Type:             "function",
			Documentation:    "TimescaleDB function that returns the value of the specified column at the first time ordered by time within each group.",
			InsertText:       "first($1, $2)",
			Parameters:       []string{"value", "time"},
			ReturnType:       "same as value",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "last",
			Type:             "function",
			Documentation:    "TimescaleDB function that returns the value of the specified column at the last time ordered by time within each group.",
			InsertText:       "last($1, $2)",
			Parameters:       []string{"value", "time"},
			ReturnType:       "same as value",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
		{
			Name:             "time_weight",
			Type:             "function",
			Documentation:    "TimescaleDB function that returns the time-weighted average of a value over time.",
			InsertText:       "time_weight($1, $2)",
			Parameters:       []string{"value", "time"},
			ReturnType:       "double precision",
			Category:         "TimescaleDB",
			CommitCharacters: []string{"("},
		},
	}...)

	// Add version information to documentation
	for i := range allCompletions {
		allCompletions[i].Documentation = fmt.Sprintf("TimescaleDB v%s: %s", tsdbContext.Version, allCompletions[i].Documentation)
	}

	return allCompletions, nil
}

// GetQuerySuggestions returns TimescaleDB query suggestions based on the database schema
func (p *TimescaleDBCompletionProvider) GetQuerySuggestions(ctx context.Context, dbID string, useCase UseCaseProvider) ([]QuerySuggestion, error) {
	// First check if TimescaleDB is available
	tsdbContext, err := p.contextProvider.GetTimescaleDBContext(ctx, dbID, useCase)
	if err != nil {
		return nil, fmt.Errorf("failed to get TimescaleDB context: %w", err)
	}

	if !tsdbContext.IsTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB is not available in the database %s", dbID)
	}

	// Base suggestions that don't depend on schema
	suggestions := []QuerySuggestion{
		{
			Title:       "Basic Time Bucket Aggregation",
			Description: "Groups time-series data into time buckets and calculates aggregates",
			Query:       "SELECT time_bucket('1 hour', time_column) AS bucket, avg(value_column), min(value_column), max(value_column) FROM table_name WHERE time_column > now() - INTERVAL '1 day' GROUP BY bucket ORDER BY bucket;",
			Category:    "Time Buckets",
		},
		{
			Title:       "Time Bucket with Gap Filling",
			Description: "Groups time-series data with gap filling for missing values",
			Query:       "SELECT time_bucket_gapfill('1 hour', time_column) AS bucket, avg(value_column), min(value_column), max(value_column) FROM table_name WHERE time_column > now() - INTERVAL '1 day' AND time_column <= now() GROUP BY bucket ORDER BY bucket;",
			Category:    "Time Buckets",
		},
		{
			Title:       "Create Hypertable",
			Description: "Converts a standard PostgreSQL table into a TimescaleDB hypertable",
			Query:       "SELECT create_hypertable('table_name', 'time_column');",
			Category:    "Hypertable Management",
		},
		{
			Title:       "Add Compression Policy",
			Description: "Adds an automatic compression policy to a hypertable",
			Query:       "SELECT add_compression_policy('table_name', INTERVAL '7 days');",
			Category:    "Hypertable Management",
		},
		{
			Title:       "Add Retention Policy",
			Description: "Adds an automatic data retention policy to a hypertable",
			Query:       "SELECT add_retention_policy('table_name', INTERVAL '30 days');",
			Category:    "Hypertable Management",
		},
		{
			Title:       "Create Continuous Aggregate",
			Description: "Creates a materialized view that automatically maintains aggregated data",
			Query:       "CREATE MATERIALIZED VIEW view_name WITH (timescaledb.continuous) AS SELECT time_bucket('1 hour', time_column) as bucket, avg(value_column) FROM table_name GROUP BY bucket;",
			Category:    "Continuous Aggregates",
		},
		{
			Title:       "Add Continuous Aggregate Policy",
			Description: "Adds a refresh policy to a continuous aggregate",
			Query:       "SELECT add_continuous_aggregate_policy('view_name', start_offset => INTERVAL '2 days', end_offset => INTERVAL '1 hour', schedule_interval => INTERVAL '1 hour');",
			Category:    "Continuous Aggregates",
		},
		{
			Title:       "Hypertable Size",
			Description: "Shows the size of a hypertable including all chunks",
			Query:       "SELECT * FROM hypertable_size('table_name');",
			Category:    "Diagnostics",
		},
		{
			Title:       "Hypertable Detailed Size",
			Description: "Shows detailed size information for a hypertable",
			Query:       "SELECT * FROM hypertable_detailed_size('table_name');",
			Category:    "Diagnostics",
		},
		{
			Title:       "Compression Stats",
			Description: "Shows compression statistics for a hypertable",
			Query:       "SELECT * FROM hypertable_compression_stats('table_name');",
			Category:    "Diagnostics",
		},
		{
			Title:       "Job Stats",
			Description: "Shows statistics for background jobs like compression and retention policies",
			Query:       "SELECT * FROM timescaledb_information.jobs;",
			Category:    "Diagnostics",
		},
	}

	// If we have hypertable information, use it to create tailored suggestions
	if len(tsdbContext.Hypertables) > 0 {
		for _, ht := range tsdbContext.Hypertables {
			tableName := ht.TableName
			timeColumn := ht.TimeColumn

			// Skip if we don't have both table name and time column
			if tableName == "" || timeColumn == "" {
				continue
			}

			// Add schema-specific suggestions
			suggestions = append(suggestions, []QuerySuggestion{
				{
					Title:       fmt.Sprintf("Time Bucket Aggregation for %s", tableName),
					Description: fmt.Sprintf("Groups data from %s table into time buckets", tableName),
					Query:       fmt.Sprintf("SELECT time_bucket('1 hour', %s) AS bucket, avg(value_column) FROM %s WHERE %s > now() - INTERVAL '1 day' GROUP BY bucket ORDER BY bucket;", timeColumn, tableName, timeColumn),
					Category:    "Time Buckets",
				},
				{
					Title:       fmt.Sprintf("Compression Policy for %s", tableName),
					Description: fmt.Sprintf("Adds compression policy to %s hypertable", tableName),
					Query:       fmt.Sprintf("SELECT add_compression_policy('%s', INTERVAL '7 days');", tableName),
					Category:    "Hypertable Management",
				},
				{
					Title:       fmt.Sprintf("Retention Policy for %s", tableName),
					Description: fmt.Sprintf("Adds retention policy to %s hypertable", tableName),
					Query:       fmt.Sprintf("SELECT add_retention_policy('%s', INTERVAL '30 days');", tableName),
					Category:    "Hypertable Management",
				},
				{
					Title:       fmt.Sprintf("Continuous Aggregate for %s", tableName),
					Description: fmt.Sprintf("Creates a continuous aggregate view for %s", tableName),
					Query:       fmt.Sprintf("CREATE MATERIALIZED VIEW %s_hourly WITH (timescaledb.continuous) AS SELECT time_bucket('1 hour', %s) as bucket, avg(value_column) FROM %s GROUP BY bucket;", tableName, timeColumn, tableName),
					Category:    "Continuous Aggregates",
				},
				{
					Title:       fmt.Sprintf("Recent Data from %s", tableName),
					Description: fmt.Sprintf("Retrieves recent data from %s with time ordering", tableName),
					Query:       fmt.Sprintf("SELECT * FROM %s WHERE %s > now() - INTERVAL '1 day' ORDER BY %s DESC LIMIT 100;", tableName, timeColumn, timeColumn),
					Category:    "Data Retrieval",
				},
				{
					Title:       fmt.Sprintf("First/Last Analysis for %s", tableName),
					Description: fmt.Sprintf("Uses first/last functions to analyze %s by segments", tableName),
					Query:       fmt.Sprintf("SELECT segment_column, first(value_column, %s), last(value_column, %s) FROM %s GROUP BY segment_column;", timeColumn, timeColumn, tableName),
					Category:    "Analytics",
				},
			}...)
		}
	}

	return suggestions, nil
}
