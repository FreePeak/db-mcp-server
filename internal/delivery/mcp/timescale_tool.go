package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/cortex/pkg/server"
	cortextools "github.com/FreePeak/cortex/pkg/tools"
)

// TimescaleDBTool implements a tool for TimescaleDB operations
type TimescaleDBTool struct {
	name        string
	description string
}

// NewTimescaleDBTool creates a new TimescaleDB tool
func NewTimescaleDBTool() *TimescaleDBTool {
	return &TimescaleDBTool{
		name:        "timescaledb",
		description: "Perform TimescaleDB operations",
	}
}

// GetName returns the name of the tool
func (t *TimescaleDBTool) GetName() string {
	return t.name
}

// GetDescription returns the description of the tool
func (t *TimescaleDBTool) GetDescription(dbID string) string {
	if dbID == "" {
		return t.description
	}
	return fmt.Sprintf("%s on %s", t.description, dbID)
}

// CreateTool creates a tool instance
func (t *TimescaleDBTool) CreateTool(name string, dbID string) interface{} {
	return cortextools.NewTool(
		name,
		cortextools.WithDescription(t.GetDescription(dbID)),
		cortextools.WithString("operation",
			cortextools.Description("TimescaleDB operation to perform"),
			cortextools.Required(),
		),
		cortextools.WithString("target_table",
			cortextools.Description("The table to perform the operation on"),
			cortextools.Required(),
		),
	)
}

// CreateHypertableTool creates a specific tool for hypertable creation
func (t *TimescaleDBTool) CreateHypertableTool(name string, dbID string) interface{} {
	return cortextools.NewTool(
		name,
		cortextools.WithDescription(fmt.Sprintf("Create TimescaleDB hypertable on %s", dbID)),
		cortextools.WithString("operation",
			cortextools.Description("Must be 'create_hypertable'"),
			cortextools.Required(),
		),
		cortextools.WithString("target_table",
			cortextools.Description("The table to convert to a hypertable"),
			cortextools.Required(),
		),
		cortextools.WithString("time_column",
			cortextools.Description("The timestamp column for the hypertable"),
			cortextools.Required(),
		),
		cortextools.WithString("chunk_time_interval",
			cortextools.Description("Time interval for chunks (e.g., '1 day')"),
		),
		cortextools.WithString("partitioning_column",
			cortextools.Description("Optional column for space partitioning"),
		),
		cortextools.WithBoolean("if_not_exists",
			cortextools.Description("Skip if hypertable already exists"),
		),
	)
}

// CreateListHypertablesTool creates a specific tool for listing hypertables
func (t *TimescaleDBTool) CreateListHypertablesTool(name string, dbID string) interface{} {
	return cortextools.NewTool(
		name,
		cortextools.WithDescription(fmt.Sprintf("List TimescaleDB hypertables on %s", dbID)),
		cortextools.WithString("operation",
			cortextools.Description("Must be 'list_hypertables'"),
			cortextools.Required(),
		),
	)
}

// CreateCompressionEnableTool creates a tool for enabling compression on a hypertable
func (t *TimescaleDBTool) CreateCompressionEnableTool(name string, dbID string) interface{} {
	return cortextools.NewTool(
		name,
		cortextools.WithDescription(fmt.Sprintf("Enable compression on TimescaleDB hypertable on %s", dbID)),
		cortextools.WithString("operation",
			cortextools.Description("Must be 'enable_compression'"),
			cortextools.Required(),
		),
		cortextools.WithString("target_table",
			cortextools.Description("The hypertable to enable compression on"),
			cortextools.Required(),
		),
		cortextools.WithString("after",
			cortextools.Description("Time interval after which to compress chunks (e.g., '7 days')"),
		),
	)
}

// CreateCompressionDisableTool creates a tool for disabling compression on a hypertable
func (t *TimescaleDBTool) CreateCompressionDisableTool(name string, dbID string) interface{} {
	return cortextools.NewTool(
		name,
		cortextools.WithDescription(fmt.Sprintf("Disable compression on TimescaleDB hypertable on %s", dbID)),
		cortextools.WithString("operation",
			cortextools.Description("Must be 'disable_compression'"),
			cortextools.Required(),
		),
		cortextools.WithString("target_table",
			cortextools.Description("The hypertable to disable compression on"),
			cortextools.Required(),
		),
	)
}

// CreateCompressionPolicyAddTool creates a tool for adding a compression policy
func (t *TimescaleDBTool) CreateCompressionPolicyAddTool(name string, dbID string) interface{} {
	return cortextools.NewTool(
		name,
		cortextools.WithDescription(fmt.Sprintf("Add compression policy to TimescaleDB hypertable on %s", dbID)),
		cortextools.WithString("operation",
			cortextools.Description("Must be 'add_compression_policy'"),
			cortextools.Required(),
		),
		cortextools.WithString("target_table",
			cortextools.Description("The hypertable to add compression policy to"),
			cortextools.Required(),
		),
		cortextools.WithString("interval",
			cortextools.Description("Time interval after which to compress chunks (e.g., '30 days')"),
			cortextools.Required(),
		),
		cortextools.WithString("segment_by",
			cortextools.Description("Column to use for segmenting data during compression"),
		),
		cortextools.WithString("order_by",
			cortextools.Description("Column(s) to use for ordering data during compression"),
		),
	)
}

// CreateCompressionPolicyRemoveTool creates a tool for removing a compression policy
func (t *TimescaleDBTool) CreateCompressionPolicyRemoveTool(name string, dbID string) interface{} {
	return cortextools.NewTool(
		name,
		cortextools.WithDescription(fmt.Sprintf("Remove compression policy from TimescaleDB hypertable on %s", dbID)),
		cortextools.WithString("operation",
			cortextools.Description("Must be 'remove_compression_policy'"),
			cortextools.Required(),
		),
		cortextools.WithString("target_table",
			cortextools.Description("The hypertable to remove compression policy from"),
			cortextools.Required(),
		),
	)
}

