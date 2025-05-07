package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/FreePeak/cortex/pkg/tools"
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
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetDescription(dbID)),
		tools.WithString("operation",
			tools.Description("TimescaleDB operation to perform"),
			tools.Required(),
		),
		tools.WithString("target_table",
			tools.Description("The table to perform the operation on"),
			tools.Required(),
		),
	)
}

// CreateHypertableTool creates a specific tool for hypertable creation
func (t *TimescaleDBTool) CreateHypertableTool(name string, dbID string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(fmt.Sprintf("Create TimescaleDB hypertable on %s", dbID)),
		tools.WithString("operation",
			tools.Description("Must be 'create_hypertable'"),
			tools.Required(),
		),
		tools.WithString("target_table",
			tools.Description("The table to convert to a hypertable"),
			tools.Required(),
		),
		tools.WithString("time_column",
			tools.Description("The timestamp column for the hypertable"),
			tools.Required(),
		),
		tools.WithString("chunk_time_interval",
			tools.Description("Time interval for chunks (e.g., '1 day')"),
		),
		tools.WithString("partitioning_column",
			tools.Description("Optional column for space partitioning"),
		),
		tools.WithBoolean("if_not_exists",
			tools.Description("Skip if hypertable already exists"),
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
	return nil
}
