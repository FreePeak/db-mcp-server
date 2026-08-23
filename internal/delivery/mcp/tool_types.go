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
		tools.WithString("save_query",
			tools.Description("Save the `query` SQL under this name for this database (bookmark for replay)"),
		),
		tools.WithBoolean("saved_queries",
			tools.Description("List this database's saved query bookmarks with SQL previews"),
		),
		tools.WithString("run_saved_query",
			tools.Description("Execute a saved query bookmark by name"),
		),
		tools.WithNumber("long_queries",
			tools.Description("List queries running longer than this many seconds (activity catalog; Postgres/MySQL)"),
		),
		tools.WithBoolean("unused_indexes",
			tools.Description("List indexes the engine has barely scanned (write-tax candidates; Postgres/MySQL)"),
		),
		tools.WithNumber("min_scans",
			tools.Description("Threshold for unused_indexes: an index needs fewer than this many scans to qualify (default 100)"),
		),
		tools.WithString("databases",
			tools.Description("Comma-separated database ids: run this SELECT on each and render per-database sections (staging vs prod spot-check)"),
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
		tools.WithString("save_query",
			tools.Description("Save the `query` SQL under this name for this database (bookmark for replay)"),
		),
		tools.WithBoolean("saved_queries",
			tools.Description("List this database's saved query bookmarks with SQL previews"),
		),
		tools.WithString("run_saved_query",
			tools.Description("Execute a saved query bookmark by name"),
		),
		tools.WithNumber("long_queries",
			tools.Description("List queries running longer than this many seconds (activity catalog; Postgres/MySQL)"),
		),
		tools.WithBoolean("unused_indexes",
			tools.Description("List indexes the engine has barely scanned (write-tax candidates; Postgres/MySQL)"),
		),
		tools.WithNumber("min_scans",
			tools.Description("Threshold for unused_indexes: an index needs fewer than this many scans to qualify (default 100)"),
		),
		tools.WithString("databases",
			tools.Description("Comma-separated database ids: run this SELECT on each and render per-database sections (staging vs prod spot-check)"),
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

// savedQueryUseCase is implemented by use cases that keep named
// per-database query bookmarks.
type savedQueryUseCase interface {
	SaveQuery(dbID, name, query string) error
	ListSavedQueries(dbID string) (string, error)
	RunSavedQuery(ctx context.Context, dbID, name string) (string, error)
}

// longQueryUseCase is implemented by use cases that list engine
// queries over an age threshold.
type longQueryUseCase interface {
	ListLongQueries(ctx context.Context, dbID string, minSeconds int) (string, error)
}

// indexUsageUseCase is implemented by use cases that report engine
// index-usage statistics.
type indexUsageUseCase interface {
	ListUnusedIndexes(ctx context.Context, dbID string, minScans int) (string, error)
}

// autoIncrementUseCase is implemented by use cases that audit
// auto-increment counters against their type ceilings.
type autoIncrementUseCase interface {
	AuditAutoIncrement(ctx context.Context, dbID string) (string, error)
}

// binaryLogUseCase is implemented by use cases that audit binary-log
// growth against retention.
type binaryLogUseCase interface {
	AuditBinaryLogs(ctx context.Context, dbID string) (string, error)
}

// matviewUseCase is implemented by use cases that audit materialized
// views that error on query (never populated).
type matviewUseCase interface {
	ListUnpopulatedMatviews(ctx context.Context, dbID string) (string, error)
}

// myISAMUseCase is implemented by use cases that audit tables still
// on the MyISAM engine.
type myISAMUseCase interface {
	ListMyISAMTables(ctx context.Context, dbID string) (string, error)
}

// unloggedTableUseCase is implemented by use cases that audit
// WAL-skipping (UNLOGGED) tables.
type unloggedTableUseCase interface {
	ListUnloggedTables(ctx context.Context, dbID string) (string, error)
}

// foreignTableUseCase is implemented by use cases that list FDW
// servers and the local names proxying to them.
type foreignTableUseCase interface {
	ListForeignTables(ctx context.Context, dbID string) (string, error)
}

// roleLimitUseCase is implemented by use cases that audit login roles
// against their connection limits.
type roleLimitUseCase interface {
	ListRoleConnectionLimits(ctx context.Context, dbID string) (string, error)
}

// invalidIndexUseCase is implemented by use cases that audit invalid
// (planner-ignored) indexes.
type invalidIndexUseCase interface {
	ListInvalidIndexes(ctx context.Context, dbID string) (string, error)
}

// checkpointUseCase is implemented by use cases that report checkpoint
// pressure (timed vs requested).
type checkpointUseCase interface {
	CheckCheckpointPressure(ctx context.Context, dbID string) (string, error)
}

// autovacuumOffUseCase is implemented by use cases that list tables
// with autovacuum explicitly disabled.
type autovacuumOffUseCase interface {
	ListAutovacuumDisabled(ctx context.Context, dbID string) (string, error)
}

// walArchiveUseCase is implemented by use cases that report WAL
// archiver health.
type walArchiveUseCase interface {
	CheckWALArchive(ctx context.Context, dbID string) (string, error)
}

// preparedXactUseCase is implemented by use cases that list in-doubt
// two-phase transactions.
type preparedXactUseCase interface {
	ListPreparedTransactions(ctx context.Context, dbID string) (string, error)
}

// extensionUseCase is implemented by use cases that list installed and
// available engine extensions.
type extensionUseCase interface {
	ListExtensions(ctx context.Context, dbID string) (string, error)
}

// charsetUseCase is implemented by use cases that audit deprecated
// column charsets.
type charsetUseCase interface {
	AuditCharsets(ctx context.Context, dbID string) (string, error)
}

// wraparoundUseCase is implemented by use cases that audit
// transaction-ID wraparound risk.
type wraparoundUseCase interface {
	CheckWraparoundRisk(ctx context.Context, dbID string) (string, error)
}

// deadlockUseCase is implemented by use cases that report cumulative
// engine deadlock counters.
type deadlockUseCase interface {
	CheckDeadlocks(ctx context.Context, dbID string) (string, error)
}

// staleSlotUseCase is implemented by use cases that report inactive
// replication slots retaining WAL.
type staleSlotUseCase interface {
	ListStaleSlots(ctx context.Context, dbID string) (string, error)
}

// seqScanUseCase is implemented by use cases that report per-table
// sequential-vs-index scan counters.
type seqScanUseCase interface {
	FindSeqScanHeavy(ctx context.Context, dbID string) (string, error)
}

// tempSpillUseCase is implemented by use cases that report disk-spill
// counters for sorts and temp tables.
type tempSpillUseCase interface {
	CheckTempSpills(ctx context.Context, dbID string) (string, error)
}

// idleSessionsUseCase is implemented by use cases that list idle
// engine connections holding pool slots.
type idleSessionsUseCase interface {
	ListIdleSessions(ctx context.Context, dbID string) (string, error)
}

// guardrailUseCase is implemented by use cases that audit engine-side
// runaway-query timeout settings.
type guardrailUseCase interface {
	CheckTimeoutGuardrails(ctx context.Context, dbID string) (string, error)
}

// saturationUseCase is implemented by use cases that report engine
// connection usage against max_connections.
type saturationUseCase interface {
	CheckConnectionSaturation(ctx context.Context, dbID string) (string, error)
}

// replicationUseCase is implemented by use cases that report replica
// replay status and lag.
type replicationUseCase interface {
	ListReplication(ctx context.Context, dbID string) (string, error)
}

// partitionUseCase is implemented by use cases that list a table's
// child partitions with bounds and sizes.
type partitionUseCase interface {
	ListPartitions(ctx context.Context, dbID, table string) (string, error)
}

// fkRulesUseCase is implemented by use cases that report FK edges with
// their ON DELETE / ON UPDATE referential actions.
type fkRulesUseCase interface {
	ListFKRules(ctx context.Context, dbID string) (string, error)
}

// typeConsistencyUseCase is implemented by use cases that flag shared
// column names with divergent types across tables.
type typeConsistencyUseCase interface {
	FindTypeInconsistencies(ctx context.Context, dbID string) (string, error)
}

// noPKUseCase is implemented by use cases that flag tables without a
// primary key.
type noPKUseCase interface {
	FindTablesWithoutPK(ctx context.Context, dbID string) (string, error)
}

// checkConstraintUseCase is implemented by use cases that list CHECK
// constraints from the engine catalogs.
type checkConstraintUseCase interface {
	ListCheckConstraints(ctx context.Context, dbID string) (string, error)
}

// fkIndexUseCase is implemented by use cases that detect foreign-key
// child columns lacking a leading index.
type fkIndexUseCase interface {
	FindMissingFKIndexes(ctx context.Context, dbID string) (string, error)
}

// redundantIndexUseCase is implemented by use cases that detect
// prefix-covered (redundant) indexes.
type redundantIndexUseCase interface {
	FindRedundantIndexes(ctx context.Context, dbID string) (string, error)
}

// keyDiffUseCase is implemented by use cases that compare primary-key
// sets of one table across two databases.
type keyDiffUseCase interface {
	DiffKeys(ctx context.Context, dbA, dbB, table string) (string, error)
}

// grantsUseCase is implemented by use cases that audit table
// privileges from the engine's grant catalogs.
type grantsUseCase interface {
	ListGrants(ctx context.Context, dbID string) (string, error)
}

// sequenceUseCase is implemented by use cases that audit integer-key
// sequence exhaustion.
type sequenceUseCase interface {
	ListSequences(ctx context.Context, dbID string) (string, error)
}

// dependencyOrderUseCase is implemented by use cases that render the
// FK-safe topological table ordering.
type dependencyOrderUseCase interface {
	DependencyOrder(ctx context.Context, dbID string) (string, error)
}

// maintenanceUseCase is implemented by use cases that surface engine
// statistics-driven upkeep suggestions.
type maintenanceUseCase interface {
	ListMaintenance(ctx context.Context, dbID string) (string, error)
}

// dataDictionaryUseCase is implemented by use cases that render the
// schema as a Markdown data dictionary.
type dataDictionaryUseCase interface {
	DataDictionary(ctx context.Context, dbID string) (string, error)
}

// sizeBaselineUseCase is implemented by use cases that keep a captured
// row-count snapshot per database for growth comparison.
type sizeBaselineUseCase interface {
	CaptureSizeBaseline(ctx context.Context, dbID string) (string, error)
	CompareSizeBaseline(ctx context.Context, dbID string) (string, error)
}

// piiAuditUseCase is implemented by use cases that merge name and
// content PII detectors into one report.
type piiAuditUseCase interface {
	AuditPII(ctx context.Context, dbID string, sampleRows int) (string, error)
}

// overviewUseCase is implemented by use cases that render a one-call
// database shape snapshot.
type overviewUseCase interface {
	DatabaseOverview(ctx context.Context, dbID string) (string, error)
}

// copyVerifyUseCase is implemented by use cases that reconcile row
// counts between databases after a copy.
type copyVerifyUseCase interface {
	VerifyCopy(ctx context.Context, srcDB, dstDB, table string) (string, error)
}

// tableCopyMaskedUseCase is implemented by use cases that copy tables
// while anonymizing PII-bearing values.
type tableCopyMaskedUseCase interface {
	CopyTableMasked(ctx context.Context, srcDB, dstDB, table string) (string, error)
}

// tableCopyUseCase is implemented by use cases that can bulk-copy one
// table between databases.
type tableCopyUseCase interface {
	CopyTable(ctx context.Context, srcDB, dstDB, table string) (string, error)
}

// tableProfileUseCase is implemented by use cases that profile every
// column of one table (nulls, distinct, range).
type tableProfileUseCase interface {
	ProfileTable(ctx context.Context, dbID, table string) (string, error)
}

// tableSizeUseCase is implemented by use cases that report per-table
// row counts and disk sizes.
type tableSizeUseCase interface {
	TableSizes(ctx context.Context, dbID string) (string, error)
}

// orphanAuditUseCase is implemented by use cases that can count child
// rows violating each foreign-key edge.
type orphanAuditUseCase interface {
	AuditOrphans(ctx context.Context, dbID string) (string, error)
}

// ddlDumpUseCase is implemented by use cases that can dump the engine's
// stored CREATE statements.
type ddlDumpUseCase interface {
	DumpDDL(ctx context.Context, dbID string) (string, error)
}

// customTypeListingUseCase is implemented by use cases that can enumerate
// user-defined enum/composite types.
type customTypeListingUseCase interface {
	ListCustomTypes(ctx context.Context, dbID string) (string, error)
}

// routineListingUseCase is implemented by use cases that can enumerate
// stored functions and procedures.
type routineListingUseCase interface {
	ListRoutines(ctx context.Context, dbID string) (string, error)
}

// triggerListingUseCase is implemented by use cases that can enumerate
// triggers with their definitions.
type triggerListingUseCase interface {
	ListTriggers(ctx context.Context, dbID string) (string, error)
}

// viewListingUseCase is implemented by use cases that can enumerate views
// with their definitions.
type viewListingUseCase interface {
	ListViews(ctx context.Context, dbID string) (string, error)
}

// csvImportUseCase is implemented by use cases that can bulk-load CSV
// content atomically.
type csvImportUseCase interface {
	ImportCSV(ctx context.Context, dbID, table, csvContent string) (string, error)
}

// migrationRunnerUseCase is implemented by use cases that can apply
// versioned .sql migrations from a directory.
type migrationRunnerUseCase interface {
	RunMigrations(ctx context.Context, dbID, dir string) (string, error)
}

// scriptExecutionUseCase is implemented by use cases that can run a
// multi-statement script atomically.
type scriptExecutionUseCase interface {
	ExecuteScript(ctx context.Context, dbID, script string) (string, error)
}

// duplicateDetectionUseCase is implemented by use cases that can report
// duplicated values in one column.
type duplicateDetectionUseCase interface {
	FindDuplicates(ctx context.Context, dbID, table, column string) (string, error)
}

// acrossQueryUseCase is implemented by use cases that can fan one SELECT
// out over several databases.
type acrossQueryUseCase interface {
	ExecuteQueryAcross(ctx context.Context, query string, dbIDs []string) (string, error)
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
	ListLongTransactions(ctx context.Context, dbID string, minAgeSecs int) (string, error)
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
	// Fan-out runs the SELECT on every listed database.
	if dbsRaw, _ := request.Parameters["databases"].(string); strings.TrimSpace(dbsRaw) != "" { //nolint:errcheck // absent means single-db mode
		var dbIDs []string
		for _, d := range strings.Split(dbsRaw, ",") {
			if t := strings.TrimSpace(d); t != "" {
				dbIDs = append(dbIDs, t)
			}
		}
		if len(dbIDs) > 1 {
			if aq, can := useCase.(acrossQueryUseCase); can {
				result, err := aq.ExecuteQueryAcross(ctx, query, dbIDs)
				if err != nil {
					return nil, err
				}
				return createTextResponse(result), nil
			}
		}
	}

	// Saved queries: bookmark and replay named SELECTs per database.
	if sq, ok := request.Parameters["save_query"].(string); ok && strings.TrimSpace(sq) != "" {
		sqlText, _ := request.Parameters["query"].(string) //nolint:errcheck // validated below
		suc, can := useCase.(savedQueryUseCase)
		if !can {
			return nil, fmt.Errorf("saved queries are not supported by this provider")
		}
		if err := suc.SaveQuery(dbID, sq, sqlText); err != nil {
			return nil, err
		}
		return createTextResponse(fmt.Sprintf("Saved %q on %s. Run with run_saved_query.", sq, dbID)), nil
	}
	if list, ok := request.Parameters["saved_queries"].(bool); ok && list {
		suc, can := useCase.(savedQueryUseCase)
		if !can {
			return nil, fmt.Errorf("saved queries are not supported by this provider")
		}
		out, err := suc.ListSavedQueries(dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	}
	if rn, ok := request.Parameters["run_saved_query"].(string); ok && strings.TrimSpace(rn) != "" {
		ruc, can := useCase.(savedQueryUseCase)
		if !can {
			return nil, fmt.Errorf("saved queries are not supported by this provider")
		}
		out, err := ruc.RunSavedQuery(ctx, dbID, rn)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	}

	// Long-query triage: active queries over an age threshold.
	if secs, ok := request.Parameters["long_queries"].(float64); ok && secs > 0 {
		luc, can := useCase.(longQueryUseCase)
		if !can {
			return nil, fmt.Errorf("long-query reporting is not supported by this provider")
		}
		out, err := luc.ListLongQueries(ctx, dbID, int(secs))
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	}

	// Unused index detection: write-tax candidates from usage stats.
	if ui, ok := request.Parameters["unused_indexes"].(bool); ok && ui {
		uuc, can := useCase.(indexUsageUseCase)
		if !can {
			return nil, fmt.Errorf("index usage reporting is not supported by this provider")
		}
		thr := 100.0
		if v, ok2 := request.Parameters["min_scans"].(float64); ok2 && v > 0 {
			thr = v
		}
		out, err := uuc.ListUnusedIndexes(ctx, dbID, int(thr))
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
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
		tools.WithString("script",
			tools.Description("Multi-statement script (semicolon-separated) executed atomically: all commit or all roll back with the failing statement named"),
		),
		tools.WithString("copy_table",
			tools.Description("Copy every row of this table from another database into this one inside one transaction; requires from_db"),
		),
		tools.WithString("from_db",
			tools.Description("Source database id for copy_table or verify_copy"),
		),
		tools.WithBoolean("mask_pii",
			tools.Description("With copy_table: anonymize PII-bearing text (emails, phones, cards, SSNs, IPs) during the copy so prod data can seed staging safely"),
		),
		tools.WithString("verify_copy",
			tools.Description("Verify a previous copy: compare row counts of this table between from_db and here; requires from_db"),
		),
		tools.WithString("migrate_dir",
			tools.Description("Directory of versioned .sql migration files (001_, 002_, …); applies pending ones in name order, each atomically, tracked in _mcp_migrations"),
		),
		tools.WithString("csv_data",
			tools.Description("CSV content (header + rows) to bulk-insert atomically; requires csv_table; capped at 10k rows"),
		),
		tools.WithString("csv_table",
			tools.Description("Target table for csv_data imports"),
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
		tools.WithString("script",
			tools.Description("Multi-statement script (semicolon-separated) executed atomically: all commit or all roll back with the failing statement named"),
		),
		tools.WithString("copy_table",
			tools.Description("Copy every row of this table from another database into this one inside one transaction; requires from_db"),
		),
		tools.WithString("from_db",
			tools.Description("Source database id for copy_table or verify_copy"),
		),
		tools.WithBoolean("mask_pii",
			tools.Description("With copy_table: anonymize PII-bearing text (emails, phones, cards, SSNs, IPs) during the copy so prod data can seed staging safely"),
		),
		tools.WithString("verify_copy",
			tools.Description("Verify a previous copy: compare row counts of this table between from_db and here; requires from_db"),
		),
		tools.WithString("migrate_dir",
			tools.Description("Directory of versioned .sql migration files (001_, 002_, …); applies pending ones in name order, each atomically, tracked in _mcp_migrations"),
		),
		tools.WithString("csv_data",
			tools.Description("CSV content (header + rows) to bulk-insert atomically; requires csv_table; capped at 10k rows"),
		),
		tools.WithString("csv_table",
			tools.Description("Target table for csv_data imports"),
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

	// Column profiling: nulls/distinct/range per column in one call.
	if prof, ok := request.Parameters["profile"].(bool); ok && prof {
		puc, can := useCase.(tableProfileUseCase)
		if !can {
			return nil, fmt.Errorf("profiling is not supported by this provider")
		}
		table, _ := request.Parameters["table"].(string) //nolint:errcheck // validated by usecase
		out, err := puc.ProfileTable(ctx, dbID, table)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	}

	// Post-copy verification: row-count reconciliation between databases.
	if vc, ok := request.Parameters["verify_copy"].(string); ok && strings.TrimSpace(vc) != "" {
		fromDB, _ := request.Parameters["from_db"].(string) //nolint:errcheck // absent means error below
		if strings.TrimSpace(fromDB) == "" {
			return nil, fmt.Errorf("from_db is required when verify_copy is provided")
		}
		vuc, can := useCase.(copyVerifyUseCase)
		if !can {
			return nil, fmt.Errorf("copy verification is not supported by this provider")
		}
		out, err := vuc.VerifyCopy(ctx, fromDB, dbID, vc)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	}

	// Cross-database table copy: destination is this tool's database.
	if ct, ok := request.Parameters["copy_table"].(string); ok && strings.TrimSpace(ct) != "" {
		fromDB, _ := request.Parameters["from_db"].(string) //nolint:errcheck // absent means error below
		if strings.TrimSpace(fromDB) == "" {
			return nil, fmt.Errorf("from_db is required when copy_table is provided")
		}
		if maskPII, _ := request.Parameters["mask_pii"].(bool); maskPII { //nolint:errcheck // absent means false
			mc, mcan := useCase.(tableCopyMaskedUseCase)
			if !mcan {
				return nil, fmt.Errorf("anonymized copy is not supported by this provider")
			}
			out, err := mc.CopyTableMasked(ctx, fromDB, dbID, ct)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
		cc, can := useCase.(tableCopyUseCase)
		if !can {
			return nil, fmt.Errorf("table copy is not supported by this provider")
		}
		out, err := cc.CopyTable(ctx, fromDB, dbID, ct)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	}

	// Migration runner: apply pending .sql files from a directory.
	if migDir, ok := request.Parameters["migrate_dir"].(string); ok && strings.TrimSpace(migDir) != "" {
		mr, can := useCase.(migrationRunnerUseCase)
		if !can {
			return nil, fmt.Errorf("migrations are not supported by this provider")
		}
		out, err := mr.RunMigrations(ctx, dbID, migDir)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	}

	// CSV import: atomic bulk insert.
	if csvData, ok := request.Parameters["csv_data"].(string); ok && strings.TrimSpace(csvData) != "" {
		csvTable, _ := request.Parameters["csv_table"].(string) //nolint:errcheck // absent means error below
		if strings.TrimSpace(csvTable) == "" {
			return nil, fmt.Errorf("csv_table is required when csv_data is provided")
		}
		im, can := useCase.(csvImportUseCase)
		if !can {
			return nil, fmt.Errorf("CSV import is not supported by this provider")
		}
		out, err := im.ImportCSV(ctx, dbID, csvTable, csvData)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	}

	// Atomic multi-statement scripts.
	if script, ok := request.Parameters["script"].(string); ok && strings.TrimSpace(script) != "" {
		sc, can := useCase.(scriptExecutionUseCase)
		if !can {
			return nil, fmt.Errorf("script execution is not supported by this provider")
		}
		out, err := sc.ExecuteScript(ctx, dbID, script)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	}

	statement, ok := request.Parameters["statement"].(string)
	if !ok {
		return nil, fmt.Errorf("statement parameter must be a string (or use the script parameter for a multi-statement batch)")
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
			tools.Description("Action (getSlowQueries, suggest_indexes, analyzeQuery, setThreshold, list_sessions, lock_waits, long_transactions, replication_status, connection_saturation, timeout_guardrails, idle_sessions, temp_spills, seq_scan_heavy, stale_slots, deadlock_counts, wraparound_risk, charset_audit, list_extensions, prepared_xacts, wal_archive, autovacuum_disabled, checkpoint_pressure, invalid_indexes, role_connection_limits, foreign_tables, unlogged_tables, myisam_tables, unpopulated_matviews, binlog_growth, auto_increment_headroom, cancel_query; query required for suggest_indexes, session_id for cancel_query)"),
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
		tools.WithNumber("min_age_secs",
			tools.Description("Minimum transaction age in seconds for long_transactions (default 60)"),
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
			tools.Description("Action (getSlowQueries, suggest_indexes, analyzeQuery, setThreshold, list_sessions, lock_waits, long_transactions, replication_status, connection_saturation, timeout_guardrails, idle_sessions, temp_spills, seq_scan_heavy, stale_slots, deadlock_counts, wraparound_risk, charset_audit, list_extensions, prepared_xacts, wal_archive, autovacuum_disabled, checkpoint_pressure, invalid_indexes, role_connection_limits, foreign_tables, unlogged_tables, myisam_tables, unpopulated_matviews, binlog_growth, auto_increment_headroom, cancel_query; query required for suggest_indexes, session_id for cancel_query)"),
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
		tools.WithNumber("min_age_secs",
			tools.Description("Minimum transaction age in seconds for long_transactions (default 60)"),
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
	case "auto_increment_headroom":
		if aic, can := useCase.(autoIncrementUseCase); can {
			out, err := aic.AuditAutoIncrement(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "binlog_growth":
		if blc, can := useCase.(binaryLogUseCase); can {
			out, err := blc.AuditBinaryLogs(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "unpopulated_matviews":
		if mvc, can := useCase.(matviewUseCase); can {
			out, err := mvc.ListUnpopulatedMatviews(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "myisam_tables":
		if mtc, can := useCase.(myISAMUseCase); can {
			out, err := mtc.ListMyISAMTables(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "unlogged_tables":
		if utc, can := useCase.(unloggedTableUseCase); can {
			out, err := utc.ListUnloggedTables(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "foreign_tables":
		if ftc, can := useCase.(foreignTableUseCase); can {
			out, err := ftc.ListForeignTables(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "role_connection_limits":
		if rlc, can := useCase.(roleLimitUseCase); can {
			out, err := rlc.ListRoleConnectionLimits(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "invalid_indexes":
		if iic, can := useCase.(invalidIndexUseCase); can {
			out, err := iic.ListInvalidIndexes(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "checkpoint_pressure":
		if cpc, can := useCase.(checkpointUseCase); can {
			out, err := cpc.CheckCheckpointPressure(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "autovacuum_disabled":
		if adc, can := useCase.(autovacuumOffUseCase); can {
			out, err := adc.ListAutovacuumDisabled(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "wal_archive":
		if wac, can := useCase.(walArchiveUseCase); can {
			out, err := wac.CheckWALArchive(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "prepared_xacts":
		if pxc, can := useCase.(preparedXactUseCase); can {
			out, err := pxc.ListPreparedTransactions(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "list_extensions":
		if lec, can := useCase.(extensionUseCase); can {
			out, err := lec.ListExtensions(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "charset_audit":
		if csc, can := useCase.(charsetUseCase); can {
			out, err := csc.AuditCharsets(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "wraparound_risk":
		if wrc, can := useCase.(wraparoundUseCase); can {
			out, err := wrc.CheckWraparoundRisk(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "deadlock_counts":
		if duc, can := useCase.(deadlockUseCase); can {
			out, err := duc.CheckDeadlocks(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "stale_slots":
		if wsc, can := useCase.(staleSlotUseCase); can {
			out, err := wsc.ListStaleSlots(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "seq_scan_heavy":
		if ssc, can := useCase.(seqScanUseCase); can {
			out, err := ssc.FindSeqScanHeavy(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "temp_spills":
		if tsc, can := useCase.(tempSpillUseCase); can {
			out, err := tsc.CheckTempSpills(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "idle_sessions":
		if isc, can := useCase.(idleSessionsUseCase); can {
			out, err := isc.ListIdleSessions(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "timeout_guardrails":
		if g, can := useCase.(guardrailUseCase); can {
			out, err := g.CheckTimeoutGuardrails(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "connection_saturation":
		if s, can := useCase.(saturationUseCase); can {
			out, err := s.CheckConnectionSaturation(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "replication_status":
		if s, can := useCase.(replicationUseCase); can {
			out, err := s.ListReplication(ctx, dbID)
			if err != nil {
				return nil, err
			}
			return createTextResponse(out), nil
		}
	case "long_transactions":
		if s, can := useCase.(sessionObservabilityUseCase); can {
			minAge := 60
			if v, ok := request.Parameters["min_age_secs"].(float64); ok && v > 0 {
				minAge = int(v)
			}
			out, err := s.ListLongTransactions(ctx, dbID, minAge)
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
		tools.WithBoolean("profile",
			tools.Description("Profile the table: per-column rows, NULL count, distinct count, and min/max"),
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
		tools.WithBoolean("profile",
			tools.Description("Profile the table: per-column rows, NULL count, distinct count, and min/max"),
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
		tools.WithString("action",
			tools.Description(`"trend" renders the rolling pool-pressure history with deltas instead of a fresh check`),
		),
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
		tools.WithString("action",
			tools.Description(`"trend" renders the rolling pool-pressure history with deltas instead of a fresh check`),
		),
	)
}

// healthTrendUseCase is implemented by use cases that keep a rolling
// per-database health sample history.
type healthTrendUseCase interface {
	HealthTrend(dbID string) (string, error)
}

// HandleRequest handles health tool requests; action=trend renders the
// rolling sample history instead of a fresh check.
func (t *HealthTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}

	if action, _ := request.Parameters["action"].(string); action == "trend" { //nolint:errcheck // non-string means no trend
		tuc, can := useCase.(healthTrendUseCase)
		if !can {
			return nil, fmt.Errorf("health trend is not supported by this provider")
		}
		out, err := tuc.HealthTrend(dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
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
			tools.Description(`Output format: "list" (default, table listing), "mermaid" (ER diagram of foreign-key relationships), "sensitive" (PII-suspect column report), "compare" (structural diff vs compare_with database), or "compare_data_counts" (per-table row counts vs compare_with; requires compare_with), "compare_samples" (row-level diff of one table vs compare_with; requires compare_with + table), "views" (views with their SQL definitions), "triggers" (triggers with target tables and bodies), "routines" (stored functions/procedures), "types" (user-defined enum/composite types), "ddl" (verbatim CREATE statements; sqlite only), "orphans" (count child rows violating each foreign key), "type_consistency" (shared column names with divergent types across tables — joins will coerce or fail), "no_pk" (user tables lacking a PRIMARY KEY — replication breaks, rows unaddressable), "fk_rules" (every FK edge with its ON DELETE / ON UPDATE behavior — CASCADE silently destroys children, NO ACTION blocks the delete; Postgres/MySQL), "partitions" (child partitions of one table with bounds, row estimates, and sizes — requires table; Postgres/MySQL), "checks" (CHECK-constraint clauses grouped by table — the business rules valid data must satisfy; Postgres/MySQL 8+), "fk_indexes" (foreign-key child columns with no leading index — parent deletes scan the child table; candidate DDL included), "redundant_indexes" (non-unique indexes whose column list is a prefix of a wider sibling — write amplification with no read benefit), "key_diff" (primary-key set difference for one table vs compare_with; requires compare_with + table), "grants" (table privileges grouped by grantee from the engine catalogs; Postgres/MySQL), "sequences" (integer-key sequences at >=80% of their ceiling — exhaustion is a silent insert-failure incident; Postgres), "dependency_order" (FK-safe topological table order for seeding/truncating, cycles flagged), "maintenance" (bloat/fragmentation/stale-statistics suggestions from engine catalogs; Postgres/MySQL), "dictionary" (whole schema as a Markdown data dictionary), "sizes" (row counts and disk size per table), "baseline_capture" / "baseline_compare" (record row counts now, diff later for growth), "overview" (one-call shape snapshot: tables/columns/indexes/FK edges/rows plus PII-name suspects), or "pii_audit" (merged name+content PII report; optional sample_rows, default 50)`),
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
			tools.Description(`Output format: "list" (default, table listing), "mermaid" (ER diagram of foreign-key relationships), "sensitive" (PII-suspect column report), "compare" (structural diff vs compare_with database), or "compare_data_counts" (per-table row counts vs compare_with; requires compare_with), "compare_samples" (row-level diff of one table vs compare_with; requires compare_with + table), "views" (views with their SQL definitions), "triggers" (triggers with target tables and bodies), "routines" (stored functions/procedures), "types" (user-defined enum/composite types), "ddl" (verbatim CREATE statements; sqlite only), "orphans" (count child rows violating each foreign key), "type_consistency" (shared column names with divergent types across tables — joins will coerce or fail), "no_pk" (user tables lacking a PRIMARY KEY — replication breaks, rows unaddressable), "fk_rules" (every FK edge with its ON DELETE / ON UPDATE behavior — CASCADE silently destroys children, NO ACTION blocks the delete; Postgres/MySQL), "partitions" (child partitions of one table with bounds, row estimates, and sizes — requires table; Postgres/MySQL), "checks" (CHECK-constraint clauses grouped by table — the business rules valid data must satisfy; Postgres/MySQL 8+), "fk_indexes" (foreign-key child columns with no leading index — parent deletes scan the child table; candidate DDL included), "redundant_indexes" (non-unique indexes whose column list is a prefix of a wider sibling — write amplification with no read benefit), "key_diff" (primary-key set difference for one table vs compare_with; requires compare_with + table), "grants" (table privileges grouped by grantee from the engine catalogs; Postgres/MySQL), "sequences" (integer-key sequences at >=80% of their ceiling — exhaustion is a silent insert-failure incident; Postgres), "dependency_order" (FK-safe topological table order for seeding/truncating, cycles flagged), "maintenance" (bloat/fragmentation/stale-statistics suggestions from engine catalogs; Postgres/MySQL), "dictionary" (whole schema as a Markdown data dictionary), "sizes" (row counts and disk size per table), "baseline_capture" / "baseline_compare" (record row counts now, diff later for growth), "overview" (one-call shape snapshot: tables/columns/indexes/FK edges/rows plus PII-name suspects), or "pii_audit" (merged name+content PII report; optional sample_rows, default 50)`),
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
	case format == "partitions":
		ptc, can := useCase.(partitionUseCase)
		if !can {
			return nil, fmt.Errorf("partition introspection is not supported by this provider")
		}
		table, _ := request.Parameters["table"].(string) //nolint:errcheck // empty means missing
		if table == "" {
			return nil, fmt.Errorf(`format "partitions" requires a "table" parameter`)
		}
		out, err := ptc.ListPartitions(ctx, dbID, table)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "fk_rules":
		fuc, can := useCase.(fkRulesUseCase)
		if !can {
			return nil, fmt.Errorf("foreign-key rule auditing is not supported by this provider")
		}
		out, err := fuc.ListFKRules(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "type_consistency":
		tuc, can := useCase.(typeConsistencyUseCase)
		if !can {
			return nil, fmt.Errorf("type-consistency auditing is not supported by this provider")
		}
		out, err := tuc.FindTypeInconsistencies(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "no_pk":
		nuc, can := useCase.(noPKUseCase)
		if !can {
			return nil, fmt.Errorf("primary-key auditing is not supported by this provider")
		}
		out, err := nuc.FindTablesWithoutPK(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "checks":
		cc, can := useCase.(checkConstraintUseCase)
		if !can {
			return nil, fmt.Errorf("CHECK-constraint reporting is not supported by this provider")
		}
		out, err := cc.ListCheckConstraints(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "fk_indexes":
		fuc, can := useCase.(fkIndexUseCase)
		if !can {
			return nil, fmt.Errorf("missing-FK-index detection is not supported by this provider")
		}
		out, err := fuc.FindMissingFKIndexes(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "redundant_indexes":
		ruc, can := useCase.(redundantIndexUseCase)
		if !can {
			return nil, fmt.Errorf("redundant-index detection is not supported by this provider")
		}
		out, err := ruc.FindRedundantIndexes(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "key_diff":
		kuc, can := useCase.(keyDiffUseCase)
		if !can {
			return nil, fmt.Errorf("key diff is not supported by this provider")
		}
		compareWith2, _ := request.Parameters["compare_with"].(string) //nolint:errcheck // absent means error below
		if strings.TrimSpace(compareWith2) == "" {
			return nil, fmt.Errorf("format=key_diff requires the compare_with parameter (database id to diff against)")
		}
		table2, _ := request.Parameters["table"].(string) //nolint:errcheck // absent means error below
		if strings.TrimSpace(table2) == "" {
			return nil, fmt.Errorf("format=key_diff requires the table parameter")
		}
		out, err := kuc.DiffKeys(ctx, dbID, compareWith2, table2)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "grants":
		guc, can := useCase.(grantsUseCase)
		if !can {
			return nil, fmt.Errorf("grants reporting is not supported by this provider")
		}
		out, err := guc.ListGrants(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "sequences":
		suc2, can := useCase.(sequenceUseCase)
		if !can {
			return nil, fmt.Errorf("sequence reporting is not supported by this provider")
		}
		out, err := suc2.ListSequences(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "dependency_order":
		doc, can := useCase.(dependencyOrderUseCase)
		if !can {
			return nil, fmt.Errorf("dependency ordering is not supported by this provider")
		}
		out, err := doc.DependencyOrder(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "maintenance":
		muc, can := useCase.(maintenanceUseCase)
		if !can {
			return nil, fmt.Errorf("maintenance reporting is not supported by this provider")
		}
		out, err := muc.ListMaintenance(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "dictionary":
		dic, can := useCase.(dataDictionaryUseCase)
		if !can {
			return nil, fmt.Errorf("data dictionary is not supported by this provider")
		}
		out, err := dic.DataDictionary(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "baseline_capture":
		buc, can := useCase.(sizeBaselineUseCase)
		if !can {
			return nil, fmt.Errorf("size baselines are not supported by this provider")
		}
		out, err := buc.CaptureSizeBaseline(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "baseline_compare":
		bcuc, can2 := useCase.(sizeBaselineUseCase)
		if !can2 {
			return nil, fmt.Errorf("size baselines are not supported by this provider")
		}
		out, err := bcuc.CompareSizeBaseline(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "pii_audit":
		pauc, can := useCase.(piiAuditUseCase)
		if !can {
			return nil, fmt.Errorf("combined PII audit is not supported by this provider")
		}
		sr := 50.0
		if v, ok2 := request.Parameters["sample_rows"].(float64); ok2 && v > 0 {
			sr = v
		}
		out, err := pauc.AuditPII(ctx, dbID, int(sr))
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "overview":
		ouc, can := useCase.(overviewUseCase)
		if !can {
			return nil, fmt.Errorf("database overview is not supported by this provider")
		}
		out, err := ouc.DatabaseOverview(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "sizes":
		ts, can := useCase.(tableSizeUseCase)
		if !can {
			return nil, fmt.Errorf("table size reporting is not supported by this provider")
		}
		out, err := ts.TableSizes(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "orphans":
		ao, can := useCase.(orphanAuditUseCase)
		if !can {
			return nil, fmt.Errorf("orphan audit is not supported by this provider")
		}
		out, err := ao.AuditOrphans(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "ddl":
		dd, can := useCase.(ddlDumpUseCase)
		if !can {
			return nil, fmt.Errorf("DDL dump is not supported by this provider")
		}
		out, err := dd.DumpDDL(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "types":
		ct, can := useCase.(customTypeListingUseCase)
		if !can {
			return nil, fmt.Errorf("custom type listing is not supported by this provider")
		}
		out, err := ct.ListCustomTypes(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "routines":
		lr, can := useCase.(routineListingUseCase)
		if !can {
			return nil, fmt.Errorf("routine listing is not supported by this provider")
		}
		out, err := lr.ListRoutines(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "triggers":
		lt, can := useCase.(triggerListingUseCase)
		if !can {
			return nil, fmt.Errorf("trigger listing is not supported by this provider")
		}
		out, err := lt.ListTriggers(ctx, dbID)
		if err != nil {
			return nil, err
		}
		return createTextResponse(out), nil
	case format == "views":
		lv, can := useCase.(viewListingUseCase)
		if !can {
			return nil, fmt.Errorf("view listing is not supported by this provider")
		}
		out, err := lv.ListViews(ctx, dbID)
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
