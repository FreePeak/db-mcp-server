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

	// GenerateSchemaCode renders the schema as application code
	// (target: "go" structs or "typescript" interfaces).
	GenerateSchemaCode(ctx context.Context, dbID, target string) (string, error)
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
			tools.Description("Mask PII in results (emails, phones, cards, SSNs, IPs)"),
		),
		tools.WithString("verbosity",
			tools.Description("Result size: full (default), normal (cells truncated at 500 chars with …(+N) markers), minimal (row count + first row preview — ideal for write confirmations/polling)"),
		),
		tools.WithString("format",
			tools.Description(`Output format: text (default, human-readable table), csv (RFC4180), json (array of row objects), or inserts (INSERT INTO statements for the queried table)`),
		),
		tools.WithBoolean("count_only",
			tools.Description("Return the row COUNT(*) for the statement instead of rows"),
		),
		tools.WithNumber("timeout_ms",
			tools.Description("Cancel the query if it exceeds this many milliseconds"),
		),
		tools.WithNumber("sample_rows",
			tools.Description("Return N randomly ordered rows instead of running the query as written (engine-aware ORDER BY)"),
		),
		tools.WithNumber("page",
			tools.Description("1-based page number; requires page_size; returns data plus total matching rows"),
		),
		tools.WithNumber("page_size",
			tools.Description("Rows per page when paging (default 50)"),
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
			tools.Description("Mask PII in results (emails, phones, cards, SSNs, IPs)"),
		),
		tools.WithString("verbosity",
			tools.Description("Result size: full (default), normal (cells truncated at 500 chars with …(+N) markers), minimal (row count + first row preview — ideal for write confirmations/polling)"),
		),
		tools.WithString("format",
			tools.Description(`Output format: text (default, human-readable table), csv (RFC4180), json (array of row objects), or inserts (INSERT INTO statements for the queried table)`),
		),
		tools.WithBoolean("count_only",
			tools.Description("Return the row COUNT(*) for the statement instead of rows"),
		),
		tools.WithNumber("timeout_ms",
			tools.Description("Cancel the query if it exceeds this many milliseconds"),
		),
		tools.WithNumber("sample_rows",
			tools.Description("Return N randomly ordered rows instead of running the query as written (engine-aware ORDER BY)"),
		),
		tools.WithNumber("page",
			tools.Description("1-based page number; requires page_size; returns data plus total matching rows"),
		),
		tools.WithNumber("page_size",
			tools.Description("Rows per page when paging (default 50)"),
		),
	)
}

// queryExportUseCase is implemented by use cases that support machine-
// readable export formats; detection keeps existing mocks and alternate
// providers compatible.
type queryExportUseCase interface {
	ExecuteQueryFormat(ctx context.Context, dbID, query string, params []interface{}, format string) (string, error)
}

// duplicateDetectionUseCase is implemented by use cases that can report
// duplicated values in one column.
type duplicateDetectionUseCase interface {
	FindDuplicates(ctx context.Context, dbID, table, column string) (string, error)
}

// sampleQueryUseCase is implemented by use cases that can draw N random
// rows from a statement with engine-appropriate ordering.
type sampleQueryUseCase interface {
	ExecuteQuerySample(ctx context.Context, dbID, query string, params []interface{}, n int) (string, error)
}

// pagedQueryUseCase is implemented by use cases that can window a SELECT
// into a page with total count.
type pagedQueryUseCase interface {
	ExecuteQueryPage(ctx context.Context, dbID, query string, params []interface{}, page, pageSize int) (string, int64, error)
}

// relatedRowsUseCase is implemented by use cases that can traverse
// foreign keys for one row.
type relatedRowsUseCase interface {
	RelatedRows(ctx context.Context, dbID, table, keyValue string) (string, error)
}

// valueSearchUseCase is implemented by use cases that can locate a literal
// across every textual column of a database.
type valueSearchUseCase interface {
	SearchValues(ctx context.Context, dbID, needle string) (string, error)
}

// columnProfilingUseCase is implemented by use cases that can compute a
// single-column statistical profile.
type columnProfilingUseCase interface {
	ProfileColumn(ctx context.Context, dbID, table, column string) (string, error)
}

