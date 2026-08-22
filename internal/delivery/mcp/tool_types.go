package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/FreePeak/cortex/pkg/tools"
)

// createTextResponse creates a simple response with a text content
func createTextResponse(text string) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": text,
			},
		},
	}
}

// addMetadata adds metadata to a response
func addMetadata(resp map[string]interface{}, key string, value interface{}) map[string]interface{} {
	if resp["metadata"] == nil {
		resp["metadata"] = make(map[string]interface{})
	}

	metadata, ok := resp["metadata"].(map[string]interface{})
	if !ok {
		// Create a new metadata map if conversion fails
		metadata = make(map[string]interface{})
		resp["metadata"] = metadata
	}

	metadata[key] = value
	return resp
}

// TODO: Refactor tool type implementations to reduce duplication and improve maintainability
// TODO: Consider using a code generation approach for repetitive tool patterns
// TODO: Add comprehensive request validation for all tool parameters
// TODO: Implement proper rate limiting and resource protection
// TODO: Add detailed documentation for each tool type and its parameters

// ToolType interface defines the structure for different types of database tools
type ToolType interface {
	// GetName returns the base name of the tool type (e.g., "query", "execute")
	GetName() string

	// GetDescription returns a description for this tool type
	GetDescription(dbID string) string

	// CreateTool creates a tool with the specified name
	// The returned tool must be compatible with server.MCPServer.AddTool's first parameter
	CreateTool(name string, dbID string) interface{}

	// CreateUnifiedTool creates a unified tool with a database parameter instead of per-database tools
	CreateUnifiedTool(name string, dbList []string) interface{}

	// HandleRequest handles tool requests for this tool type
	HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error)
}

// UseCaseProvider interface abstracts database use case operations
type UseCaseProvider interface {
	ExecuteQuery(ctx context.Context, dbID, query string, params []interface{}) (string, error)
	ExecuteStatement(ctx context.Context, dbID, statement string, params []interface{}) (string, error)
	ExecuteTransaction(ctx context.Context, dbID, action string, txID string, statement string, params []interface{}, readOnly bool) (string, map[string]interface{}, error)
	GetDatabaseInfo(dbID string) (map[string]interface{}, error)
	ListDatabases() []string
	GetDatabaseType(dbID string) (string, error)
	IsLazyLoading() bool
	// ExecuteExplain returns the engine's execution plan for a statement.
	ExecuteExplain(ctx context.Context, dbID, statement string, analyze bool) (string, error)
}

// BaseToolType provides common functionality for tool types
type BaseToolType struct {
	name        string
	description string
}

// GetName returns the name of the tool type
func (b *BaseToolType) GetName() string {
	return b.name
}

// GetDescription returns a description for the tool type
func (b *BaseToolType) GetDescription(dbID string) string {
	return fmt.Sprintf("%s on %s database", b.description, dbID)
}

// GetUnifiedDescription returns a description for unified mode with available databases listed
func (b *BaseToolType) GetUnifiedDescription(dbList []string) string {
	return fmt.Sprintf("%s on specified database. Available databases: %s",
		b.description, strings.Join(dbList, ", "))
}

//------------------------------------------------------------------------------
// QueryTool implementation
//------------------------------------------------------------------------------

// QueryTool handles SQL query operations
type QueryTool struct {
	BaseToolType
}

// NewQueryTool creates a new query tool type
func NewQueryTool() *QueryTool {
	return &QueryTool{
		BaseToolType: BaseToolType{
			name:        "query",
			description: "Execute SQL query",
		},
	}
}

// CreateTool creates a query tool
func (t *QueryTool) CreateTool(name string, dbID string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetDescription(dbID)),
		tools.WithString("query",
			tools.Description("SQL query to execute"),
			tools.Required(),
		),
		tools.WithArray("params",
			tools.Description("Query parameters"),
			tools.Items(map[string]interface{}{"type": "string"}),
		),
	)
}

// CreateUnifiedTool creates a unified query tool with database parameter
func (t *QueryTool) CreateUnifiedTool(name string, dbList []string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetUnifiedDescription(dbList)),
		tools.WithString("database",
			tools.Description(fmt.Sprintf("Database ID to use. Available: %s", strings.Join(dbList, ", "))),
			tools.Required(),
		),
		tools.WithString("query",
			tools.Description("SQL query to execute"),
			tools.Required(),
		),
		tools.WithArray("params",
			tools.Description("Query parameters"),
			tools.Items(map[string]interface{}{"type": "string"}),
		),
	)
}