// CreateCompressionSettingsTool creates a tool for retrieving compression settings
func (t *TimescaleDBTool) CreateCompressionSettingsTool(name string, dbID string) interface{} {
	return cortextools.NewTool(
		name,
		cortextools.WithDescription(fmt.Sprintf("Get compression settings for TimescaleDB hypertable on %s", dbID)),
		cortextools.WithString("operation",
			cortextools.Description("Must be 'get_compression_settings'"),
			cortextools.Required(),
		),
		cortextools.WithString("target_table",
			cortextools.Description("The hypertable to get compression settings for"),
			cortextools.Required(),
		),
	)
}

// CreateRetentionPolicyTool creates a specific tool for managing retention policies
func (t *TimescaleDBTool) CreateRetentionPolicyTool(name string, dbID string) interface{} {
	return cortextools.NewTool(
		name,
		cortextools.WithDescription(fmt.Sprintf("Manage TimescaleDB retention policies on %s", dbID)),
		cortextools.WithString("operation",
			cortextools.Description("Operation (add_retention_policy, remove_retention_policy, get_retention_policy)"),
			cortextools.Required(),
		),
		cortextools.WithString("target_table",
			cortextools.Description("The hypertable to manage retention policy for"),
			cortextools.Required(),
		),
		cortextools.WithString("retention_interval",
			cortextools.Description("Time interval for data retention (e.g., '30 days', '6 months')"),
		),
	)
}

// CreateTimeSeriesQueryTool creates a specific tool for time-series queries
func (t *TimescaleDBTool) CreateTimeSeriesQueryTool(name string, dbID string) interface{} {
	return cortextools.NewTool(
		name,
		cortextools.WithDescription(fmt.Sprintf("Execute time-series queries on TimescaleDB %s", dbID)),
		cortextools.WithString("operation",
			cortextools.Description("Must be 'time_series_query'"),
			cortextools.Required(),
		),
		cortextools.WithString("target_table",
			cortextools.Description("The table to query"),
			cortextools.Required(),
		),
		cortextools.WithString("time_column",
			cortextools.Description("The timestamp column for time bucketing"),
			cortextools.Required(),
		),
		cortextools.WithString("bucket_interval",
			cortextools.Description("Time bucket interval (e.g., '1 hour', '1 day')"),
			cortextools.Required(),
		),
		cortextools.WithString("start_time",
			cortextools.Description("Start of time range (e.g., '2023-01-01')"),
		),
		cortextools.WithString("end_time",
			cortextools.Description("End of time range (e.g., '2023-01-31')"),
		),
		cortextools.WithString("aggregations",
			cortextools.Description("Comma-separated list of aggregations (e.g., 'AVG(temp),MAX(temp),COUNT(*)')"),
		),
		cortextools.WithString("where_condition",
			cortextools.Description("Additional WHERE conditions"),
		),
		cortextools.WithString("group_by",
			cortextools.Description("Additional GROUP BY columns (comma-separated)"),
		),
		cortextools.WithString("limit",
			cortextools.Description("Maximum number of rows to return"),
		),
	)
}

// CreateTimeSeriesAnalyzeTool creates a specific tool for analyzing time-series data
func (t *TimescaleDBTool) CreateTimeSeriesAnalyzeTool(name string, dbID string) interface{} {
	return cortextools.NewTool(
		name,
		cortextools.WithDescription(fmt.Sprintf("Analyze time-series data patterns on TimescaleDB %s", dbID)),
		cortextools.WithString("operation",
			cortextools.Description("Must be 'analyze_time_series'"),
			cortextools.Required(),
		),
		cortextools.WithString("target_table",
			cortextools.Description("The table to analyze"),
			cortextools.Required(),
		),
		cortextools.WithString("time_column",
			cortextools.Description("The timestamp column"),
			cortextools.Required(),
		),
		cortextools.WithString("start_time",
			cortextools.Description("Start of time range (e.g., '2023-01-01')"),
		),
		cortextools.WithString("end_time",
			cortextools.Description("End of time range (e.g., '2023-01-31')"),
		),
	)
}

// CreateContinuousAggregateTool creates a specific tool for creating continuous aggregates
func (t *TimescaleDBTool) CreateContinuousAggregateTool(name string, dbID string) interface{} {
	return cortextools.NewTool(
		name,
		cortextools.WithDescription(fmt.Sprintf("Create TimescaleDB continuous aggregate on %s", dbID)),
		cortextools.WithString("operation",
			cortextools.Description("Must be 'create_continuous_aggregate'"),
			cortextools.Required(),
		),
		cortextools.WithString("view_name",
			cortextools.Description("Name for the continuous aggregate view"),
			cortextools.Required(),
		),
		cortextools.WithString("source_table",
			cortextools.Description("Source table with raw data"),
			cortextools.Required(),
		),
		cortextools.WithString("time_column",
			cortextools.Description("Time column to bucket"),
			cortextools.Required(),
		),
		cortextools.WithString("bucket_interval",
			cortextools.Description("Time bucket interval (e.g., '1 hour', '1 day')"),
			cortextools.Required(),
		),
		cortextools.WithString("aggregations",
			cortextools.Description("Comma-separated list of aggregations (e.g., 'AVG(temp),MAX(temp),COUNT(*)')"),
		),
		cortextools.WithString("where_condition",
			cortextools.Description("WHERE condition to filter source data"),
		),
		cortextools.WithBoolean("with_data",
			cortextools.Description("Whether to materialize data immediately"),
		),
		cortextools.WithBoolean("refresh_policy",
			cortextools.Description("Whether to add a refresh policy"),
		),
		cortextools.WithString("refresh_interval",
			cortextools.Description("Refresh interval (e.g., '1 day')"),
		),
	)
}

