package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	cortexServer "github.com/FreePeak/cortex/pkg/server"
	cortexTools "github.com/FreePeak/cortex/pkg/tools"
	sdkAuth "github.com/modelcontextprotocol/go-sdk/auth"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/FreePeak/db-mcp-server/internal/domain"
	"github.com/FreePeak/db-mcp-server/internal/usecase"
)

func TestServerWrapperSDKStreamableRoundTrip(t *testing.T) {
	wrapper := NewServerWrapper(cortexServer.NewMCPServer("test", "1.0.0", nil))
	tool := cortexTools.NewTool(
		"echo",
		cortexTools.WithDescription("echo text"),
		cortexTools.WithString("text", cortexTools.Description("text to echo"), cortexTools.Required()),
	)
	var seen usecase.Caller
	err := wrapper.AddTool(context.Background(), tool, func(ctx context.Context, request cortexServer.ToolCallRequest) (interface{}, error) {
		seen = usecase.CallerFromContext(ctx)
		text := request.Parameters["text"].(string)
		if text == "error" {
			return nil, errors.New("tool failed")
		}
		return &Response{
			Content:  []TextContent{{Type: "text", Text: "echo:" + text}},
			Metadata: map[string]interface{}{"transactionId": "tx-test"},
		}, nil
	})
	require.NoError(t, err)

	httpServer := httptest.NewServer(NewStreamableHTTPHandler(wrapper.SDKServer()))
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.StreamableClientTransport{
		Endpoint:             httpServer.URL,
		HTTPClient:           httpServer.Client(),
		DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tools.Tools, 1)
	require.Equal(t, "echo", tools.Tools[0].Name)
	schema, ok := tools.Tools[0].InputSchema.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "object", schema["type"])
	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, properties, "text")

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"text": "hello"},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)
	textContent, ok := result.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	require.Equal(t, "echo:hello", textContent.Text)
	require.Equal(t, "tx-test", result.Meta["transactionId"])
	require.NotEmpty(t, seen.Session, "streamable tool call must carry the MCP session ID to the use case")

	errorResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"text": "error"},
	})
	require.NoError(t, err)
	require.True(t, errorResult.IsError)
	require.Len(t, errorResult.Content, 1)
	errorText, ok := errorResult.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	require.Equal(t, "tool failed", errorText.Text)

	_, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "echo",
		Arguments: []string{"not-an-object"},
	})
	require.Error(t, err)
}

func TestAdaptCortexHandlerEnforcesStatementScope(t *testing.T) {
	calls := 0
	handler := adaptCortexHandler(func(context.Context, cortexServer.ToolCallRequest) (interface{}, error) {
		calls++
		return &Response{Content: []TextContent{{Type: "text", Text: "ok"}}}, nil
	})
	readOnly := &sdkAuth.TokenInfo{Scopes: []string{"db:read"}}
	call := func(name, arguments string) *sdkmcp.CallToolRequest {
		return &sdkmcp.CallToolRequest{
			Params: &sdkmcp.CallToolParamsRaw{Name: name, Arguments: json.RawMessage(arguments)},
			Extra:  &sdkmcp.RequestExtra{TokenInfo: readOnly},
		}
	}
	ctx := context.Background()

	refused, err := handler(ctx, call("query_orders", `{"query":"DELETE FROM orders"}`))
	require.NoError(t, err)
	require.True(t, refused.IsError)
	require.Equal(t, 0, calls, "a db:read token must not reach the handler for a DELETE")

	allowed, err := handler(ctx, call("query_orders", `{"query":"SELECT 1"}`))
	require.NoError(t, err)
	require.False(t, allowed.IsError)
	require.Equal(t, 1, calls)

	_, err = handler(ctx, call("query_orders", `[1]`))
	require.Error(t, err, "arguments that are not a JSON object are rejected before the handler")
	require.Equal(t, 1, calls)
}

func TestToolScopeAllowed(t *testing.T) {
	select1 := map[string]interface{}{"query": "SELECT 1"}
	request := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{Name: "query"},
		Extra:  &sdkmcp.RequestExtra{TokenInfo: &sdkAuth.TokenInfo{Scopes: []string{"db:read"}}},
	}
	require.True(t, toolScopeAllowed(request, select1))
	request.Params.Name = "execute"
	require.False(t, toolScopeAllowed(request, select1))
	request.Params.Name = "query"
	request.Extra.TokenInfo.Scopes = []string{"db:admin"}
	request.Params.Name = "execute"
	require.True(t, toolScopeAllowed(request, select1))
}

