package mcp

// TODO: Refactor tool registration to reduce code duplication
// TODO: Implement better error handling with error types instead of generic errors
// TODO: Add metrics collection for tool usage and performance
// TODO: Improve logging with structured logs and log levels
// TODO: Consider implementing tool discovery mechanism to avoid hardcoded tool lists

import (
	"context"
	"fmt"

	"github.com/FreePeak/cortex/pkg/server"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// ToolRegistry structure to handle tool registration
type ToolRegistry struct {
	server          *ServerWrapper
	mcpServer       *server.MCPServer
	databaseUseCase UseCaseProvider
	factory         *ToolTypeFactory
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(mcpServer *server.MCPServer) *ToolRegistry {
	factory := NewToolTypeFactory()
	return &ToolRegistry{
		server:    NewServerWrapper(mcpServer),
		mcpServer: mcpServer,
		factory:   factory,
	}
}

// RegisterAllTools registers all tools with the server
func (tr *ToolRegistry) RegisterAllTools(ctx context.Context, useCase UseCaseProvider) error {
	tr.databaseUseCase = useCase

	// Get available databases
	dbList := useCase.ListDatabases()
	logger.Info("Found %d database connections for tool registration: %v", len(dbList), dbList)

	if len(dbList) == 0 {
		logger.Info("No databases available, registering mock tools")
		return tr.RegisterMockTools(ctx)
	}

	// Register database-specific tools
	registrationErrors := 0
	for _, dbID := range dbList {
		if err := tr.registerDatabaseTools(ctx, dbID); err != nil {
			logger.Error("Error registering tools for database %s: %v", dbID, err)
			registrationErrors++
		} else {
			logger.Info("Successfully registered tools for database %s", dbID)
		}
	}

	// Register common tools
	tr.registerCommonTools(ctx)

	if registrationErrors > 0 {
		return fmt.Errorf("errors occurred while registering tools for %d databases", registrationErrors)
	}
	return nil
}

// registerDatabaseTools registers all tools for a specific database
func (tr *ToolRegistry) registerDatabaseTools(ctx context.Context, dbID string) error {
	// Get all tool types from the factory
	toolTypeNames := []string{
		"query", "execute", "transaction", "performance", "schema",
	}

	logger.Info("Registering tools for database %s", dbID)

	// Special case for postgres - skip the database info call that's failing
	dbType, err := tr.databaseUseCase.GetDatabaseType(dbID)
	if err == nil && dbType == "postgres" {
		// For PostgreSQL, we'll manually create a minimal info structure
		// rather than calling the problematic method
		logger.Info("Using special handling for PostgreSQL database: %s", dbID)

		// Create a mock database info for PostgreSQL
		dbInfo := map[string]interface{}{
			"database": dbID,
			"tables":   []map[string]interface{}{},
		}

		logger.Info("Created mock database info for PostgreSQL database %s: %+v", dbID, dbInfo)

		// Register each tool type for this database
		registrationErrors := 0
		for _, typeName := range toolTypeNames {
			// Use simpler tool names: <tooltype>_<dbID>
			toolName := fmt.Sprintf("%s_%s", typeName, dbID)
			if err := tr.registerTool(ctx, typeName, toolName, dbID); err != nil {
				logger.Error("Error registering tool %s: %v", toolName, err)
				registrationErrors++
			} else {
				logger.Info("Successfully registered tool %s", toolName)
			}
		}

		// Check if TimescaleDB is available for this PostgreSQL database
		// by executing a simple check query
		checkQuery := "SELECT 1 FROM pg_extension WHERE extname = 'timescaledb'"
		result, err := tr.databaseUseCase.ExecuteQuery(ctx, dbID, checkQuery, nil)
		if err == nil && result != "[]" && result != "" {
			logger.Info("TimescaleDB extension detected for database %s, registering TimescaleDB tools", dbID)

			// Register TimescaleDB-specific tools
			timescaleTool := NewTimescaleDBTool()

			// Register time series query tool
			tsQueryToolName := fmt.Sprintf("timescaledb_timeseries_query_%s", dbID)
			tsQueryTool := timescaleTool.CreateTimeSeriesQueryTool(tsQueryToolName, dbID)
			if err := tr.server.AddTool(ctx, tsQueryTool, func(ctx context.Context, request server.ToolCallRequest) (interface{}, error) {
				response, err := timescaleTool.HandleRequest(ctx, request, dbID, tr.databaseUseCase)
				return FormatResponse(response, err)
			}); err != nil {
				logger.Error("Error registering TimescaleDB time series query tool: %v", err)
				registrationErrors++
			} else {
				logger.Info("Successfully registered TimescaleDB time series query tool: %s", tsQueryToolName)
			}

			// Register time series analyze tool
			tsAnalyzeToolName := fmt.Sprintf("timescaledb_analyze_timeseries_%s", dbID)
			tsAnalyzeTool := timescaleTool.CreateTimeSeriesAnalyzeTool(tsAnalyzeToolName, dbID)
			if err := tr.server.AddTool(ctx, tsAnalyzeTool, func(ctx context.Context, request server.ToolCallRequest) (interface{}, error) {
				response, err := timescaleTool.HandleRequest(ctx, request, dbID, tr.databaseUseCase)
				return FormatResponse(response, err)
			}); err != nil {
				logger.Error("Error registering TimescaleDB time series analyze tool: %v", err)
				registrationErrors++
			} else {
				logger.Info("Successfully registered TimescaleDB time series analyze tool: %s", tsAnalyzeToolName)
			}
		}

		if registrationErrors > 0 {
			return fmt.Errorf("errors occurred while registering %d tools", registrationErrors)
		}

		logger.Info("Completed registering tools for database %s", dbID)
		return nil
	}

	// For other database types, continue with the normal approach
	// Check if this database actually exists
	dbInfo, err := tr.databaseUseCase.GetDatabaseInfo(dbID)
	if err != nil {
		return fmt.Errorf("failed to get database info for %s: %w", dbID, err)
	}

	logger.Info("Database %s info: %+v", dbID, dbInfo)

	// Register each tool type for this database
	registrationErrors := 0
	for _, typeName := range toolTypeNames {
		// Use simpler tool names: <tooltype>_<dbID>
		toolName := fmt.Sprintf("%s_%s", typeName, dbID)
		if err := tr.registerTool(ctx, typeName, toolName, dbID); err != nil {
			logger.Error("Error registering tool %s: %v", toolName, err)
			registrationErrors++
		} else {
			logger.Info("Successfully registered tool %s", toolName)
		}
	}

	if registrationErrors > 0 {
		return fmt.Errorf("errors occurred while registering %d tools", registrationErrors)
	}

	logger.Info("Completed registering tools for database %s", dbID)
	return nil
}

// registerTool registers a tool with the server
func (tr *ToolRegistry) registerTool(ctx context.Context, toolTypeName string, name string, dbID string) error {
	logger.Info("Registering tool '%s' of type '%s' (database: %s)", name, toolTypeName, dbID)

	toolTypeImpl, ok := tr.factory.GetToolType(toolTypeName)
	if !ok {
		return fmt.Errorf("failed to get tool type for '%s'", toolTypeName)
	}

	tool := toolTypeImpl.CreateTool(name, dbID)

	return tr.server.AddTool(ctx, tool, func(ctx context.Context, request server.ToolCallRequest) (interface{}, error) {
		response, err := toolTypeImpl.HandleRequest(ctx, request, dbID, tr.databaseUseCase)
		return FormatResponse(response, err)
	})
}

// registerCommonTools registers tools that are not specific to a database
func (tr *ToolRegistry) registerCommonTools(ctx context.Context) {
	// Register the list_databases tool with simple name
	_, ok := tr.factory.GetToolType("list_databases")
	if ok {
		// Use simple name for list_databases tool
		listDbName := "list_databases"
		if err := tr.registerTool(ctx, "list_databases", listDbName, ""); err != nil {
			logger.Error("Error registering %s tool: %v", listDbName, err)
		} else {
			logger.Info("Successfully registered tool %s", listDbName)
		}
	}
}

// RegisterMockTools registers mock tools with the server when no db connections available
func (tr *ToolRegistry) RegisterMockTools(ctx context.Context) error {
	logger.Info("Registering mock tools")

	// For each tool type, register a simplified mock tool
	for toolTypeName := range tr.factory.toolTypes {
		// Format: mock_<tooltype>
		mockToolName := fmt.Sprintf("mock_%s", toolTypeName)

		toolTypeImpl, ok := tr.factory.GetToolType(toolTypeName)
		if !ok {
			logger.Warn("Failed to get tool type for '%s'", toolTypeName)
			continue
		}

		tool := toolTypeImpl.CreateTool(mockToolName, "mock")

		err := tr.server.AddTool(ctx, tool, func(ctx context.Context, request server.ToolCallRequest) (interface{}, error) {
			response, err := toolTypeImpl.HandleRequest(ctx, request, "mock", tr.databaseUseCase)
			return FormatResponse(response, err)
		})

		if err != nil {
			logger.Error("Failed to register mock tool '%s': %v", mockToolName, err)
			continue
		}
	}

	return nil
}

// RegisterCursorCompatibleTools is kept for backward compatibility but does nothing
// as we now register tools with simple names directly
func (tr *ToolRegistry) RegisterCursorCompatibleTools(ctx context.Context) error {
	// This function is intentionally empty as we now register tools with simple names directly
	return nil
}