// CreateContinuousAggregateRefreshTool creates a specific tool for refreshing continuous aggregates
func (t *TimescaleDBTool) CreateContinuousAggregateRefreshTool(name string, dbID string) interface{} {
	return cortextools.NewTool(
		name,
		cortextools.WithDescription(fmt.Sprintf("Refresh TimescaleDB continuous aggregate on %s", dbID)),
		cortextools.WithString("operation",
			cortextools.Description("Must be 'refresh_continuous_aggregate'"),
			cortextools.Required(),
		),
		cortextools.WithString("view_name",
			cortextools.Description("Name of the continuous aggregate view"),
			cortextools.Required(),
		),
		cortextools.WithString("start_time",
			cortextools.Description("Start of time range to refresh (e.g., '2023-01-01')"),
		),
		cortextools.WithString("end_time",
			cortextools.Description("End of time range to refresh (e.g., '2023-01-31')"),
		),
	)
}

// HandleRequest handles a tool request
func (t *TimescaleDBTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Extract parameters from the request
	if request.Parameters == nil {
		return nil, fmt.Errorf("missing parameters")
	}

	operation, ok := request.Parameters["operation"].(string)
	if !ok || operation == "" {
		return nil, fmt.Errorf("operation parameter is required")
	}

	// Route to the appropriate handler based on the operation
	switch strings.ToLower(operation) {
	case "create_hypertable":
		return t.handleCreateHypertable(ctx, request, dbID, useCase)
	case "list_hypertables":
		return t.handleListHypertables(ctx, request, dbID, useCase)
	case "enable_compression":
		return t.handleEnableCompression(ctx, request, dbID, useCase)
	case "disable_compression":
		return t.handleDisableCompression(ctx, request, dbID, useCase)
	case "add_compression_policy":
		return t.handleAddCompressionPolicy(ctx, request, dbID, useCase)
	case "remove_compression_policy":
		return t.handleRemoveCompressionPolicy(ctx, request, dbID, useCase)
	case "get_compression_settings":
		return t.handleGetCompressionSettings(ctx, request, dbID, useCase)
	case "add_retention_policy":
		return t.handleAddRetentionPolicy(ctx, request, dbID, useCase)
	case "remove_retention_policy":
		return t.handleRemoveRetentionPolicy(ctx, request, dbID, useCase)
	case "get_retention_policy":
		return t.handleGetRetentionPolicy(ctx, request, dbID, useCase)
	case "time_series_query":
		return t.handleTimeSeriesQuery(ctx, request, dbID, useCase)
	case "analyze_time_series":
		return t.handleTimeSeriesAnalyze(ctx, request, dbID, useCase)
	case "create_continuous_aggregate":
		return t.handleCreateContinuousAggregate(ctx, request, dbID, useCase)
	case "refresh_continuous_aggregate":
		return t.handleRefreshContinuousAggregate(ctx, request, dbID, useCase)
	default:
		return map[string]interface{}{"message": fmt.Sprintf("Operation '%s' not implemented yet", operation)}, nil
	}
}

// handleCreateHypertable handles the create_hypertable operation
func (t *TimescaleDBTool) handleCreateHypertable(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Extract required parameters
	targetTable, ok := request.Parameters["target_table"].(string)
	if !ok || targetTable == "" {
		return nil, fmt.Errorf("target_table parameter is required")
	}

	timeColumn, ok := request.Parameters["time_column"].(string)
	if !ok || timeColumn == "" {
		return nil, fmt.Errorf("time_column parameter is required")
	}

	// Extract optional parameters
	chunkTimeInterval := getStringParam(request.Parameters, "chunk_time_interval")
	partitioningColumn := getStringParam(request.Parameters, "partitioning_column")
	ifNotExists := getBoolParam(request.Parameters, "if_not_exists")

	// Build the SQL statement to create a hypertable
	sql := buildCreateHypertableSQL(targetTable, timeColumn, chunkTimeInterval, partitioningColumn, ifNotExists)

	// Check if the database is PostgreSQL (TimescaleDB requires PostgreSQL)
	dbType, err := useCase.GetDatabaseType(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database type: %w", err)
	}

	if !strings.Contains(strings.ToLower(dbType), "postgres") {
		return nil, fmt.Errorf("TimescaleDB operations are only supported on PostgreSQL databases")
	}

	// Execute the statement
	result, err := useCase.ExecuteStatement(ctx, dbID, sql, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create hypertable: %w", err)
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("Successfully created hypertable '%s' with time column '%s'", targetTable, timeColumn),
		"details": result,
	}, nil
}

// handleListHypertables handles the list_hypertables operation
func (t *TimescaleDBTool) handleListHypertables(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Check if the database is PostgreSQL (TimescaleDB requires PostgreSQL)
	dbType, err := useCase.GetDatabaseType(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database type: %w", err)
	}

	if !strings.Contains(strings.ToLower(dbType), "postgres") {
		return nil, fmt.Errorf("TimescaleDB operations are only supported on PostgreSQL databases")
	}

	// Build the SQL query to list hypertables
	sql := `
		SELECT h.table_name, h.schema_name, d.column_name as time_column,
			count(d.id) as num_dimensions,
			(
				SELECT column_name FROM _timescaledb_catalog.dimension 
				WHERE hypertable_id = h.id AND column_type != 'TIMESTAMP' 
				AND column_type != 'TIMESTAMPTZ' 
				LIMIT 1
			) as space_column
		FROM _timescaledb_catalog.hypertable h
		JOIN _timescaledb_catalog.dimension d ON h.id = d.hypertable_id
		GROUP BY h.id, h.table_name, h.schema_name
	`

	// Execute the statement
	result, err := useCase.ExecuteStatement(ctx, dbID, sql, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list hypertables: %w", err)
	}

	return map[string]interface{}{
		"message": "Successfully retrieved hypertables list",
		"details": result,
	}, nil
}