func TestRequiredToolScope(t *testing.T) {
	const read, write = "db:read", "db:write"
	cases := []struct {
		name      string
		tool      string
		arguments map[string]interface{}
		want      string
	}{
		// Per-database and unified query tools are classified by statement.
		{"query select is read", "query_orders", map[string]interface{}{"query": "SELECT * FROM orders"}, read},
		{"query delete is write", "query_orders", map[string]interface{}{"query": "DELETE FROM orders"}, write},
		{"query drop is write", "query_orders", map[string]interface{}{"query": "DROP TABLE orders"}, write},
		{"query stacked delete is write", "query_orders", map[string]interface{}{"query": "SELECT 1; DELETE FROM orders"}, write},
		{"query data-modifying cte is write", "query_orders", map[string]interface{}{"query": "WITH d AS (DELETE FROM orders RETURNING 1) SELECT * FROM d"}, write},
		{"query select into is write", "query_x", map[string]interface{}{"query": "SELECT * INTO newt FROM orders"}, write},
		{"query unknown verb is write", "query_orders", map[string]interface{}{"query": "FLUSH TABLES"}, write},
		{"query missing sql is write", "query_orders", map[string]interface{}{}, write},
		{"query blank sql is write", "query_orders", map[string]interface{}{"query": "  "}, write},
		{"query non-string sql is write", "query_orders", map[string]interface{}{"query": 42}, write},
		{"query without arguments is write", "query_orders", nil, write},
		{"unified query select is read", "query", map[string]interface{}{"database": "orders", "query": "SELECT 1"}, read},
		{"unified query delete is write", "query", map[string]interface{}{"database": "orders", "query": "DELETE FROM t"}, write},

		// explain classifies its statement whether or not ANALYZE executes it.
		{"explain select is read", "explain_orders", map[string]interface{}{"statement": "SELECT 1"}, read},
		{"explain analyze delete is write", "explain_orders", map[string]interface{}{"statement": "DELETE FROM t", "analyze": true}, write},
		{"explain delete without analyze is write", "explain_orders", map[string]interface{}{"statement": "DELETE FROM t"}, write},
		{"explain missing statement is write", "explain_orders", map[string]interface{}{}, write},

		// performance: reset clears history and validate_suggestions runs CREATE EXTENSION.
		{"performance stats is read", "performance_orders", map[string]interface{}{"action": "stats"}, read},
		{"performance suggest is read", "performance_orders", map[string]interface{}{"action": "suggest", "query": "SELECT 1"}, read},
		{"performance engine_slow_queries is read", "performance_orders", map[string]interface{}{"action": "engine_slow_queries"}, read},
		{"performance reset is write", "performance_orders", map[string]interface{}{"action": "reset"}, write},
		{"performance validate_suggestions is write", "performance_orders", map[string]interface{}{"action": "validate_suggestions"}, write},
		{"performance unknown action is write", "performance_orders", map[string]interface{}{"action": "bogus"}, write},
		{"performance missing action is write", "performance_orders", map[string]interface{}{}, write},

		// Catalog and metadata tools never write.
		{"schema is read", "schema_orders", map[string]interface{}{}, read},
		{"describe is read", "describe_orders", map[string]interface{}{}, read},
		{"health is read", "health_orders", map[string]interface{}{}, read},
		{"filter_tables is read", "filter_tables", map[string]interface{}{"database": "orders", "pattern": "x"}, read},
		{"list_databases is read", "list_databases", map[string]interface{}{}, read},

		// execute and transaction tools stay write regardless of arguments.
		{"execute is write", "execute_orders", map[string]interface{}{"statement": "SELECT 1"}, write},
		{"unified execute is write", "execute", map[string]interface{}{"database": "orders", "statement": "SELECT 1"}, write},
		{"transaction is write", "transaction_orders", map[string]interface{}{"action": "list_snapshots"}, write},

		// TimescaleDB tools are classified by the operation argument.
		{"timescale list_hypertables is read", "timescaledb_list_hypertables_orders", map[string]interface{}{"operation": "list_hypertables"}, read},
		{"timescale get_retention_policy is read", "timescaledb_retention_policy_orders", map[string]interface{}{"operation": "get_retention_policy"}, read},
		{"timescale drop_continuous_aggregate is write", "timescaledb_list_hypertables_orders", map[string]interface{}{"operation": "drop_continuous_aggregate"}, write},
		{"timescale operation match is case-insensitive", "timescaledb_list_hypertables_orders", map[string]interface{}{"operation": "DROP_Continuous_Aggregate"}, write},
		{"timescale unknown operation is write", "timescaledb_list_hypertables_orders", map[string]interface{}{"operation": "frobnicate"}, write},
		{"timescale missing operation is write", "timescaledb_list_hypertables_orders", map[string]interface{}{}, write},
		{"timescale time_series_query is read", "timescaledb_timeseries_query_orders", map[string]interface{}{
			"operation": "time_series_query", "target_table": "metrics", "time_column": "ts", "bucket_interval": "1 hour",
		}, read},
		{"timescale time_series_query injected where is write", "timescaledb_timeseries_query_orders", map[string]interface{}{
			"operation": "time_series_query", "target_table": "metrics", "time_column": "ts", "bucket_interval": "1 hour",
			"where_condition": "1=1; DELETE FROM metrics",
		}, write},
		{"timescale time_series_query missing table is write", "timescaledb_timeseries_query_orders", map[string]interface{}{
			"operation": "time_series_query", "time_column": "ts", "bucket_interval": "1 hour",
		}, write},
		{"timescale analyze_time_series is read", "timescaledb_analyze_timeseries_orders", map[string]interface{}{
			"operation": "analyze_time_series", "target_table": "metrics", "time_column": "ts",
		}, read},
		{"timescale analyze_time_series injected table is write", "timescaledb_analyze_timeseries_orders", map[string]interface{}{
			"operation": "analyze_time_series", "target_table": "metrics; DROP TABLE users", "time_column": "ts",
		}, write},

		// Unknown tools fail closed.
		{"unknown tool is write", "frobnicate_orders", map[string]interface{}{}, write},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, requiredToolScope(tc.tool, tc.arguments))
		})
	}
}