// HandleRequest handles query tool requests
func (t *QueryTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// If dbID is not provided, extract it from the tool name
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}

	query, ok := request.Parameters["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter must be a string")
	}

	var queryParams []interface{}
	if request.Parameters["params"] != nil {
		if paramsArr, ok := request.Parameters["params"].([]interface{}); ok {
			queryParams = paramsArr
		}
	}

	result, err := useCase.ExecuteQuery(ctx, dbID, query, queryParams)
	if err != nil {
		return nil, err
	}

	return createTextResponse(result), nil
}

// extractDatabaseIDFromName extracts the database ID from a tool name
func extractDatabaseIDFromName(name string) string {
	// Format is: <tooltype>_<dbID>
	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return ""
	}

	// The database ID is the last part
	return parts[len(parts)-1]
}

//------------------------------------------------------------------------------
// ExecuteTool implementation
//------------------------------------------------------------------------------

// ExecuteTool handles SQL statement execution
type ExecuteTool struct {
	BaseToolType
}

// NewExecuteTool creates a new execute tool type
func NewExecuteTool() *ExecuteTool {
	return &ExecuteTool{
		BaseToolType: BaseToolType{
			name:        "execute",
			description: "Execute SQL statement",
		},
	}
}

// CreateTool creates an execute tool
func (t *ExecuteTool) CreateTool(name string, dbID string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetDescription(dbID)),
		tools.WithString("statement",
			tools.Description("SQL statement to execute (INSERT, UPDATE, DELETE, etc.)"),
			tools.Required(),
		),
		tools.WithArray("params",
			tools.Description("Statement parameters"),
			tools.Items(map[string]interface{}{"type": "string"}),
		),
	)
}

// CreateUnifiedTool creates a unified execute tool with database parameter
func (t *ExecuteTool) CreateUnifiedTool(name string, dbList []string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetUnifiedDescription(dbList)),
		tools.WithString("database",
			tools.Description(fmt.Sprintf("Database ID to use. Available: %s", strings.Join(dbList, ", "))),
			tools.Required(),
		),
		tools.WithString("statement",
			tools.Description("SQL statement to execute (INSERT, UPDATE, DELETE, etc.)"),
			tools.Required(),
		),
		tools.WithArray("params",
			tools.Description("Statement parameters"),
			tools.Items(map[string]interface{}{"type": "string"}),
		),
	)
}

// HandleRequest handles execute tool requests
func (t *ExecuteTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// If dbID is not provided, extract it from the tool name
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}

	statement, ok := request.Parameters["statement"].(string)
	if !ok {
		return nil, fmt.Errorf("statement parameter must be a string")
	}

	var statementParams []interface{}
	if request.Parameters["params"] != nil {
		if paramsArr, ok := request.Parameters["params"].([]interface{}); ok {
			statementParams = paramsArr
		}
	}

	result, err := useCase.ExecuteStatement(ctx, dbID, statement, statementParams)
	if err != nil {
		return nil, err
	}

	return createTextResponse(result), nil
}

//------------------------------------------------------------------------------
// TransactionTool implementation
//------------------------------------------------------------------------------

// TransactionTool handles database transactions
type TransactionTool struct {
	BaseToolType
}

// NewTransactionTool creates a new transaction tool type
func NewTransactionTool() *TransactionTool {
	return &TransactionTool{
		BaseToolType: BaseToolType{
			name:        "transaction",
			description: "Manage transactions",
		},
	}
}

// CreateTool creates a transaction tool
func (t *TransactionTool) CreateTool(name string, dbID string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetDescription(dbID)),
		tools.WithString("action",
			tools.Description("Transaction action (begin, commit, rollback, execute)"),
			tools.Required(),
		),
		tools.WithString("transactionId",
			tools.Description("Transaction ID (required for commit, rollback, execute)"),
		),
		tools.WithString("statement",
			tools.Description("SQL statement to execute within transaction (required for execute)"),
		),
		tools.WithArray("params",
			tools.Description("Statement parameters"),
			tools.Items(map[string]interface{}{"type": "string"}),
		),
		tools.WithBoolean("readOnly",
			tools.Description("Whether the transaction is read-only (for begin)"),
		),
	)
}