// handleEnableCompression handles the enable_compression operation
func (t *TimescaleDBTool) handleEnableCompression(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Extract required parameters
	targetTable, ok := request.Parameters["target_table"].(string)
	if !ok || targetTable == "" {
		return nil, fmt.Errorf("target_table parameter is required")
	}

	// Extract optional interval parameter
	afterInterval := getStringParam(request.Parameters, "after")

	// Check if the database is PostgreSQL (TimescaleDB requires PostgreSQL)
	dbType, err := useCase.GetDatabaseType(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database type: %w", err)
	}

	if !strings.Contains(strings.ToLower(dbType), "postgres") {
		return nil, fmt.Errorf("TimescaleDB operations are only supported on PostgreSQL databases")
	}

	// Build the SQL statement to enable compression
	sql := fmt.Sprintf("ALTER TABLE %s SET (timescaledb.compress = true)", targetTable)

	// Execute the statement
	_, err = useCase.ExecuteStatement(ctx, dbID, sql, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to enable compression: %w", err)
	}

	var message string
	// If interval is specified, add compression policy
	if afterInterval != "" {
		// Build the SQL statement for compression policy
		policySQL := fmt.Sprintf("SELECT add_compression_policy('%s', INTERVAL '%s')", targetTable, afterInterval)

		// Execute the statement
		_, err = useCase.ExecuteStatement(ctx, dbID, policySQL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to add compression policy: %w", err)
		}

		message = fmt.Sprintf("Successfully enabled compression on hypertable '%s' with automatic compression after '%s'", targetTable, afterInterval)
	} else {
		message = fmt.Sprintf("Successfully enabled compression on hypertable '%s'", targetTable)
	}

	return map[string]interface{}{
		"message": message,
	}, nil
}

// handleDisableCompression handles the disable_compression operation
func (t *TimescaleDBTool) handleDisableCompression(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Extract required parameters
	targetTable, ok := request.Parameters["target_table"].(string)
	if !ok || targetTable == "" {
		return nil, fmt.Errorf("target_table parameter is required")
	}

	// Check if the database is PostgreSQL (TimescaleDB requires PostgreSQL)
	dbType, err := useCase.GetDatabaseType(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database type: %w", err)
	}

	if !strings.Contains(strings.ToLower(dbType), "postgres") {
		return nil, fmt.Errorf("TimescaleDB operations are only supported on PostgreSQL databases")
	}

	// First, find and remove any existing compression policy
	policyQuery := fmt.Sprintf(
		"SELECT job_id FROM timescaledb_information.jobs WHERE hypertable_name = '%s' AND proc_name = 'policy_compression'",
		targetTable,
	)

	policyResult, err := useCase.ExecuteStatement(ctx, dbID, policyQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing compression policy: %w", err)
	}

	// Check if a policy exists and remove it
	if policyResult != "" && policyResult != "[]" {
		// Parse the JSON result
		var policies []map[string]interface{}
		if err := json.Unmarshal([]byte(policyResult), &policies); err != nil {
			return nil, fmt.Errorf("failed to parse policy result: %w", err)
		}

		if len(policies) > 0 && policies[0]["job_id"] != nil {
			// Remove the policy
			jobID := policies[0]["job_id"]
			removePolicyQuery := fmt.Sprintf("SELECT remove_compression_policy(%v)", jobID)
			_, err = useCase.ExecuteStatement(ctx, dbID, removePolicyQuery, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to remove compression policy: %w", err)
			}
		}
	}

	// Build the SQL statement to disable compression
	sql := fmt.Sprintf("ALTER TABLE %s SET (timescaledb.compress = false)", targetTable)

	// Execute the statement
	_, err = useCase.ExecuteStatement(ctx, dbID, sql, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to disable compression: %w", err)
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("Successfully disabled compression on hypertable '%s'", targetTable),
	}, nil
}

// handleAddCompressionPolicy handles the add_compression_policy operation
func (t *TimescaleDBTool) handleAddCompressionPolicy(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Extract required parameters
	targetTable, ok := request.Parameters["target_table"].(string)
	if !ok || targetTable == "" {
		return nil, fmt.Errorf("target_table parameter is required")
	}

	interval, ok := request.Parameters["interval"].(string)
	if !ok || interval == "" {
		return nil, fmt.Errorf("interval parameter is required")
	}

	// Extract optional parameters
	segmentBy := getStringParam(request.Parameters, "segment_by")
	orderBy := getStringParam(request.Parameters, "order_by")

	// Check if the database is PostgreSQL (TimescaleDB requires PostgreSQL)
	dbType, err := useCase.GetDatabaseType(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database type: %w", err)
	}

	if !strings.Contains(strings.ToLower(dbType), "postgres") {
		return nil, fmt.Errorf("TimescaleDB operations are only supported on PostgreSQL databases")
	}

	// First, check if compression is enabled
	compressionQuery := fmt.Sprintf(
		"SELECT compress FROM timescaledb_information.hypertables WHERE hypertable_name = '%s'",
		targetTable,
	)

	compressionResult, err := useCase.ExecuteStatement(ctx, dbID, compressionQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check compression status: %w", err)
	}

	// Parse the result to check if compression is enabled
	var hypertables []map[string]interface{}
	if err := json.Unmarshal([]byte(compressionResult), &hypertables); err != nil {
		return nil, fmt.Errorf("failed to parse hypertable info: %w", err)
	}

	if len(hypertables) == 0 {
		return nil, fmt.Errorf("table '%s' is not a hypertable", targetTable)
	}

	isCompressed := false
	if compress, ok := hypertables[0]["compress"]; ok && compress != nil {
		isCompressed = fmt.Sprintf("%v", compress) == "true"
	}

	// If compression isn't enabled, enable it first
	if !isCompressed {
		enableSQL := fmt.Sprintf("ALTER TABLE %s SET (timescaledb.compress = true)", targetTable)
		_, err = useCase.ExecuteStatement(ctx, dbID, enableSQL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to enable compression: %w", err)
		}
	}

	// Build the compression policy SQL
	var policyQueryBuilder strings.Builder
	policyQueryBuilder.WriteString(fmt.Sprintf("SELECT add_compression_policy('%s', INTERVAL '%s'", targetTable, interval))

	if segmentBy != "" {
		policyQueryBuilder.WriteString(fmt.Sprintf(", segmentby => '%s'", segmentBy))
	}

	if orderBy != "" {
		policyQueryBuilder.WriteString(fmt.Sprintf(", orderby => '%s'", orderBy))
	}

	policyQueryBuilder.WriteString(")")

	// Execute the statement to add the compression policy
	_, err = useCase.ExecuteStatement(ctx, dbID, policyQueryBuilder.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to add compression policy: %w", err)
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("Successfully added compression policy to hypertable '%s'", targetTable),
	}, nil
}