func TestRequiredToolScope_QueryWriteNeedsWrite(t *testing.T) {
	cases := []struct {
		name  string
		tool  string
		query string
		want  string
	}{
		{"per-database insert", "query_orders", "INSERT INTO orders VALUES (1)", "db:write"},
		{"per-database update", "query_orders", "UPDATE orders SET a = 1", "db:write"},
		{"per-database select", "query_orders", "SELECT * FROM orders", "db:read"},
		{"unified insert", "query", "INSERT INTO orders VALUES (1)", "db:write"},
		{"unified select", "query", "SELECT 1", "db:read"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, requiredToolScope(tc.tool, map[string]interface{}{"query": tc.query}))
		})
	}
}

func TestRequiredToolScope_ExplainWrite(t *testing.T) {
	require.Equal(t, "db:write", requiredToolScope("explain_orders", map[string]interface{}{"statement": "INSERT INTO orders VALUES (1)"}))
	require.Equal(t, "db:read", requiredToolScope("explain_orders", map[string]interface{}{"statement": "SELECT 1"}))
}

func TestRequiredToolScope_UnknownToolFailsClosed(t *testing.T) {
	arguments := map[string]interface{}{"query": "SELECT 1"}
	require.Equal(t, "db:write", requiredToolScope("frobnicate_orders", arguments))

	request := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{Name: "frobnicate_orders"},
		Extra:  &sdkmcp.RequestExtra{TokenInfo: &sdkAuth.TokenInfo{Scopes: []string{"db:read"}}},
	}
	require.False(t, toolScopeAllowed(request, arguments))
}

func TestAdaptCortexHandler_ReadTokenCannotWriteViaQuery(t *testing.T) {
	calls := 0
	handler := adaptCortexHandler(func(context.Context, cortexServer.ToolCallRequest) (interface{}, error) {
		calls++
		return &Response{Content: []TextContent{{Type: "text", Text: "ok"}}}, nil
	})
	request := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{Name: "query", Arguments: json.RawMessage(`{"query":"INSERT INTO orders VALUES (1)"}`)},
		Extra:  &sdkmcp.RequestExtra{TokenInfo: &sdkAuth.TokenInfo{Scopes: []string{"db:read"}}},
	}

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Equal(t, 0, calls, "a db:read token must not reach the handler for an INSERT")
}