// CreateUnifiedTool creates a unified transaction tool with database parameter
func (t *TransactionTool) CreateUnifiedTool(name string, dbList []string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetUnifiedDescription(dbList)),
		tools.WithString("database",
			tools.Description(fmt.Sprintf("Database ID to use. Available: %s", strings.Join(dbList, ", "))),
			tools.Required(),
		),
		tools.WithString("action",
			tools.Description("Transaction action (begin, commit, rollback, execute)"),
			tools.Required(),
		),
		tools.WithString("transactionId",
			tools.Description("Transaction ID (required for commit, rollback, execute)"),
		),
		tools.WithString("statement",
			tools.Description("SQL statement to execute within transaction (required for execute)"),
		),
		tools.WithArray("params",
			tools.Description("Statement parameters"),
			tools.Items(map[string]interface{}{"type": "string"}),
		),
		tools.WithBoolean("readOnly",
			tools.Description("Whether the transaction is read-only (for begin)"),
		),
	)
}

// HandleRequest handles transaction tool requests
func (t *TransactionTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// If dbID is not provided, extract it from the tool name
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}

	action, ok := request.Parameters["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action parameter must be a string")
	}

	txID := ""
	if request.Parameters["transactionId"] != nil {
		var ok bool
		txID, ok = request.Parameters["transactionId"].(string)
		if !ok {
			return nil, fmt.Errorf("transactionId parameter must be a string")
		}
	}

	statement := ""
	if request.Parameters["statement"] != nil {
		var ok bool
		statement, ok = request.Parameters["statement"].(string)
		if !ok {
			return nil, fmt.Errorf("statement parameter must be a string")
		}
	}

	var params []interface{}
	if request.Parameters["params"] != nil {
		if paramsArr, ok := request.Parameters["params"].([]interface{}); ok {
			params = paramsArr
		}
	}

	readOnly := false
	if request.Parameters["readOnly"] != nil {
		var ok bool
		readOnly, ok = request.Parameters["readOnly"].(bool)
		if !ok {
			return nil, fmt.Errorf("readOnly parameter must be a boolean")
		}
	}

	message, metadata, err := useCase.ExecuteTransaction(ctx, dbID, action, txID, statement, params, readOnly)
	if err != nil {
		return nil, err
	}

	// Create response with text and metadata
	resp := createTextResponse(message)

	// Add metadata if provided
	for k, v := range metadata {
		addMetadata(resp, k, v)
	}

	return resp, nil
}

//------------------------------------------------------------------------------
// PerformanceTool implementation
//------------------------------------------------------------------------------

// PerformanceTool handles query performance analysis
type PerformanceTool struct {
	BaseToolType
}

// NewPerformanceTool creates a new performance tool type
func NewPerformanceTool() *PerformanceTool {
	return &PerformanceTool{
		BaseToolType: BaseToolType{
			name:        "performance",
			description: "Analyze query performance",
		},
	}
}

// CreateTool creates a performance analysis tool
func (t *PerformanceTool) CreateTool(name string, dbID string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetDescription(dbID)),
		tools.WithString("action",
			tools.Description("Action (getSlowQueries, getMetrics, analyzeQuery, reset, setThreshold)"),
			tools.Required(),
		),
		tools.WithString("query",
			tools.Description("SQL query to analyze (required for analyzeQuery)"),
		),
		tools.WithNumber("limit",
			tools.Description("Maximum number of results to return"),
		),
		tools.WithNumber("threshold",
			tools.Description("Slow query threshold in milliseconds (required for setThreshold)"),
		),
	)
}

// CreateUnifiedTool creates a unified performance tool with database parameter
func (t *PerformanceTool) CreateUnifiedTool(name string, dbList []string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetUnifiedDescription(dbList)),
		tools.WithString("database",
			tools.Description(fmt.Sprintf("Database ID to use. Available: %s", strings.Join(dbList, ", "))),
			tools.Required(),
		),
		tools.WithString("action",
			tools.Description("Action (getSlowQueries, getMetrics, analyzeQuery, reset, setThreshold)"),
			tools.Required(),
		),
		tools.WithString("query",
			tools.Description("SQL query to analyze (required for analyzeQuery)"),
		),
		tools.WithNumber("limit",
			tools.Description("Maximum number of results to return"),
		),
		tools.WithNumber("threshold",
			tools.Description("Slow query threshold in milliseconds (required for setThreshold)"),
		),
	)
}