// handleRemoveCompressionPolicy handles the remove_compression_policy operation
func (t *TimescaleDBTool) handleRemoveCompressionPolicy(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Extract required parameters
	targetTable, ok := request.Parameters["target_table"].(string)
	if !ok || targetTable == "" {
		return nil, fmt.Errorf("target_table parameter is required")
	}

	// Check if the database is PostgreSQL (TimescaleDB requires PostgreSQL)
	dbType, err := useCase.GetDatabaseType(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database type: %w", err)
	}

	if !strings.Contains(strings.ToLower(dbType), "postgres") {
		return nil, fmt.Errorf("TimescaleDB operations are only supported on PostgreSQL databases")
	}

	// Find the policy ID
	policyQuery := fmt.Sprintf(
		"SELECT job_id FROM timescaledb_information.jobs WHERE hypertable_name = '%s' AND proc_name = 'policy_compression'",
		targetTable,
	)

	policyResult, err := useCase.ExecuteStatement(ctx, dbID, policyQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find compression policy: %w", err)
	}

	// Parse the result to get the job ID
	var policies []map[string]interface{}
	if err := json.Unmarshal([]byte(policyResult), &policies); err != nil {
		return nil, fmt.Errorf("failed to parse policy info: %w", err)
	}

	if len(policies) == 0 {
		return map[string]interface{}{
			"message": fmt.Sprintf("No compression policy found for hypertable '%s'", targetTable),
		}, nil
	}

	jobID := policies[0]["job_id"]
	if jobID == nil {
		return nil, fmt.Errorf("invalid job ID for compression policy")
	}

	// Remove the policy
	removeSQL := fmt.Sprintf("SELECT remove_compression_policy(%v)", jobID)
	_, err = useCase.ExecuteStatement(ctx, dbID, removeSQL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to remove compression policy: %w", err)
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("Successfully removed compression policy from hypertable '%s'", targetTable),
	}, nil
}

// handleGetCompressionSettings handles the get_compression_settings operation
func (t *TimescaleDBTool) handleGetCompressionSettings(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Extract required parameters
	targetTable, ok := request.Parameters["target_table"].(string)
	if !ok || targetTable == "" {
		return nil, fmt.Errorf("target_table parameter is required")
	}

	// Check if the database is PostgreSQL (TimescaleDB requires PostgreSQL)
	dbType, err := useCase.GetDatabaseType(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database type: %w", err)
	}

	if !strings.Contains(strings.ToLower(dbType), "postgres") {
		return nil, fmt.Errorf("TimescaleDB operations are only supported on PostgreSQL databases")
	}

	// Check if the table is a hypertable and has compression enabled
	hypertableQuery := fmt.Sprintf(
		"SELECT compress FROM timescaledb_information.hypertables WHERE hypertable_name = '%s'",
		targetTable,
	)

	hypertableResult, err := useCase.ExecuteStatement(ctx, dbID, hypertableQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check hypertable info: %w", err)
	}

	// Parse the result
	var hypertables []map[string]interface{}
	if err := json.Unmarshal([]byte(hypertableResult), &hypertables); err != nil {
		return nil, fmt.Errorf("failed to parse hypertable info: %w", err)
	}

	if len(hypertables) == 0 {
		return nil, fmt.Errorf("table '%s' is not a hypertable", targetTable)
	}

	// Create settings object
	settings := map[string]interface{}{
		"hypertable_name":      targetTable,
		"compression_enabled":  false,
		"segment_by":           nil,
		"order_by":             nil,
		"chunk_time_interval":  nil,
		"compression_interval": nil,
	}

	isCompressed := false
	if compress, ok := hypertables[0]["compress"]; ok && compress != nil {
		isCompressed = fmt.Sprintf("%v", compress) == "true"
	}

	settings["compression_enabled"] = isCompressed

	if isCompressed {
		// Get compression settings
		compressionQuery := fmt.Sprintf(
			"SELECT segmentby, orderby FROM timescaledb_information.compression_settings WHERE hypertable_name = '%s'",
			targetTable,
		)

		compressionResult, err := useCase.ExecuteStatement(ctx, dbID, compressionQuery, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get compression settings: %w", err)
		}

		var compressionSettings []map[string]interface{}
		if err := json.Unmarshal([]byte(compressionResult), &compressionSettings); err != nil {
			return nil, fmt.Errorf("failed to parse compression settings: %w", err)
		}

		if len(compressionSettings) > 0 {
			if segmentBy, ok := compressionSettings[0]["segmentby"]; ok && segmentBy != nil {
				settings["segment_by"] = segmentBy
			}

			if orderBy, ok := compressionSettings[0]["orderby"]; ok && orderBy != nil {
				settings["order_by"] = orderBy
			}
		}

		// Get policy information
		policyQuery := fmt.Sprintf(
			"SELECT s.schedule_interval, h.chunk_time_interval FROM timescaledb_information.jobs j "+
				"JOIN timescaledb_information.job_stats s ON j.job_id = s.job_id "+
				"JOIN timescaledb_information.hypertables h ON j.hypertable_name = h.hypertable_name "+
				"WHERE j.hypertable_name = '%s' AND j.proc_name = 'policy_compression'",
			targetTable,
		)

		policyResult, err := useCase.ExecuteStatement(ctx, dbID, policyQuery, nil)
		if err == nil {
			var policyInfo []map[string]interface{}
			if err := json.Unmarshal([]byte(policyResult), &policyInfo); err != nil {
				return nil, fmt.Errorf("failed to parse policy info: %w", err)
			}

			if len(policyInfo) > 0 {
				if interval, ok := policyInfo[0]["schedule_interval"]; ok && interval != nil {
					settings["compression_interval"] = interval
				}

				if chunkInterval, ok := policyInfo[0]["chunk_time_interval"]; ok && chunkInterval != nil {
					settings["chunk_time_interval"] = chunkInterval
				}
			}
		}
	}

	return map[string]interface{}{
		"message":  fmt.Sprintf("Retrieved compression settings for hypertable '%s'", targetTable),
		"settings": settings,
	}, nil
}