func TestAdaptCortexHandlerPassesPrincipalToUseCase(t *testing.T) {
	var seen usecase.Caller
	handler := adaptCortexHandler(func(ctx context.Context, _ cortexServer.ToolCallRequest) (interface{}, error) {
		seen = usecase.CallerFromContext(ctx)
		return nil, nil
	})

	authed := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{Name: "list_databases"},
		Extra:  &sdkmcp.RequestExtra{TokenInfo: &sdkAuth.TokenInfo{UserID: "user@example.com", Scopes: []string{"db:read"}}},
	}
	_, err := handler(context.Background(), authed)
	require.NoError(t, err)
	require.Equal(t, "user@example.com", seen.Principal)

	unauthenticated := &sdkmcp.CallToolRequest{Params: &sdkmcp.CallToolParamsRaw{Name: "query"}}
	_, err = handler(context.Background(), unauthenticated)
	require.NoError(t, err)
	require.Empty(t, seen.Principal)
	require.Empty(t, seen.Session)
}

func restrictedRequest(name string, databases []string) *sdkmcp.CallToolRequest {
	return &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{Name: name},
		Extra: &sdkmcp.RequestExtra{TokenInfo: &sdkAuth.TokenInfo{
			UserID: "agent-a",
			Scopes: []string{"db:read"},
			Extra:  map[string]any{"databases": databases},
		}},
	}
}

func TestToolDatabaseAllowed(t *testing.T) {
	tests := []struct {
		name      string
		request   *sdkmcp.CallToolRequest
		arguments map[string]interface{}
		want      bool
	}{
		{"no token info (stdio) is unrestricted", &sdkmcp.CallToolRequest{Params: &sdkmcp.CallToolParamsRaw{Name: "query"}}, map[string]interface{}{"database": "prod"}, true},
		{"token without allowlist (legacy or OIDC) is unrestricted", &sdkmcp.CallToolRequest{Params: &sdkmcp.CallToolParamsRaw{Name: "query"}, Extra: &sdkmcp.RequestExtra{TokenInfo: &sdkAuth.TokenInfo{UserID: "api-key"}}}, map[string]interface{}{"database": "prod"}, true},
		{"empty allowlist fails closed", restrictedRequest("query", []string{}), map[string]interface{}{"database": "prod"}, false},
		{"nil allowlist fails closed", restrictedRequest("query", nil), map[string]interface{}{"database": "prod"}, false},
		{"star allows any database", restrictedRequest("query", []string{"*"}), map[string]interface{}{"database": "prod"}, true},
		{"star among entries allows any database", restrictedRequest("query", []string{"sales", "*"}), map[string]interface{}{"database": "prod"}, true},
		{"star does not allow a missing database argument", restrictedRequest("query", []string{"*"}), map[string]interface{}{}, false},
		{"allowed database", restrictedRequest("query", []string{"sales"}), map[string]interface{}{"database": "sales"}, true},
		{"second listed database", restrictedRequest("query", []string{"sales", "prod"}), map[string]interface{}{"database": "prod"}, true},
		{"database name is case sensitive", restrictedRequest("query", []string{"sales"}), map[string]interface{}{"database": "SALES"}, false},
		{"database name with whitespace is denied", restrictedRequest("query", []string{"sales"}), map[string]interface{}{"database": " sales"}, false},
		{"non-string database argument is denied", restrictedRequest("query", []string{"sales"}), map[string]interface{}{"database": 42}, false},
		{"star literal in arguments is not a wildcard", restrictedRequest("query", []string{"sales"}), map[string]interface{}{"database": "*"}, false},
		{"allowed database on write tool", restrictedRequest("execute", []string{"sales"}), map[string]interface{}{"database": "sales"}, true},
		{"database outside allowlist", restrictedRequest("query", []string{"sales"}), map[string]interface{}{"database": "prod"}, false},
		{"unified tool without database argument", restrictedRequest("query", []string{"sales"}), map[string]interface{}{}, false},
		{"empty database argument", restrictedRequest("query", []string{"sales"}), map[string]interface{}{"database": ""}, false},
		{"per-database tool name cannot bypass the allowlist", restrictedRequest("query_prod", []string{"sales"}), map[string]interface{}{"database": "sales"}, false},
		{"per-database tool name without argument", restrictedRequest("query_sales", []string{"sales"}), map[string]interface{}{}, false},
		{"list_databases has no database target", restrictedRequest("list_databases", []string{"sales"}), map[string]interface{}{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, toolDatabaseAllowed(tc.request, tc.arguments))
		})
	}
}

