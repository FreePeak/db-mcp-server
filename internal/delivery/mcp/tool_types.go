package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/FreePeak/cortex/pkg/tools"

	"github.com/FreePeak/db-mcp-server/internal/usecase"
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
	// DescribeTable returns column/index/row-count metadata for one table.
	DescribeTable(ctx context.Context, dbID, table string) (map[string]interface{}, error)
	// AnalyzePerformance reports tracked query metrics, slow queries, and
	// static SQL issue suggestions.
	AnalyzePerformance(ctx context.Context, dbID, action, query string, limit, thresholdMs int) (string, error)
	// HealthCheck reports connectivity, pool pressure, and engine stats.
	HealthCheck(ctx context.Context, dbID string) (map[string]interface{}, error)
	// RelationshipGraph renders the database's FK relationships as Mermaid.
	RelationshipGraph(ctx context.Context, dbID string) (string, error)
}

// maskedQueryProvider is implemented by use cases that can redact PII by
// content shape. Kept separate from UseCaseProvider so existing providers
// and test doubles stay source-compatible: the query tool falls back to
// ExecuteQuery when the capability is absent.
type maskedQueryProvider interface {
	ExecuteQueryMasked(ctx context.Context, dbID, query string, params []interface{}, mask bool) (string, error)
}

// dryRunCapable is implemented by use cases that can classify a statement
// offline. Optional, like maskedQueryProvider: a provider without it rejects
// dry_run explicitly rather than silently executing.
type dryRunCapable interface {
	ExecuteStatementDryRun(ctx context.Context, dbID, statement string) (*usecase.RiskReport, error)
}