// HandleRequest handles performance tool requests
func (t *PerformanceTool) HandleRequest(_ context.Context, request server.ToolCallRequest, dbID string, _ UseCaseProvider) (interface{}, error) {
	// If dbID is not provided, extract it from the tool name
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}

	// This is a simplified implementation
	// In a real implementation, this would analyze query performance

	action, ok := request.Parameters["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action parameter must be a string")
	}

	var limit int
	if request.Parameters["limit"] != nil {
		if limitParam, ok := request.Parameters["limit"].(float64); ok {
			limit = int(limitParam)
		}
	}

	query := ""
	if request.Parameters["query"] != nil {
		var ok bool
		query, ok = request.Parameters["query"].(string)
		if !ok {
			return nil, fmt.Errorf("query parameter must be a string")
		}
	}

	var threshold int
	if request.Parameters["threshold"] != nil {
		if thresholdParam, ok := request.Parameters["threshold"].(float64); ok {
			threshold = int(thresholdParam)
		}
	}

	// This is where we would call the useCase to analyze performance
	// For now, just return a placeholder
	output := fmt.Sprintf("Performance analysis for action '%s' on database '%s'\n", action, dbID)

	if query != "" {
		output += fmt.Sprintf("Query: %s\n", query)
	}

	if limit > 0 {
		output += fmt.Sprintf("Limit: %d\n", limit)
	}

	if threshold > 0 {
		output += fmt.Sprintf("Threshold: %d ms\n", threshold)
	}

	return createTextResponse(output), nil
}

//------------------------------------------------------------------------------
// ExplainTool implementation
//------------------------------------------------------------------------------

// ExplainTool handles execution-plan analysis without (normally) running
// the statement.
type ExplainTool struct {
	BaseToolType
}

// NewExplainTool creates a new explain tool type
func NewExplainTool() *ExplainTool {
	return &ExplainTool{
		BaseToolType: BaseToolType{
			name:        "explain",
			description: "Show the database execution plan for a SQL statement",
		},
	}
}

// CreateTool creates an explain tool for a specific database
func (t *ExplainTool) CreateTool(name string, dbID string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetDescription(dbID)),
		tools.WithString("statement",
			tools.Description("SQL statement to explain (SELECT, INSERT, UPDATE, or DELETE)"),
			tools.Required(),
		),
		tools.WithBoolean("analyze",
			tools.Description("Execute the statement and include real timing/buffer statistics (writes still refused on read-only databases)"),
		),
	)
}

// CreateUnifiedTool creates an explain tool with a database parameter
func (t *ExplainTool) CreateUnifiedTool(name string, dbList []string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetUnifiedDescription(dbList)),
		tools.WithString("database",
			tools.Description(fmt.Sprintf("Database ID to use. Available: %s", strings.Join(dbList, ", "))),
			tools.Required(),
		),
		tools.WithString("statement",
			tools.Description("SQL statement to explain (SELECT, INSERT, UPDATE, or DELETE)"),
			tools.Required(),
		),
		tools.WithBoolean("analyze",
			tools.Description("Execute the statement and include real timing/buffer statistics (writes still refused on read-only databases)"),
		),
	)
}

// HandleRequest handles explain tool requests
func (t *ExplainTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}

	statement, ok := request.Parameters["statement"].(string)
	if !ok {
		return nil, fmt.Errorf("statement parameter must be a string")
	}

	analyze := false
	if request.Parameters["analyze"] != nil {
		if b, ok := request.Parameters["analyze"].(bool); ok {
			analyze = b
		}
	}

	result, err := useCase.ExecuteExplain(ctx, dbID, statement, analyze)
	if err != nil {
		return nil, err
	}
	return createTextResponse(result), nil
}

//------------------------------------------------------------------------------
// SchemaTool implementation
//------------------------------------------------------------------------------

// SchemaTool handles database schema exploration
type SchemaTool struct {
	BaseToolType
}