func TestAdaptCortexHandlerDeniesDatabaseOutsideAllowlist(t *testing.T) {
	called := false
	handler := adaptCortexHandler(func(context.Context, cortexServer.ToolCallRequest) (interface{}, error) {
		called = true
		return nil, nil
	})

	denied := restrictedRequest("query", []string{"sales"})
	denied.Params.Arguments = json.RawMessage(`{"database":"prod","query":"select 1"}`)
	result, err := handler(context.Background(), denied)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.False(t, called, "handler must not run for a database outside the allowlist")
	text, ok := result.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	require.Contains(t, text.Text, "not allowed to use this database")
}

func TestAdaptCortexHandlerScopeAndAllowlistBothApply(t *testing.T) {
	called := false
	handler := adaptCortexHandler(func(context.Context, cortexServer.ToolCallRequest) (interface{}, error) {
		called = true
		return nil, nil
	})

	// The token holds db:read only, so the write tool is refused on scope even
	// though the database is on the allowlist.
	denied := restrictedRequest("execute", []string{"sales"})
	denied.Params.Arguments = json.RawMessage(`{"database":"sales","query":"delete from t"}`)
	result, err := handler(context.Background(), denied)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.False(t, called, "handler must not run when the scope is missing")
	text, ok := result.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	require.Contains(t, text.Text, "lacks the required database scope")
}

// listDatabasesUseCase reports a fixed set of configured database ids.
type listDatabasesUseCase struct {
	UseCaseProvider
	databases []string
}

func (u listDatabasesUseCase) ListDatabases() []string { return u.databases }

// listDatabasesText returns the text list_databases produced.
func listDatabasesText(t *testing.T, result interface{}) string {
	t.Helper()
	content, ok := result.(map[string]interface{})["content"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, content, 1)
	return content[0]["text"].(string)
}

func TestListDatabasesFiltersToAllowlist(t *testing.T) {
	useCase := listDatabasesUseCase{databases: []string{"a", "b"}}
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"allowlist limits the list", withDatabaseAllowlist(context.Background(), []string{"a"}), "Available databases:\n\n1. a\n"},
		{"star lists every database", withDatabaseAllowlist(context.Background(), []string{"*"}), "Available databases:\n\n1. a\n2. b\n"},
		{"no allowlist lists every database", context.Background(), "Available databases:\n\n1. a\n2. b\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := NewListDatabasesTool().HandleRequest(tc.ctx, cortexServer.ToolCallRequest{Name: listDatabasesTool}, "", useCase)
			require.NoError(t, err)
			assert.Equal(t, tc.want, listDatabasesText(t, result))
		})
	}
}

func TestAdaptCortexHandlerLimitsListDatabasesToAllowlist(t *testing.T) {
	useCase := listDatabasesUseCase{databases: []string{"a", "b"}}
	handler := adaptCortexHandler(func(ctx context.Context, request cortexServer.ToolCallRequest) (interface{}, error) {
		return NewListDatabasesTool().HandleRequest(ctx, request, "", useCase)
	})

	result, err := handler(context.Background(), restrictedRequest(listDatabasesTool, []string{"a"}))
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "Available databases:\n\n1. a\n", result.Content[0].(*sdkmcp.TextContent).Text)
}

// readAuditRecords decodes every JSONL record in an audit file.
func readAuditRecords(t *testing.T, path string) []map[string]interface{} {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	var out []map[string]interface{}
	dec := json.NewDecoder(f)
	for dec.More() {
		var rec map[string]interface{}
		require.NoError(t, dec.Decode(&rec))
		out = append(out, rec)
	}
	return out
}

// TestAdaptCortexHandler_AuditsScopeDenial checks that a call refused by the
// tool scope check is audited once, with its tool, target and statement, and
// never reaches the handler.
func TestAdaptCortexHandler_AuditsScopeDenial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	require.NoError(t, usecase.EnableAuditLog(path))
	t.Cleanup(usecase.DisableAuditLog)

	calls := 0
	handler := adaptCortexHandler(func(context.Context, cortexServer.ToolCallRequest) (interface{}, error) {
		calls++
		return nil, nil
	})
	request := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{Name: "query_orders", Arguments: json.RawMessage(`{"query":"INSERT INTO orders VALUES (1)"}`)},
		Extra:  &sdkmcp.RequestExtra{TokenInfo: &sdkAuth.TokenInfo{UserID: "reader", Scopes: []string{"db:read"}}},
	}

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Equal(t, 0, calls)

	recs := readAuditRecords(t, path)
	require.Len(t, recs, 1)
	require.Equal(t, "denied", recs[0]["op"])
	require.Equal(t, "query_orders", recs[0]["tool"])
	require.Equal(t, "orders", recs[0]["database"])
	require.Equal(t, "INSERT INTO orders VALUES (1)", recs[0]["statement"])
	require.Equal(t, "reader", recs[0]["principal"])
}

