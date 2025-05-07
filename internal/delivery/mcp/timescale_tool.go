package mcp

import (
	"context"
	"fmt"
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

// HandleRequest handles a tool request
func (t *TimescaleDBTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase interface{}) (interface{}, error) {
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
	case "add_retention_policy":
		return t.handleAddRetentionPolicy(ctx, request, dbID, useCase)
	case "remove_retention_policy":
		return t.handleRemoveRetentionPolicy(ctx, request, dbID, useCase)
	case "get_retention_policy":
		return t.handleGetRetentionPolicy(ctx, request, dbID, useCase)
	default:
		return map[string]interface{}{"message": fmt.Sprintf("Operation '%s' not implemented yet", operation)}, nil
	}
}

// handleCreateHypertable handles the create_hypertable operation
func (t *TimescaleDBTool) handleCreateHypertable(ctx context.Context, request server.ToolCallRequest, dbID string, useCase interface{}) (interface{}, error) {
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

	// Cast useCase to the expected type
	dbUseCase, ok := useCase.(interface {
		ExecuteStatement(ctx context.Context, dbID, statement string, params []interface{}) (string, error)
		GetDatabaseType(dbID string) (string, error)
	})
	if !ok {
		return nil, fmt.Errorf("invalid useCase type")
	}

	// Check if the database is PostgreSQL (TimescaleDB requires PostgreSQL)
	dbType, err := dbUseCase.GetDatabaseType(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database type: %w", err)
	}

	if !strings.Contains(strings.ToLower(dbType), "postgres") {
		return nil, fmt.Errorf("TimescaleDB operations are only supported on PostgreSQL databases")
	}

	// Execute the statement
	result, err := dbUseCase.ExecuteStatement(ctx, dbID, sql, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create hypertable: %w", err)
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("Successfully created hypertable '%s' with time column '%s'", targetTable, timeColumn),
		"details": result,
	}, nil
}

// handleListHypertables handles the list_hypertables operation
func (t *TimescaleDBTool) handleListHypertables(ctx context.Context, request server.ToolCallRequest, dbID string, useCase interface{}) (interface{}, error) {
	// Cast useCase to the expected type
	dbUseCase, ok := useCase.(interface {
		ExecuteStatement(ctx context.Context, dbID, statement string, params []interface{}) (string, error)
		GetDatabaseType(dbID string) (string, error)
	})
	if !ok {
		return nil, fmt.Errorf("invalid useCase type")
	}

	// Check if the database is PostgreSQL (TimescaleDB requires PostgreSQL)
	dbType, err := dbUseCase.GetDatabaseType(dbID)
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
	result, err := dbUseCase.ExecuteStatement(ctx, dbID, sql, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list hypertables: %w", err)
	}

	return map[string]interface{}{
		"message": "Successfully retrieved hypertables list",
		"details": result,
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
	// We'll just return nil for now since the actual tool registration happens elsewhere
	// Once we understand the proper way to register tools in this codebase, we can implement this function

	// For now, we'll just log that we have a list_hypertables tool available
	return nil
}