// snapshotCapable is implemented by use cases that keep pre-mutation row
// captures, so an agent can undo a DELETE/UPDATE through the transaction
// tool. Optional: providers without it report the action as unsupported.
type snapshotCapable interface {
	ListSnapshots(dbID string) []usecase.MutationSnapshot
	RollbackSnapshot(ctx context.Context, dbID, snapshotID string) (string, error)
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
		tools.WithBoolean("mask_pii",
			tools.Description("Redact PII in the results by content shape (emails, phone numbers, card numbers, SSNs, IPs) as well as by configured column rules"),
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
		tools.WithBoolean("mask_pii",
			tools.Description("Redact PII in the results by content shape (emails, phone numbers, card numbers, SSNs, IPs) as well as by configured column rules"),
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

	// Absent or non-boolean mask_pii means "not requested"; the parameter is
	// additive only, so a malformed value can never disable masking that the
	// database config enforces.
	maskPII := false
	if requested, ok := request.Parameters["mask_pii"].(bool); ok {
		maskPII = requested
	}

	var result string
	var err error
	if masked, ok := useCase.(maskedQueryProvider); ok {
		result, err = masked.ExecuteQueryMasked(ctx, dbID, query, queryParams, maskPII)
	} else {
		result, err = useCase.ExecuteQuery(ctx, dbID, query, queryParams)
	}
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
		tools.WithBoolean("dry_run",
			tools.Description("Classify the statement offline and report its risk without executing anything"),
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
		tools.WithBoolean("dry_run",
			tools.Description("Classify the statement offline and report its risk without executing anything"),
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

	dryRun := false
	if requested, ok := request.Parameters["dry_run"].(bool); ok {
		dryRun = requested
	}
	if dryRun {
		capable, ok := useCase.(dryRunCapable)
		if !ok {
			return nil, fmt.Errorf("dry_run is not supported by this provider")
		}
		report, rerr := capable.ExecuteStatementDryRun(ctx, dbID, statement)
		if rerr != nil {
			return nil, rerr
		}
		return createTextResponse(formatRiskReport(report)), nil
	}

	result, err := useCase.ExecuteStatement(ctx, dbID, statement, statementParams)
	if err != nil {
		return nil, err
	}

	return createTextResponse(result), nil
}

// formatRiskReport renders a RiskReport as compact agent-readable text.
func formatRiskReport(r *usecase.RiskReport) string {
	var b strings.Builder
	b.WriteString("DRY RUN — nothing was executed.\n")
	fmt.Fprintf(&b, "Kind: %s  Risk: %s  Statements: %d\n",
		titleCase(r.Kind), titleCase(r.Risk), r.Statements)
	if r.MissingWhere {
		b.WriteString("Missing WHERE clause on an UPDATE/DELETE.\n")
	}
	if len(r.Notes) > 0 {
		b.WriteString("\nAdvisories:\n")
		for _, n := range r.Notes {
			fmt.Fprintf(&b, "- %s\n", n)
		}
	}
	b.WriteString("\nRe-run without dry_run to execute.")
	return b.String()
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
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
			tools.Description("Transaction action (begin, commit, rollback, execute, list_snapshots, rollback_snapshot)"),
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
		tools.WithString("snapshot_id",
			tools.Description("Snapshot ID to restore (required for the rollback_snapshot action)"),
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
			tools.Description("Transaction action (begin, commit, rollback, execute, list_snapshots, rollback_snapshot)"),
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
		tools.WithString("snapshot_id",
			tools.Description("Snapshot ID to restore (required for the rollback_snapshot action)"),
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

	// Snapshot actions are capability-detected so existing providers and test
	// doubles stay source-compatible.
	if action == "list_snapshots" || action == "rollback_snapshot" {
		sc, capable := useCase.(snapshotCapable)
		if !capable {
			return nil, fmt.Errorf("%s is not supported by this provider", action)
		}
		if action == "list_snapshots" {
			snaps := sc.ListSnapshots(dbID)
			var b strings.Builder
			fmt.Fprintf(&b, "Snapshots for %s: %d\n", dbID, len(snaps))
			for _, sn := range snaps {
				fmt.Fprintf(&b, "- %s  %s on %s (%d rows) at %s\n",
					sn.ID, sn.Kind, sn.Table, len(sn.Rows), sn.Timestamp.Format(time.RFC3339))
			}
			if len(snaps) == 0 {
				b.WriteString("(none yet — a DELETE or UPDATE through the execute tool records one)\n")
			}
			return createTextResponse(b.String()), nil
		}
		// Absent or non-string snapshot_id is reported below rather than
		// silently rolling back something else.
		snapshotID := ""
		if id, ok := request.Parameters["snapshot_id"].(string); ok {
			snapshotID = id
		}
		if snapshotID == "" {
			return nil, fmt.Errorf("snapshot_id parameter is required for rollback_snapshot")
		}
		msg, rerr := sc.RollbackSnapshot(ctx, dbID, snapshotID)
		if rerr != nil {
			return nil, rerr
		}
		return createTextResponse(msg), nil
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
			tools.Description("Action (stats, slow_queries, engine_slow_queries, suggest, suggest_indexes, workload_suggestions, index_health, db_health, reset)"),
			tools.Required(),
		),
		tools.WithString("query",
			tools.Description("SQL query to analyze (required for suggest and suggest_indexes actions)"),
		),
		tools.WithNumber("limit",
			tools.Description("Maximum number of results to return"),
		),
		tools.WithNumber("threshold",
			tools.Description("Slow query threshold in milliseconds; when > 0 it is applied before running the action"),
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
			tools.Description("Action (stats, slow_queries, engine_slow_queries, suggest, suggest_indexes, workload_suggestions, index_health, db_health, reset)"),
			tools.Required(),
		),
		tools.WithString("query",
			tools.Description("SQL query to analyze (required for suggest and suggest_indexes actions)"),
		),
		tools.WithNumber("limit",
			tools.Description("Maximum number of results to return"),
		),
		tools.WithNumber("threshold",
			tools.Description("Slow query threshold in milliseconds; when > 0 it is applied before running the action"),
		),
	)
}

// HandleRequest handles performance tool requests
func (t *PerformanceTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// If dbID is not provided, extract it from the tool name
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}

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

	output, err := useCase.AnalyzePerformance(ctx, dbID, action, query, limit, threshold)
	if err != nil {
		return nil, err
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
// DescribeTool implementation
//------------------------------------------------------------------------------

// DescribeTool handles per-table metadata inspection: columns, indexes,
// and row estimates.
type DescribeTool struct {
	BaseToolType
}

// NewDescribeTool creates a new describe tool type
func NewDescribeTool() *DescribeTool {
	return &DescribeTool{
		BaseToolType: BaseToolType{
			name:        "describe",
			description: "Show columns, indexes, and row estimate for a table",
		},
	}
}

// CreateTool creates a describe tool for a specific database
func (t *DescribeTool) CreateTool(name string, dbID string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetDescription(dbID)),
		tools.WithString("table",
			tools.Description("Table name to inspect (schema-qualified allowed, e.g. public.users)"),
			tools.Required(),
		),
	)
}

// CreateUnifiedTool creates a describe tool with a database parameter
func (t *DescribeTool) CreateUnifiedTool(name string, dbList []string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetUnifiedDescription(dbList)),
		tools.WithString("database",
			tools.Description(fmt.Sprintf("Database ID to use. Available: %s", strings.Join(dbList, ", "))),
			tools.Required(),
		),
		tools.WithString("table",
			tools.Description("Table name to inspect (schema-qualified allowed, e.g. public.users)"),
			tools.Required(),
		),
	)
}

// HandleRequest handles describe tool requests
func (t *DescribeTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}

	table, ok := request.Parameters["table"].(string)
	if !ok {
		return nil, fmt.Errorf("table parameter must be a string")
	}

	info, err := useCase.DescribeTable(ctx, dbID, table)
	if err != nil {
		return nil, err
	}
	return createTextResponse(formatDescribeResult(info)), nil
}

// mapString safely extracts a string field from a metadata map.
func mapString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// formatDescribeResult renders describe output as compact readable text.
func formatDescribeResult(info map[string]interface{}) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Table %v (database %v, engine %v)\n\n", info["table"], info["database"], info["dbType"])

	b.WriteString("Columns:\n")
	columns, _ := describeRows(info["columns"])
	for _, c := range columns {
		fmt.Fprintf(&b, "  %-28s %-20s nullable=%v default=%s\n",
			mapString(c, "column_name"), mapString(c, "data_type"),
			c["is_nullable"], mapString(c, "column_default"))
	}

	b.WriteString("\nIndexes:\n")
	indexes, _ := describeRows(info["indexes"])
	for _, ix := range indexes {
		name := mapString(ix, "index_name")
		def := mapString(ix, "definition")
		if def != "" {
			fmt.Fprintf(&b, "  %s — %s\n", name, def)
		} else {
			fmt.Fprintf(&b, "  %s\n", name)
		}
	}

	b.WriteString("\nConstraints:\n")
	constraints, _ := describeRows(info["constraints"])
	if len(constraints) == 0 {
		b.WriteString("  (none found)\n")
	}
	for _, c := range constraints {
		line := fmt.Sprintf("  %s %s (%s)",
			mapString(c, "constraint_type"), mapString(c, "constraint_name"), mapString(c, "column_name"))
		if refTable := mapString(c, "referenced_table"); refTable != "" {
			line += fmt.Sprintf(" -> %s(%s)", refTable, mapString(c, "referenced_column"))
		}
		b.WriteString(line + "\n")
	}

	if rc, ok := info["rowCount"].(string); ok && rc != "" {
		fmt.Fprintf(&b, "\nRows (estimate): %s\n", rc)
	}
	return b.String()
}