// TestAgentKey_DatabaseDenialIsAudited checks that a call refused by an agent
// key's database allowlist is audited once, under the key id as principal and
// the tool name, and never reaches the handler.
func TestAgentKey_DatabaseDenialIsAudited(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	require.NoError(t, usecase.EnableAuditLog(path))
	t.Cleanup(usecase.DisableAuditLog)

	calls := 0
	handler := adaptCortexHandler(func(context.Context, cortexServer.ToolCallRequest) (interface{}, error) {
		calls++
		return nil, nil
	})
	request := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{Name: "query", Arguments: json.RawMessage(`{"database":"other","query":"SELECT 1"}`)},
		Extra: &sdkmcp.RequestExtra{TokenInfo: &sdkAuth.TokenInfo{
			UserID: "agent-a",
			Scopes: []string{"db:read"},
			Extra:  map[string]any{"databases": []string{"sales"}},
		}},
	}

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Equal(t, 0, calls)

	recs := readAuditRecords(t, path)
	require.Len(t, recs, 1)
	require.Equal(t, "denied", recs[0]["op"])
	require.Equal(t, "agent-a", recs[0]["principal"])
	require.Equal(t, "query", recs[0]["tool"])
	require.Equal(t, "other", recs[0]["database"])
}

// sqliteBridgeDB adapts an in-memory SQLite handle to domain.Database so the
// bridge test can run a real use case behind the handler.
type sqliteBridgeDB struct {
	db *sql.DB
}

func (s sqliteBridgeDB) Query(ctx context.Context, query string, args ...interface{}) (domain.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}
func (s sqliteBridgeDB) Exec(ctx context.Context, statement string, args ...interface{}) (domain.Result, error) {
	return s.db.ExecContext(ctx, statement, args...)
}
func (s sqliteBridgeDB) Begin(context.Context, *domain.TxOptions) (domain.Tx, error) {
	return nil, errors.New("transactions are not used in this test")
}
func (s sqliteBridgeDB) IsReadOnly() bool { return false }
func (s sqliteBridgeDB) MaxRows() int     { return 0 }

// sqliteBridgeRepo serves one database for every ID.
type sqliteBridgeRepo struct {
	db domain.Database
}

func (r sqliteBridgeRepo) GetDatabase(string) (domain.Database, error) { return r.db, nil }
func (r sqliteBridgeRepo) ListDatabases() []string                     { return nil }
func (r sqliteBridgeRepo) GetDatabaseType(string) (string, error)      { return "sqlite", nil }
func (r sqliteBridgeRepo) IsLazyLoading() bool                         { return false }

// TestAdaptCortexHandler_RecordsToolOnSuccess checks that a call which runs
// through the use case writes a normal query record carrying the tool name and
// the token principal.
func TestAdaptCortexHandler_RecordsToolOnSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	require.NoError(t, usecase.EnableAuditLog(path))
	t.Cleanup(usecase.DisableAuditLog)

	raw, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	uc := usecase.NewDatabaseUseCase(sqliteBridgeRepo{db: sqliteBridgeDB{db: raw}})
	handler := adaptCortexHandler(func(ctx context.Context, _ cortexServer.ToolCallRequest) (interface{}, error) {
		text, err := uc.ExecuteQuery(ctx, "orders", "SELECT 1", nil)
		if err != nil {
			return nil, err
		}
		return &Response{Content: []TextContent{{Type: "text", Text: text}}}, nil
	})
	request := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{Name: "query_orders", Arguments: json.RawMessage(`{"query":"SELECT 1"}`)},
		Extra:  &sdkmcp.RequestExtra{TokenInfo: &sdkAuth.TokenInfo{UserID: "reader", Scopes: []string{"db:read"}}},
	}

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	recs := readAuditRecords(t, path)
	require.Len(t, recs, 1)
	require.Equal(t, "query", recs[0]["op"])
	require.Equal(t, "orders", recs[0]["database"])
	require.Equal(t, "query_orders", recs[0]["tool"])
	require.Equal(t, "reader", recs[0]["principal"])
}