// timeoutQueryUseCase is implemented by use cases that support a per-query
// deadline.
type timeoutQueryUseCase interface {
	ExecuteQueryWithTimeout(ctx context.Context, dbID, query string, params []interface{}, timeoutMs int) (string, error)
}

// rowCountPreviewUseCase is implemented by use cases that can price a
// SELECT via a COUNT(*) wrap without fetching rows.
type rowCountPreviewUseCase interface {
	CountQueryRows(ctx context.Context, dbID, query string, params []interface{}) (string, error)
}

// sessionObservabilityUseCase is implemented by use cases that can list
// active engine sessions and cancel running queries.
type sessionObservabilityUseCase interface {
	ListActiveSessions(ctx context.Context, dbID string) (string, error)
	ListBlockingWaits(ctx context.Context, dbID string) (string, error)
	CancelQuery(ctx context.Context, dbID string, sessionID int64) (string, error)
}

// schemaCompareUseCase is implemented by use cases that can structurally
// diff two databases' schemas.
type schemaCompareUseCase interface {
	CompareSchemas(ctx context.Context, dbIDA, dbIDB string) (string, error)
}

// dataCompareUseCase is implemented by use cases that can compare table
// row counts between two databases.
type dataCompareUseCase interface {
	CompareTableCounts(ctx context.Context, dbIDA, dbIDB string) (string, error)
	CompareTableSamples(ctx context.Context, dbIDA, dbIDB, table string, limit int) (string, error)
}

// piIMaskingUseCase is implemented by use cases that support opt-in PII
// masking; detection keeps existing mocks and alternate providers compatible.
type piIMaskingUseCase interface {
	ExecuteQueryMasked(ctx context.Context, dbID, query string, params []interface{}, mask bool, verbosity usecase.ResultVerbosity) (string, error)
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

	maskPII := false
	if v, ok := request.Parameters["mask_pii"].(bool); ok {
		maskPII = v
	}

	verbosity := usecase.VerbosityFull
	if v, ok := request.Parameters["verbosity"].(string); ok {
		switch usecase.ResultVerbosity(v) {
		case usecase.VerbosityMinimal, usecase.VerbosityNormal:
			verbosity = usecase.ResultVerbosity(v)
		}
	}
	// Random sampling draws N arbitrary rows.
	if n, ok := request.Parameters["sample_rows"].(float64); ok && int(n) > 0 {
		if sq, can := useCase.(sampleQueryUseCase); can {
			result, err := sq.ExecuteQuerySample(ctx, dbID, query, queryParams, int(n))
			if err != nil {
				return nil, err
			}
			return createTextResponse(result), nil
		}
	}

	// Pagination windows the result and reports the total in one call.
	page, hasPage := request.Parameters["page"].(float64)
	pageSize, hasSize := request.Parameters["page_size"].(float64)
	if hasPage && int(page) > 0 || hasSize && int(pageSize) > 0 {
		if pq, can := useCase.(pagedQueryUseCase); can {
			result, _, err := pq.ExecuteQueryPage(ctx, dbID, query, queryParams,
				int(page), int(pageSize))
			if err != nil {
				return nil, err
			}
			return createTextResponse(result), nil
		}
	}

	// count_only prices the statement instead of fetching rows.
	if countOnly, ok := request.Parameters["count_only"].(bool); ok && countOnly {
		if c, can := useCase.(rowCountPreviewUseCase); can {
			result, err := c.CountQueryRows(ctx, dbID, query, queryParams)
			if err != nil {
				return nil, err
			}
			return createTextResponse(result), nil
		}
	}

	// Export formats bypass the text renderer entirely.
	if format, _ := request.Parameters["format"].(string); format == "csv" || format == "json" || format == "inserts" { //nolint:errcheck // absent means text
		if x, canExport := useCase.(queryExportUseCase); canExport {
			result, err := x.ExecuteQueryFormat(ctx, dbID, query, queryParams, format)
			if err != nil {
				return nil, err
			}
			return createTextResponse(result), nil
		}
	}

	// Per-query deadline, when requested and supported.
	if tm, ok := request.Parameters["timeout_ms"].(float64); ok && int(tm) > 0 {
		if tq, can := useCase.(timeoutQueryUseCase); can {
			if m, canMask := useCase.(piIMaskingUseCase); canMask {
				result, err := func() (string, error) {
					tctx, cancel := context.WithTimeout(ctx, time.Duration(int(tm))*time.Millisecond)
					defer cancel()
					return m.ExecuteQueryMasked(tctx, dbID, query, queryParams, maskPII, verbosity)
				}()
				if err != nil {
					return nil, err
				}
				return createTextResponse(result), nil
			}
			result, err := tq.ExecuteQueryWithTimeout(ctx, dbID, query, queryParams, int(tm))
			if err != nil {
				return nil, err
			}
			return createTextResponse(result), nil
		}
	}

	// Route through the masked path whenever the provider supports it; the
	// use case layer enforces server-level MaskPII config there.
	if m, canMask := useCase.(piIMaskingUseCase); canMask {
		result, err := m.ExecuteQueryMasked(ctx, dbID, query, queryParams, maskPII, verbosity)
		if err != nil {
			return nil, err
		}
		return createTextResponse(result), nil
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
			description: "Execute SQL statements",
		},
	}
}