// NewSchemaTool creates a new schema tool type
func NewSchemaTool() *SchemaTool {
	return &SchemaTool{
		BaseToolType: BaseToolType{
			name:        "schema",
			description: "Get schema of",
		},
	}
}

// CreateTool creates a schema tool
func (t *SchemaTool) CreateTool(name string, dbID string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetDescription(dbID)),
		// Use any string parameter for compatibility
		tools.WithString("random_string",
			tools.Description("Dummy parameter (optional)"),
		),
	)
}

// CreateUnifiedTool creates a unified schema tool with database parameter
func (t *SchemaTool) CreateUnifiedTool(name string, dbList []string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetUnifiedDescription(dbList)),
		tools.WithString("database",
			tools.Description(fmt.Sprintf("Database ID to use. Available: %s", strings.Join(dbList, ", "))),
			tools.Required(),
		),
		tools.WithString("random_string",
			tools.Description("Dummy parameter (optional)"),
		),
	)
}

// HandleRequest handles schema tool requests
func (t *SchemaTool) HandleRequest(_ context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// If dbID is not provided, extract it from the tool name
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}

	info, err := useCase.GetDatabaseInfo(dbID)
	if err != nil {
		return nil, err
	}

	// Format response text
	infoStr := fmt.Sprintf("Database Schema for %s:\n\n%+v", dbID, info)
	return createTextResponse(infoStr), nil
}

//------------------------------------------------------------------------------
// ListDatabasesTool implementation
//------------------------------------------------------------------------------

// ListDatabasesTool handles listing available databases
type ListDatabasesTool struct {
	BaseToolType
}

// NewListDatabasesTool creates a new list databases tool type
func NewListDatabasesTool() *ListDatabasesTool {
	return &ListDatabasesTool{
		BaseToolType: BaseToolType{
			name:        "list_databases",
			description: "List all available databases",
		},
	}
}

// CreateTool creates a list databases tool
func (t *ListDatabasesTool) CreateTool(name string, dbID string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetDescription(dbID)),
		// Use any string parameter for compatibility
		tools.WithString("random_string",
			tools.Description("Dummy parameter (optional)"),
		),
	)
}

// CreateUnifiedTool creates a unified list databases tool (same as regular, no database parameter needed)
func (t *ListDatabasesTool) CreateUnifiedTool(name string, _ []string) interface{} {
	return t.CreateTool(name, "")
}

// HandleRequest handles list databases tool requests
func (t *ListDatabasesTool) HandleRequest(_ context.Context, _ server.ToolCallRequest, _ string, useCase UseCaseProvider) (interface{}, error) {
	databases := useCase.ListDatabases()

	// Format as text for display
	output := "Available databases:\n\n"
	for i, db := range databases {
		output += fmt.Sprintf("%d. %s\n", i+1, db)
	}

	if len(databases) == 0 {
		output += "No databases configured.\n"
	}

	return createTextResponse(output), nil
}

//------------------------------------------------------------------------------
// ToolTypeFactory provides a factory for creating tool types
//------------------------------------------------------------------------------

// ToolTypeFactory creates and manages tool types
type ToolTypeFactory struct {
	toolTypes map[string]ToolType
}

// NewToolTypeFactory creates a new tool type factory with all registered tool types
func NewToolTypeFactory() *ToolTypeFactory {
	factory := &ToolTypeFactory{
		toolTypes: make(map[string]ToolType),
	}

	// Register all tool types
	factory.Register(NewQueryTool())
	factory.Register(NewExecuteTool())
	factory.Register(NewTransactionTool())
	factory.Register(NewPerformanceTool())
	factory.Register(NewExplainTool())
	factory.Register(NewSchemaTool())
	factory.Register(NewListDatabasesTool())
	factory.Register(NewListDirectoryTool())
	factory.Register(NewFilterTablesTool())

	return factory
}

// Register adds a tool type to the factory
func (f *ToolTypeFactory) Register(toolType ToolType) {
	f.toolTypes[toolType.GetName()] = toolType
}

// GetToolType returns a tool type by name
func (f *ToolTypeFactory) GetToolType(name string) (ToolType, bool) {
	// Handle new simpler format: <tooltype>_<dbID> or just the tool type name
	parts := strings.Split(name, "_")
	if len(parts) > 0 {
		// First part is the tool type name
		toolType, ok := f.toolTypes[parts[0]]
		if ok {
			return toolType, true
		}
	}

	// Direct tool type lookup
	toolType, ok := f.toolTypes[name]
	return toolType, ok
}