func describeRows(v interface{}) ([]map[string]interface{}, bool) {
	rows, ok := v.([]map[string]interface{})
	return rows, ok
}

//------------------------------------------------------------------------------

//------------------------------------------------------------------------------
// HealthTool implementation
//------------------------------------------------------------------------------

// HealthTool reports connectivity, connection-pool pressure, and
// best-effort engine statistics for a database.
type HealthTool struct {
	BaseToolType
}

// NewHealthTool creates a new health tool type
func NewHealthTool() *HealthTool {
	return &HealthTool{
		BaseToolType: BaseToolType{
			name:        "health",
			description: "Report connectivity, connection-pool state, and engine health statistics",
		},
	}
}

// CreateTool creates a health tool for a specific database
func (t *HealthTool) CreateTool(name string, dbID string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetDescription(dbID)),
	)
}

// CreateUnifiedTool creates a health tool with a database parameter
func (t *HealthTool) CreateUnifiedTool(name string, dbList []string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetUnifiedDescription(dbList)),
		tools.WithString("database",
			tools.Description(fmt.Sprintf("Database ID to use. Available: %s", strings.Join(dbList, ", "))),
			tools.Required(),
		),
	)
}

// HandleRequest handles health tool requests
func (t *HealthTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}

	info, err := useCase.HealthCheck(ctx, dbID)
	if err != nil {
		return nil, err
	}
	return createTextResponse(formatHealthResult(info)), nil
}