// dryRunCapable is implemented by use cases offering offline statement-risk
// analysis; detection keeps existing mocks and alternate providers compatible.
type dryRunCapable interface {
	ExecuteStatementDryRun(ctx context.Context, dbID, statement string) (*usecase.RiskReport, error)
}

// snapshotCapable is implemented by use cases exposing pre-mutation
// snapshots for agent-driven undo.
type snapshotCapable interface {
	ListSnapshots(dbID string) []usecase.MutationSnapshot
	RollbackSnapshot(ctx context.Context, dbID, snapshotID string) (string, error)
}

// sensitiveColumnCapable is implemented by use cases offering PII column
// discovery.
type sensitiveColumnCapable interface {
	FindSensitiveColumns(ctx context.Context, dbID string) ([]usecase.SensitiveFinding, error)
}

// contentPICapable is implemented by use cases offering sampled content-based
// PII detection; optional companion to sensitiveColumnCapable.
type contentPICapable interface {
	ScanContentPII(ctx context.Context, dbID string, sampleRows int) ([]usecase.ContentPIIFinding, error)
}

// queryHistoryCapable is implemented by use cases exposing executed-statement
// history for introspection.
type queryHistoryCapable interface {
	GetQueryHistory(dbID string) []usecase.HistoryEntry
}

