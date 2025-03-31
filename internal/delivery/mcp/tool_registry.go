package mcp

// TODO: Refactor tool registration to reduce code duplication
// TODO: Implement better error handling with error types instead of generic errors
// TODO: Add metrics collection for tool usage and performance
// TODO: Improve logging with structured logs and log levels
// TODO: Consider implementing tool discovery mechanism to avoid hardcoded tool lists

import (
	"context"
	"fmt"
	"log"

	"github.com/FreePeak/cortex/pkg/server"
)

// ToolRegistry structure to handle tool registration
type ToolRegistry struct {
	server          *ServerWrapper
	mcpServer       *server.MCPServer
	databaseUseCase UseCaseProvider
	factory         *ToolTypeFactory
	serverName      string
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(mcpServer *server.MCPServer, serverName string) *ToolRegistry {
	factory := NewToolTypeFactory()
	return &ToolRegistry{
		server:     NewServerWrapper(mcpServer),
		mcpServer:  mcpServer,
		factory:    factory,
		serverName: serverName,
	}
}

// RegisterAllTools registers all tools with the server
func (tr *ToolRegistry) RegisterAllTools(ctx context.Context, useCase UseCaseProvider) error {
	tr.databaseUseCase = useCase

	// Get available databases
	dbList := useCase.ListDatabases()
	log.Printf("Found %d database connections for tool registration: %v", len(dbList), dbList)

	if len(dbList) == 0 {
		log.Printf("No databases available, registering mock tools")
		return tr.RegisterMockTools(ctx)
	}

	// Register database-specific tools
	registrationErrors := 0
	for _, dbID := range dbList {
		if err := tr.registerDatabaseTools(ctx, dbID); err != nil {
			log.Printf("Error registering tools for database %s: %v", dbID, err)
			registrationErrors++
		} else {
			log.Printf("Successfully registered tools for database %s", dbID)
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

	log.Printf("Registering tools for database %s", dbID)

	// Check if this database actually exists
	dbInfo, err := tr.databaseUseCase.GetDatabaseInfo(dbID)
	if err != nil {
		return fmt.Errorf("failed to get database info for %s: %w", dbID, err)
	}

	log.Printf("Database %s info: %+v", dbID, dbInfo)

	// Register each tool type for this database
	registrationErrors := 0
	for _, typeName := range toolTypeNames {
		// Create tool name with cursor-compatible prefix: mcp_<servername>_<tooltype>_<dbID>
		toolName := fmt.Sprintf("mcp_%s_%s_%s", tr.serverName, typeName, dbID)
		if err := tr.registerTool(ctx, typeName, toolName, dbID); err != nil {
			log.Printf("Error registering tool %s: %v", toolName, err)
			registrationErrors++
		} else {
			log.Printf("Successfully registered tool %s", toolName)
		}
	}

	if registrationErrors > 0 {
		return fmt.Errorf("errors occurred while registering %d tools", registrationErrors)
	}

	log.Printf("Completed registering tools for database %s", dbID)
	return nil
}

// registerTool registers a tool with the server
func (tr *ToolRegistry) registerTool(ctx context.Context, toolTypeName string, name string, dbID string) error {
	log.Printf("Registering tool '%s' of type '%s' (database: %s)", name, toolTypeName, dbID)

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
	// Register the list_databases tool with prefix for Cursor compatibility
	_, ok := tr.factory.GetToolType("list_databases")
	if ok {
		// Use prefixed name for list_databases tool: mcp_<servername>_list_databases
		listDbName := fmt.Sprintf("mcp_%s_list_databases", tr.serverName)
		if err := tr.registerTool(ctx, "list_databases", listDbName, ""); err != nil {
			log.Printf("Error registering %s tool: %v", listDbName, err)
		} else {
			log.Printf("Successfully registered tool %s", listDbName)
		}
	}
}

// RegisterMockTools registers mock tools with the server when no db connections available
func (tr *ToolRegistry) RegisterMockTools(ctx context.Context) error {
	log.Printf("Registering mock tools")

	// For each tool type, register a simplified mock tool
	for toolTypeName := range tr.factory.toolTypes {
		// Format: mcp_<servername>_mock_<tooltype>
		mockToolName := fmt.Sprintf("mcp_%s_mock_%s", tr.serverName, toolTypeName)

		toolTypeImpl, ok := tr.factory.GetToolType(toolTypeName)
		if !ok {
			log.Printf("Failed to get tool type for '%s'", toolTypeName)
			continue
		}

		tool := toolTypeImpl.CreateTool(mockToolName, "mock")

		err := tr.server.AddTool(ctx, tool, func(ctx context.Context, request server.ToolCallRequest) (interface{}, error) {
			response, err := toolTypeImpl.HandleRequest(ctx, request, "mock", tr.databaseUseCase)
			return FormatResponse(response, err)
		})

		if err != nil {
			log.Printf("Failed to register mock tool '%s': %v", mockToolName, err)
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