// GetToolTypeForSourceName finds the appropriate tool type for a source name
func (f *ToolTypeFactory) GetToolTypeForSourceName(sourceName string) (ToolType, string, bool) {
	// Handle simpler format: <tooltype>_<dbID>
	parts := strings.Split(sourceName, "_")

	if len(parts) >= 2 {
		// First part is tool type, last part is dbID
		toolTypeName := parts[0]
		dbID := parts[len(parts)-1]

		toolType, ok := f.toolTypes[toolTypeName]
		if ok {
			return toolType, dbID, true
		}
	}

	// Handle case for global tools
	if sourceName == "list_databases" {
		toolType, ok := f.toolTypes["list_databases"]
		return toolType, "", ok
	}

	return nil, "", false
}

// GetAllToolTypes returns all registered tool types
func (f *ToolTypeFactory) GetAllToolTypes() []ToolType {
	types := make([]ToolType, 0, len(f.toolTypes))
	for _, toolType := range f.toolTypes {
		types = append(types, toolType)
	}
	return types
}

//------------------------------------------------------------------------------
// FilterTablesTool implementation
//------------------------------------------------------------------------------

// FilterTablesTool returns the list of tables whose names match a substring
// pattern. It is the substring-search equivalent of mcp-alchemy's
// filter_table_names and resolves FreePeak/db-mcp-server issue #54: callers
// with many tables (WordPress wp_*, Drupal pre_*, etc.) no longer have to
// retrieve the full schema list and then filter client-side.
type FilterTablesTool struct {
	BaseToolType
}

// NewFilterTablesTool creates a new filter_tables tool type
func NewFilterTablesTool() *FilterTablesTool {
	return &FilterTablesTool{
		BaseToolType: BaseToolType{
			name:        "filter_tables",
			description: "Find tables by substring match",
		},
	}
}

// CreateTool creates a per-database filter_tables tool
func (t *FilterTablesTool) CreateTool(name string, dbID string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetDescription(dbID)),
		tools.WithString("pattern",
			tools.Description("Case-insensitive substring to match table names against"),
			tools.Required(),
		),
	)
}

// CreateUnifiedTool creates a unified filter_tables tool with a database parameter
func (t *FilterTablesTool) CreateUnifiedTool(name string, dbList []string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetUnifiedDescription(dbList)),
		tools.WithString("database",
			tools.Description(fmt.Sprintf("Database ID to use. Available: %s", strings.Join(dbList, ", "))),
			tools.Required(),
		),
		tools.WithString("pattern",
			tools.Description("Case-insensitive substring to match table names against"),
			tools.Required(),
		),
	)
}

// HandleRequest handles the filter_tables request: it pulls the database
// schema, filters the tables whose names contain the pattern (case
// insensitive), and returns a small text response with the matched names.
func (t *FilterTablesTool) HandleRequest(_ context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}

	pattern, _ := request.Parameters["pattern"].(string) //nolint:errcheck // type assertion; missing pattern is handled below
	if pattern == "" {
		return nil, fmt.Errorf("pattern parameter is required")
	}

	info, err := useCase.GetDatabaseInfo(dbID)
	if err != nil {
		return nil, err
	}

	tablesRaw, ok := info["tables"].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("database %q returned unexpected schema format", dbID)
	}

	lower := strings.ToLower(pattern)
	var matched []string
	for _, tbl := range tablesRaw {
		// Try common column names that may carry the table name.
		for _, key := range []string{"table_name", "tablename", "TABLE_NAME", "name"} {
			if v, ok := tbl[key].(string); ok {
				if strings.Contains(strings.ToLower(v), lower) {
					matched = append(matched, v)
				}
				break
			}
		}
	}

	output := fmt.Sprintf("Tables matching %q in %s:\n\n", pattern, dbID)
	if len(matched) == 0 {
		output += "(no matches)"
	} else {
		for _, name := range matched {
			output += fmt.Sprintf("- %s\n", name)
		}
	}
	output += fmt.Sprintf("\nMatched %d of %d tables.", len(matched), len(tablesRaw))

	return createTextResponse(output), nil
}