func pluralY(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

// schemaDriftCapable is implemented by use cases offering schema baselines
// and drift detection.
type schemaDriftCapable interface {
	CaptureSchemaSnapshot(ctx context.Context, dbID string) (*usecase.SchemaSnapshot, error)
	CheckSchemaDrift(ctx context.Context, dbID, baselineID string) (*usecase.SchemaDriftReport, error)
	ListSchemaSnapshots(dbID string) []usecase.SchemaSnapshot
}

// CreateTool creates a per-database execute tool
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
		tools.WithBoolean("dry_run",
			tools.Description("Analyze the statement's risk (destructive ops, missing WHERE, table rewrites) WITHOUT executing it"),
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
		tools.WithBoolean("dry_run",
			tools.Description("Analyze the statement's risk (destructive ops, missing WHERE, table rewrites) WITHOUT executing it"),
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

	// Offline pre-flight: report what the statement WOULD do without running it.
	dryRun, _ := request.Parameters["dry_run"].(bool) //nolint:errcheck // type assertion; absent param means false
	if dryRun {
		dc, capable := useCase.(dryRunCapable)
		if !capable {
			return nil, fmt.Errorf("dry_run is not supported by this provider")
		}
		report, rerr := dc.ExecuteStatementDryRun(ctx, dbID, statement)
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
	fmt.Fprintf(&b, "DRY RUN — nothing was executed.\n")
	fmt.Fprintf(&b, "Kind: %s  Risk: %s  Statements: %d\n", strings.ToUpper(r.Kind[:1])+r.Kind[1:], strings.ToUpper(r.Risk[:1])+r.Risk[1:], r.Statements)
	if len(r.Notes) > 0 {
		b.WriteString("\nAdvisories:\n")
		for _, n := range r.Notes {
			fmt.Fprintf(&b, "- %s\n", n)
		}
	}
	b.WriteString("\nRe-run without dry_run to execute.")
	return b.String()
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
			tools.Description("Transaction action (begin, commit, rollback, execute, list_snapshots, rollback_snapshot, capture_schema_snapshot, check_schema_drift, list_schema_snapshots, list_query_history)"),
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
			tools.Description("Transaction action (begin, commit, rollback, execute, list_snapshots, rollback_snapshot, capture_schema_snapshot, check_schema_drift, list_schema_snapshots, list_query_history)"),
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

	// Snapshot management actions (capability-detected so existing mocks
	// and alternate providers stay compatible).
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
			return createTextResponse(b.String()), nil
		}
		snapID, _ := request.Parameters["snapshot_id"].(string) //nolint:errcheck // absent param handled below
		if snapID == "" {
			return nil, fmt.Errorf("snapshot_id parameter is required for rollback_snapshot")
		}
		msg, err := sc.RollbackSnapshot(ctx, dbID, snapID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(msg), nil
	}

	// Schema drift actions (capability-detected).
	if action == "capture_schema_snapshot" || action == "check_schema_drift" || action == "list_schema_snapshots" {
		sd, capable := useCase.(schemaDriftCapable)
		if !capable {
			return nil, fmt.Errorf("%s is not supported by this provider", action)
		}
		switch action {
		case "capture_schema_snapshot":
			snap, err := sd.CaptureSchemaSnapshot(ctx, dbID)
			if err != nil {
				return nil, err
			}
			var b strings.Builder
			fmt.Fprintf(&b, "Schema baseline %s captured: %d tables.\n", snap.ID, len(snap.Tables))
			for t, cols := range snap.Tables {
				names := make([]string, 0, len(cols))
				for _, c := range cols {
					names = append(names, c.Name+" "+c.Type)
				}
				fmt.Fprintf(&b, "- %s (%s)\n", t, strings.Join(names, ", "))
			}
			return createTextResponse(b.String()), nil
		case "check_schema_drift":
			baselineID, _ := request.Parameters["baseline_id"].(string) //nolint:errcheck // absent handled below
			if baselineID == "" {
				return nil, fmt.Errorf("baseline_id parameter is required for check_schema_drift")
			}
			report, err := sd.CheckSchemaDrift(ctx, dbID, baselineID)
			if err != nil {
				return nil, err
			}
			var b strings.Builder
			if !report.Drifted {
				b.WriteString("No schema drift detected (matches baseline " + baselineID + ").\n")
			} else {
				fmt.Fprintf(&b, "Schema drift detected vs %s:\n", baselineID)
				for _, ch := range report.Changes {
					b.WriteString("- " + ch + "\n")
				}
			}
			return createTextResponse(b.String()), nil
		case "list_schema_snapshots":
			snaps := sd.ListSchemaSnapshots(dbID)
			var b strings.Builder
			fmt.Fprintf(&b, "Schema baselines for %s: %d\n", dbID, len(snaps))
			for _, sn := range snaps {
				fmt.Fprintf(&b, "- %s  %d tables  %s\n", sn.ID, len(sn.Tables), sn.Timestamp.Format(time.RFC3339))
			}
			return createTextResponse(b.String()), nil
		}
	}

	// Query history action (capability-detected).
	if action == "list_query_history" {
		hc, capable := useCase.(queryHistoryCapable)
		if !capable {
			return nil, fmt.Errorf("list_query_history is not supported by this provider")
		}
		entries := hc.GetQueryHistory(dbID)
		var b strings.Builder
		fmt.Fprintf(&b, "Query history for %s: %d entr%s\n", dbID, len(entries), pluralY(len(entries)))
		for _, h := range entries {
			status := "ok"
			if !h.Success {
				status = "failed"
			}
			fmt.Fprintf(&b, "- [%s] %s  %.2fms  %s\n", status, strings.ToUpper(h.Kind[:1])+h.Kind[1:], h.DurationMs, h.Statement)
			if h.Error != "" {
				b.WriteString("    error: " + h.Error + "\n")
			}
		}
		return createTextResponse(b.String()), nil
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
			tools.Description("Action (getSlowQueries, suggest_indexes, analyzeQuery, setThreshold, list_sessions, lock_waits, cancel_query; query required for suggest_indexes, session_id for cancel_query)"),
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
			tools.Description("Action (getSlowQueries, suggest_indexes, analyzeQuery, setThreshold, list_sessions, lock_waits, cancel_query; query required for suggest_indexes, session_id for cancel_query)"),
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

	// Session observability actions bypass the analyzer: they talk to the
	// engine's session catalog directly.
	switch action {
	case "list_sessions":
		if s, can := useCase.(sessionObservabilityUseCase); can {
			out, err := s.ListActiveSessions(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "lock_waits":
		if s, can := useCase.(sessionObservabilityUseCase); can {
			out, err := s.ListBlockingWaits(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "cancel_query":
		if s, can := useCase.(sessionObservabilityUseCase); can {
			sessionID := int64(0)
			if v, ok := request.Parameters["session_id"].(float64); ok {
				sessionID = int64(v)
			} else if vStr, ok := request.Parameters["query_id"].(string); ok && vStr != "" {
				return nil, fmt.Errorf("session_id parameter must be a number")
			}
			if sessionID <= 0 {
				return nil, fmt.Errorf("session_id parameter is required for cancel_query")
			}
			out, err := s.CancelQuery(ctx, dbID, sessionID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
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
		tools.WithString("profile_column",
			tools.Description("Profile one column instead of describing the table: rows, null count, cardinality, min/max, top values"),
		),
		tools.WithString("related_key",
			tools.Description("Primary-key value: follow this row's foreign keys to parents and list referencing children"),
		),
		tools.WithString("duplicates_column",
			tools.Description("Report duplicated values in this column with counts and an example PK per group"),
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
		tools.WithString("profile_column",
			tools.Description("Profile one column instead of describing the table: rows, null count, cardinality, min/max, top values"),
		),
		tools.WithString("related_key",
			tools.Description("Primary-key value: follow this row's foreign keys to parents and list referencing children"),
		),
		tools.WithString("duplicates_column",
			tools.Description("Report duplicated values in this column with counts and an example PK per group"),
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

	// Duplicate detection on one column.
	if colName, ok := request.Parameters["duplicates_column"].(string); ok && strings.TrimSpace(colName) != "" {
		if dd, can := useCase.(duplicateDetectionUseCase); can {
			out, err := dd.FindDuplicates(ctx, dbID, table, colName)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	}

	// FK traversal: resolve one row by PK and render its relations.
	if keyVal, ok := request.Parameters["related_key"].(string); ok && strings.TrimSpace(keyVal) != "" {
		if rr, can := useCase.(relatedRowsUseCase); can {
			out, err := rr.RelatedRows(ctx, dbID, table, keyVal)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	}

	if col, ok := request.Parameters["profile_column"].(string); ok && strings.TrimSpace(col) != "" {
		if pc, can := useCase.(columnProfilingUseCase); can {
			out, err := pc.ProfileColumn(ctx, dbID, table, col)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
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
		tools.WithString("table",
			tools.Description("Table name; required only for format=compare_samples"),
		),
		tools.WithString("format",
			tools.Description(`Output format: "list" (default, table listing), "mermaid" (ER diagram of foreign-key relationships), "sensitive" (PII-suspect column report), "compare" (structural diff vs compare_with database), or "compare_data_counts" (per-table row counts vs compare_with; requires compare_with), or "compare_samples" (row-level diff of one table vs compare_with; requires compare_with + table)`),
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
		tools.WithString("table",
			tools.Description("Table name; required only for format=compare_samples"),
		),
		tools.WithString("format",
			tools.Description(`Output format: "list" (default, table listing), "mermaid" (ER diagram of foreign-key relationships), "sensitive" (PII-suspect column report), "compare" (structural diff vs compare_with database), or "compare_data_counts" (per-table row counts vs compare_with; requires compare_with), or "compare_samples" (row-level diff of one table vs compare_with; requires compare_with + table)`),
		),
	)
}

// HandleRequest handles schema tool requests
func (t *SchemaTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	// If dbID is not provided, extract it from the tool name
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}

	// format selects alternate renderings of the schema (mermaid ERD,
	// sensitive-column report) instead of a plain table listing.
	format, _ := request.Parameters["format"].(string) //nolint:errcheck // absent means default listing
	switch {
	case format == "mermaid":
		graph, err := useCase.RelationshipGraph(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(graph), nil
	case format == "compare_data_counts":
		compareWith, _ := request.Parameters["compare_with"].(string) //nolint:errcheck // absent means error below
		if strings.TrimSpace(compareWith) == "" {
			return nil, fmt.Errorf("format=compare_data_counts requires the compare_with parameter (database id to diff against)")
		}
		dc, can := useCase.(dataCompareUseCase)
		if !can {
			return nil, fmt.Errorf("row-count comparison is not supported by this provider")
		}
		out, err := dc.CompareTableCounts(ctx, dbID, compareWith)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "compare_samples":
		compareWith, _ := request.Parameters["compare_with"].(string) //nolint:errcheck // absent means error below
		if strings.TrimSpace(compareWith) == "" {
			return nil, fmt.Errorf("format=compare_samples requires the compare_with parameter (database id to diff against)")
		}
		table, _ := request.Parameters["table"].(string) //nolint:errcheck // absent means error below
		if strings.TrimSpace(table) == "" {
			return nil, fmt.Errorf("format=compare_samples requires the table parameter")
		}
		limit := 50
		if v, ok := request.Parameters["limit"].(float64); ok && int(v) > 0 {
			limit = int(v)
		}
		dc, can := useCase.(dataCompareUseCase)
		if !can {
			return nil, fmt.Errorf("sample comparison is not supported by this provider")
		}
		out, err := dc.CompareTableSamples(ctx, dbID, compareWith, table, limit)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "compare":
		compareWith, _ := request.Parameters["compare_with"].(string) //nolint:errcheck // absent means error below
		if strings.TrimSpace(compareWith) == "" {
			return nil, fmt.Errorf("format=compare requires the compare_with parameter (database id to diff against)")
		}
		sc, can := useCase.(schemaCompareUseCase)
		if !can {
			return nil, fmt.Errorf("schema comparison is not supported by this provider")
		}
		diff, err := sc.CompareSchemas(ctx, dbID, compareWith)
		if err != nil {
			return nil, err
		}
		return createTextResponse(diff), nil
	case format == "sensitive":
		sc, capable := useCase.(sensitiveColumnCapable)
		if !capable {
			return nil, fmt.Errorf("sensitive column discovery is not supported by this provider")
		}
		findings, err := sc.FindSensitiveColumns(ctx, dbID)
		if err != nil {
			return nil, err
		}
		report := usecase.FormatSensitiveColumnsReport(dbID, findings)
		// Merge content-based findings when the provider supports sampling.
		if cc, contentCapable := useCase.(contentPICapable); contentCapable {
			contentFindings, cerr := cc.ScanContentPII(ctx, dbID, 50)
			if cerr == nil && len(contentFindings) > 0 {
				var b strings.Builder
				b.WriteString(report)
				b.WriteString("\nContent-detected columns (PII patterns found in sampled values):\n")
				for _, f := range contentFindings {
					fmt.Fprintf(&b, "  %s.%s [%s] (%d rows sampled)\n",
						f.Table, f.Column, strings.Join(f.Categories, ", "), f.SamplesScanned)
				}
				report = b.String()
			}
		}
		return createTextResponse(report), nil
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
	factory.Register(NewGenerateSchemaTool())
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
			description: "Find tables by substring match, or find where a value lives",
		},
	}
}

// CreateTool creates a per-database filter_tables tool
func (t *FilterTablesTool) CreateTool(name string, dbID string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetDescription(dbID)),
		tools.WithString("pattern",
			tools.Description("Case-insensitive substring to match table names against (ignored when value is set)"),
			tools.Required(),
		),
		tools.WithString("value",
			tools.Description("Search every textual column of every table for this literal instead of filtering table names"),
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
			tools.Description("Case-insensitive substring to match table names against (ignored when value is set)"),
			tools.Required(),
		),
		tools.WithString("value",
			tools.Description("Search every textual column of every table for this literal instead of filtering table names"),
		),
	)
}

// HandleRequest handles the filter_tables request: it pulls the database
// schema, filters the tables whose names contain the pattern (case
// insensitive), and returns a small text response with the matched names.
func (t *FilterTablesTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}

	// Value search mode: locate a literal across all textual columns.
	if value, _ := request.Parameters["value"].(string); strings.TrimSpace(value) != "" { //nolint:errcheck // absent means table-name mode
		if vs, can := useCase.(valueSearchUseCase); can {
			out, err := vs.SearchValues(ctx, dbID, value)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
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