// handleAddRetentionPolicy handles the add_retention_policy operation
func (t *TimescaleDBTool) handleAddRetentionPolicy(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Extract required parameters
	targetTable, ok := request.Parameters["target_table"].(string)
	if !ok || targetTable == "" {
		return nil, fmt.Errorf("target_table parameter is required")
	}

	retentionInterval, ok := request.Parameters["retention_interval"].(string)
	if !ok || retentionInterval == "" {
		return nil, fmt.Errorf("retention_interval parameter is required")
	}

	// Check if the database is PostgreSQL (TimescaleDB requires PostgreSQL)
	dbType, err := useCase.GetDatabaseType(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database type: %w", err)
	}

	if !strings.Contains(strings.ToLower(dbType), "postgres") {
		return nil, fmt.Errorf("TimescaleDB operations are only supported on PostgreSQL databases")
	}

	// Build the SQL statement to add a retention policy
	sql := fmt.Sprintf("SELECT add_retention_policy('%s', INTERVAL '%s')", targetTable, retentionInterval)

	// Execute the statement
	result, err := useCase.ExecuteStatement(ctx, dbID, sql, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to add retention policy: %w", err)
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("Successfully added retention policy to '%s' with interval '%s'", targetTable, retentionInterval),
		"details": result,
	}, nil
}

// handleRemoveRetentionPolicy handles the remove_retention_policy operation
func (t *TimescaleDBTool) handleRemoveRetentionPolicy(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Extract required parameters
	targetTable, ok := request.Parameters["target_table"].(string)
	if !ok || targetTable == "" {
		return nil, fmt.Errorf("target_table parameter is required")
	}

	// Check if the database is PostgreSQL (TimescaleDB requires PostgreSQL)
	dbType, err := useCase.GetDatabaseType(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database type: %w", err)
	}

	if !strings.Contains(strings.ToLower(dbType), "postgres") {
		return nil, fmt.Errorf("TimescaleDB operations are only supported on PostgreSQL databases")
	}

	// First, find the policy job ID
	findPolicySQL := fmt.Sprintf(
		"SELECT job_id FROM timescaledb_information.jobs WHERE hypertable_name = '%s' AND proc_name = 'policy_retention'",
		targetTable,
	)

	// Execute the statement to find the policy
	policyResult, err := useCase.ExecuteStatement(ctx, dbID, findPolicySQL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find retention policy: %w", err)
	}

	// Check if we found a policy
	if policyResult == "[]" || policyResult == "" {
		return map[string]interface{}{
			"message": fmt.Sprintf("No retention policy found for table '%s'", targetTable),
		}, nil
	}

	// Now remove the policy - assuming we received a JSON array with the job_id
	removeSQL := fmt.Sprintf(
		"SELECT remove_retention_policy((SELECT job_id FROM timescaledb_information.jobs WHERE hypertable_name = '%s' AND proc_name = 'policy_retention' LIMIT 1))",
		targetTable,
	)

	// Execute the statement to remove the policy
	result, err := useCase.ExecuteStatement(ctx, dbID, removeSQL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to remove retention policy: %w", err)
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("Successfully removed retention policy from '%s'", targetTable),
		"details": result,
	}, nil
}

// handleGetRetentionPolicy handles the get_retention_policy operation
func (t *TimescaleDBTool) handleGetRetentionPolicy(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Extract required parameters
	targetTable, ok := request.Parameters["target_table"].(string)
	if !ok || targetTable == "" {
		return nil, fmt.Errorf("target_table parameter is required")
	}

	// Check if the database is PostgreSQL (TimescaleDB requires PostgreSQL)
	dbType, err := useCase.GetDatabaseType(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database type: %w", err)
	}

	if !strings.Contains(strings.ToLower(dbType), "postgres") {
		return nil, fmt.Errorf("TimescaleDB operations are only supported on PostgreSQL databases")
	}

	// Build the SQL query to get retention policy details
	sql := fmt.Sprintf(`
		SELECT 
			'%s' as hypertable_name,
			js.schedule_interval as retention_interval,
			CASE WHEN j.job_id IS NOT NULL THEN true ELSE false END as retention_enabled
		FROM 
			timescaledb_information.jobs j
		JOIN 
			timescaledb_information.job_stats js ON j.job_id = js.job_id
		WHERE 
			j.hypertable_name = '%s' AND j.proc_name = 'policy_retention'
	`, targetTable, targetTable)

	// Execute the statement
	result, err := useCase.ExecuteStatement(ctx, dbID, sql, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get retention policy: %w", err)
	}

	// Check if we got any results
	if result == "[]" || result == "" {
		// No retention policy found, return a default structure
		return map[string]interface{}{
			"message": fmt.Sprintf("No retention policy found for table '%s'", targetTable),
			"details": fmt.Sprintf(`[{"hypertable_name":"%s","retention_enabled":false}]`, targetTable),
		}, nil
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("Successfully retrieved retention policy for '%s'", targetTable),
		"details": result,
	}, nil
}