// formatHealthResult renders the health payload as compact readable text.
func formatHealthResult(info map[string]interface{}) string {
	var b strings.Builder
	status := "healthy"
	if s, ok := info["healthy"].(bool); ok && !s {
		status = "UNHEALTHY"
	}
	fmt.Fprintf(&b, "Database %v: %s\n", info["database"], status)
	if errMsg, ok := info["error"].(string); ok {
		fmt.Fprintf(&b, "Error: %s\n", errMsg)
		return b.String()
	}
	if ping, ok := info["ping_ms"].(float64); ok {
		fmt.Fprintf(&b, "Ping: %.2f ms\n", ping)
	}
	for _, key := range []string{"read_only", "max_rows", "statement_timeout_seconds"} {
		if v, ok := info[key]; ok {
			fmt.Fprintf(&b, "%v: %v\n", key, v)
		}
	}
	for _, key := range []string{"pool_open_connections", "pool_in_use", "pool_idle", "pool_wait_count", "pool_wait_duration_ms"} {
		if v, ok := info[key]; ok {
			fmt.Fprintf(&b, "%v: %v\n", key, v)
		}
	}
	for _, key := range []string{"buffer_cache_hit_ratio_pct", "buffer_cache_miss_ratio_pct", "engine_stats_error"} {
		if v, ok := info[key].(string); ok && v != "" {
			fmt.Fprintf(&b, "%v: %s\n", key, v)
		}
	}
	return b.String()
}

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
		tools.WithString("format",
			tools.Description(`Output format: "list" (default, table listing) or "mermaid" (entity-relationship diagram of foreign-key relationships)`),
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
		tools.WithString("format",
			tools.Description(`Output format: "list" (default, table listing) or "mermaid" (entity-relationship diagram of foreign-key relationships)`),
		),
	)
}

// HandleRequest handles schema tool requests
func (t *SchemaTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// If dbID is not provided, extract it from the tool name
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}

	// format=mermaid renders the entity-relationship graph instead of a
	// plain table listing.
	if format, ok := request.Parameters["format"].(string); ok && format == "mermaid" {
		graph, err := useCase.RelationshipGraph(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(graph), nil
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
	factory.Register(NewDescribeTool())
	factory.Register(NewHealthTool())
	factory.Register(NewSchemaTool())
	factory.Register(NewListDatabasesTool())
	factory.Register(NewFilterTablesTool())

	return factory
}

// Register adds a tool type to the factory
func (f *ToolTypeFactory) Register(toolType ToolType) {
	f.toolTypes[toolType.GetName()] = toolType
}

// GetToolType returns a tool type by name
func (f *ToolTypeFactory) GetToolType(name string) (ToolType, bool) {
	// Exact lookup first: names such as "list_databases" must resolve to
	// their own tool type instead of being shadowed by a prefix match
	// (e.g. splitting on "_" would wrongly map "list_databases" to "list").
	if toolType, ok := f.toolTypes[name]; ok {
		return toolType, true
	}

	// Fall back to <tooltype>_<dbID> format, using the first segment.
	parts := strings.Split(name, "_")
	if len(parts) > 0 {
		if toolType, ok := f.toolTypes[parts[0]]; ok {
			return toolType, true
		}
	}

	return nil, false
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