// handleTimeSeriesQuery handles the time_series_query operation
func (t *TimescaleDBTool) handleTimeSeriesQuery(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Extract required parameters
	targetTable, ok := request.Parameters["target_table"].(string)
	if !ok || targetTable == "" {
		return nil, fmt.Errorf("target_table parameter is required")
	}

	timeColumn, ok := request.Parameters["time_column"].(string)
	if !ok || timeColumn == "" {
		return nil, fmt.Errorf("time_column parameter is required")
	}

	bucketInterval, ok := request.Parameters["bucket_interval"].(string)
	if !ok || bucketInterval == "" {
		return nil, fmt.Errorf("bucket_interval parameter is required")
	}

	// Extract optional parameters
	startTimeStr := getStringParam(request.Parameters, "start_time")
	endTimeStr := getStringParam(request.Parameters, "end_time")
	aggregations := getStringParam(request.Parameters, "aggregations")
	whereCondition := getStringParam(request.Parameters, "where_condition")
	groupBy := getStringParam(request.Parameters, "group_by")
	limitStr := getStringParam(request.Parameters, "limit")

	// Set default values for optional parameters
	if aggregations == "" {
		aggregations = "count(*) as count"
	}

	// Build WHERE clause
	whereClause := ""
	if startTimeStr != "" && endTimeStr != "" {
		whereClause = fmt.Sprintf("%s BETWEEN '%s' AND '%s'", timeColumn, startTimeStr, endTimeStr)
		if whereCondition != "" {
			whereClause = fmt.Sprintf("%s AND %s", whereClause, whereCondition)
		}
	} else if whereCondition != "" {
		whereClause = whereCondition
	} else {
		whereClause = "1=1" // Always true if no conditions
	}

	// Set default group by if not provided
	if groupBy == "" {
		groupBy = "time_bucket"
	} else {
		groupBy = fmt.Sprintf("time_bucket, %s", groupBy)
	}

	// Set default limit if not provided
	limit := 1000 // Default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Build the SQL query
	sql := fmt.Sprintf(`
		SELECT 
			time_bucket('%s', %s) as time_bucket,
			%s
		FROM 
			%s
		WHERE 
			%s
		GROUP BY 
			%s
		ORDER BY 
			time_bucket
		LIMIT %d
	`, bucketInterval, timeColumn, aggregations, targetTable, whereClause, groupBy, limit)

	// Execute the query
	result, err := useCase.ExecuteStatement(ctx, dbID, sql, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute time-series query: %w", err)
	}

	return map[string]interface{}{
		"message": "Successfully retrieved time-series data",
		"details": result,
	}, nil
}

// handleTimeSeriesAnalyze handles the analyze_time_series operation
func (t *TimescaleDBTool) handleTimeSeriesAnalyze(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Extract required parameters
	targetTable, ok := request.Parameters["target_table"].(string)
	if !ok || targetTable == "" {
		return nil, fmt.Errorf("target_table parameter is required")
	}

	timeColumn, ok := request.Parameters["time_column"].(string)
	if !ok || timeColumn == "" {
		return nil, fmt.Errorf("time_column parameter is required")
	}

	// Extract optional parameters
	startTimeStr := getStringParam(request.Parameters, "start_time")
	endTimeStr := getStringParam(request.Parameters, "end_time")

	// Build WHERE clause
	whereClause := ""
	if startTimeStr != "" && endTimeStr != "" {
		whereClause = fmt.Sprintf("WHERE %s BETWEEN '%s' AND '%s'", timeColumn, startTimeStr, endTimeStr)
	}

	// Build the SQL query for basic time series analysis
	sql := fmt.Sprintf(`
		SELECT 
			COUNT(*) as row_count,
			MIN(%s) as min_time,
			MAX(%s) as max_time,
			(MAX(%s) - MIN(%s)) as time_span,
			COUNT(DISTINCT date_trunc('day', %s)) as unique_days
		FROM 
			%s
		%s
	`, timeColumn, timeColumn, timeColumn, timeColumn, timeColumn, targetTable, whereClause)

	// Execute the query
	result, err := useCase.ExecuteStatement(ctx, dbID, sql, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze time-series data: %w", err)
	}

	return map[string]interface{}{
		"message": "Successfully analyzed time-series data",
		"details": result,
	}, nil
}

// handleCreateContinuousAggregate handles the create_continuous_aggregate operation
func (t *TimescaleDBTool) handleCreateContinuousAggregate(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Extract required parameters
	viewName, ok := request.Parameters["view_name"].(string)
	if !ok || viewName == "" {
		return nil, fmt.Errorf("view_name parameter is required")
	}

	sourceTable, ok := request.Parameters["source_table"].(string)
	if !ok || sourceTable == "" {
		return nil, fmt.Errorf("source_table parameter is required")
	}

	timeColumn, ok := request.Parameters["time_column"].(string)
	if !ok || timeColumn == "" {
		return nil, fmt.Errorf("time_column parameter is required")
	}

	bucketInterval, ok := request.Parameters["bucket_interval"].(string)
	if !ok || bucketInterval == "" {
		return nil, fmt.Errorf("bucket_interval parameter is required")
	}

	// Extract optional parameters
	aggregationsStr := getStringParam(request.Parameters, "aggregations")
	whereCondition := getStringParam(request.Parameters, "where_condition")
	withData := getBoolParam(request.Parameters, "with_data")
	refreshPolicy := getBoolParam(request.Parameters, "refresh_policy")
	refreshInterval := getStringParam(request.Parameters, "refresh_interval")

	// Parse aggregations from comma-separated string
	var aggregationsParts []string
	if aggregationsStr != "" {
		aggregationsParts = strings.Split(aggregationsStr, ",")
	} else {
		// Default aggregation if none specified
		aggregationsParts = []string{"COUNT(*) AS count"}
	}

	// Build the SQL statement to create a continuous aggregate
	var builder strings.Builder
	builder.WriteString("CREATE MATERIALIZED VIEW ")
	builder.WriteString(viewName)
	builder.WriteString("\nAS SELECT\n    time_bucket('")
	builder.WriteString(bucketInterval)
	builder.WriteString("', ")
	builder.WriteString(timeColumn)
	builder.WriteString(") AS time_bucket")

	// Add aggregations
	for _, agg := range aggregationsParts {
		builder.WriteString(",\n    ")
		builder.WriteString(strings.TrimSpace(agg))
	}

	// Add FROM clause
	builder.WriteString("\nFROM ")
	builder.WriteString(sourceTable)

	// Add WHERE clause if specified
	if whereCondition != "" {
		builder.WriteString("\nWHERE ")
		builder.WriteString(whereCondition)
	}

	// Add GROUP BY clause
	builder.WriteString("\nGROUP BY time_bucket")

	// Add WITH DATA or WITH NO DATA
	if withData {
		builder.WriteString("\nWITH DATA")
	} else {
		builder.WriteString("\nWITH NO DATA")
	}

	// Execute the statement
	_, err := useCase.ExecuteStatement(ctx, dbID, builder.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create continuous aggregate: %w", err)
	}

	// Add refresh policy if requested
	if refreshPolicy && refreshInterval != "" {
		policySQL := fmt.Sprintf("SELECT add_continuous_aggregate_policy('%s', start_offset => INTERVAL '1 week', end_offset => INTERVAL '1 hour', schedule_interval => INTERVAL '%s')", viewName, refreshInterval)
		_, err := useCase.ExecuteStatement(ctx, dbID, policySQL, nil)
		if err != nil {
			return map[string]interface{}{
				"message": fmt.Sprintf("Created continuous aggregate '%s' but failed to add refresh policy: %s", viewName, err.Error()),
			}, nil
		}
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("Successfully created continuous aggregate '%s'", viewName),
		"sql":     builder.String(),
	}, nil
}

// handleRefreshContinuousAggregate handles the refresh_continuous_aggregate operation
func (t *TimescaleDBTool) handleRefreshContinuousAggregate(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// Extract required parameters
	viewName, ok := request.Parameters["view_name"].(string)
	if !ok || viewName == "" {
		return nil, fmt.Errorf("view_name parameter is required")
	}

	// Extract optional parameters
	startTimeStr := getStringParam(request.Parameters, "start_time")
	endTimeStr := getStringParam(request.Parameters, "end_time")

	// Build the SQL statement to refresh a continuous aggregate
	var sql string
	if startTimeStr != "" && endTimeStr != "" {
		sql = fmt.Sprintf("CALL refresh_continuous_aggregate('%s', '%s', '%s')",
			viewName, startTimeStr, endTimeStr)
	} else {
		sql = fmt.Sprintf("CALL refresh_continuous_aggregate('%s', NULL, NULL)", viewName)
	}

	// Execute the statement
	_, err := useCase.ExecuteStatement(ctx, dbID, sql, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh continuous aggregate: %w", err)
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("Successfully refreshed continuous aggregate '%s'", viewName),
	}, nil
}

// getStringParam safely extracts a string parameter from a parameter map
func getStringParam(params map[string]interface{}, key string) string {
	if value, ok := params[key].(string); ok {
		return value
	}
	return ""
}

// getBoolParam safely extracts a boolean parameter from a parameter map
func getBoolParam(params map[string]interface{}, key string) bool {
	if value, ok := params[key].(bool); ok {
		return value
	}
	return false
}

// buildCreateHypertableSQL constructs the SQL statement to create a hypertable
func buildCreateHypertableSQL(table, timeColumn, chunkTimeInterval, partitioningColumn string, ifNotExists bool) string {
	var args []string

	// Add required arguments: table name and time column
	args = append(args, fmt.Sprintf("'%s'", table))
	args = append(args, fmt.Sprintf("'%s'", timeColumn))

	// Build optional parameters
	var options []string

	if chunkTimeInterval != "" {
		options = append(options, fmt.Sprintf("chunk_time_interval => interval '%s'", chunkTimeInterval))
	}

	if partitioningColumn != "" {
		options = append(options, fmt.Sprintf("partitioning_column => '%s'", partitioningColumn))
	}

	options = append(options, fmt.Sprintf("if_not_exists => %t", ifNotExists))

	// Construct the full SQL statement
	sql := fmt.Sprintf("SELECT create_hypertable(%s", strings.Join(args, ", "))

	if len(options) > 0 {
		sql += ", " + strings.Join(options, ", ")
	}

	sql += ")"

	return sql
}

// RegisterTimescaleDBTools registers TimescaleDB tools
func RegisterTimescaleDBTools(registry interface{}) error {
	// Cast the registry to the expected type
	toolRegistry, ok := registry.(*ToolTypeFactory)
	if !ok {
		return fmt.Errorf("invalid registry type")
	}

	// Create the TimescaleDB tool
	tool := NewTimescaleDBTool()

	// Register it with the factory
	toolRegistry.Register(tool)

	return nil
}
