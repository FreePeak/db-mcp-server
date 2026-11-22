package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	cortexServer "github.com/FreePeak/cortex/pkg/server"
	cortexTypes "github.com/FreePeak/cortex/pkg/types"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/FreePeak/db-mcp-server/internal/usecase"
)

func convertSDKTool(tool *cortexTypes.Tool) *sdkmcp.Tool {
	properties := make(map[string]interface{})
	required := make([]string, 0)
	for _, parameter := range tool.Parameters {
		if parameter.Name == "" {
			continue
		}
		property := map[string]interface{}{"type": parameter.Type}
		if parameter.Description != "" {
			property["description"] = parameter.Description
		}
		if parameter.Items != nil {
			property["items"] = parameter.Items
		}
		properties[parameter.Name] = property
		if parameter.Required {
			required = append(required, parameter.Name)
		}
	}
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		inputSchema["required"] = required
	}
	return &sdkmcp.Tool{
		Name:        tool.Name,
		Description: tool.Description,
		InputSchema: inputSchema,
	}
}

func adaptCortexHandler(handler func(context.Context, cortexServer.ToolCallRequest) (interface{}, error)) sdkmcp.ToolHandler {
	return func(ctx context.Context, request *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		if request == nil || request.Params == nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "tools/call request is missing parameters"}
		}
		arguments := map[string]interface{}{}
		if len(request.Params.Arguments) > 0 && string(request.Params.Arguments) != "null" {
			if err := json.Unmarshal(request.Params.Arguments, &arguments); err != nil {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "invalid tool arguments: " + err.Error()}
			}
		}
		if !toolScopeAllowed(request, arguments) {
			return &sdkmcp.CallToolResult{
				IsError: true,
				Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "the authenticated principal lacks the required database scope"}},
			}, nil
		}
		if !toolDatabaseAllowed(request, arguments) {
			return &sdkmcp.CallToolResult{
				IsError: true,
				Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "the authenticated principal is not allowed to use this database"}},
			}, nil
		}
		ctx = usecase.WithCaller(ctx, callerFromRequest(request))
		result, err := handler(ctx, cortexServer.ToolCallRequest{
			Name:       request.Params.Name,
			Parameters: arguments,
		})
		if err != nil {
			return &sdkmcp.CallToolResult{
				IsError: true,
				Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: err.Error()}},
			}, nil
		}
		return convertCortexResult(result)
	}
}

func toolScopeAllowed(request *sdkmcp.CallToolRequest, arguments map[string]interface{}) bool {
	if request.Extra == nil || request.Extra.TokenInfo == nil {
		return true
	}
	required := requiredToolScope(request.Params.Name, arguments)
	for _, scope := range request.Extra.TokenInfo.Scopes {
		if scope == required || scope == "db:admin" {
			return true
		}
	}
	return false
}

// listDatabasesTool is the one registered tool that takes no database target.
const listDatabasesTool = "list_databases"

// toolDatabaseAllowed enforces a per-agent database allowlist. Principals
// without an allowlist (stdio, legacy key, OIDC) are unrestricted. A principal
// that carries one is denied anything not listed, so an empty list denies
// everything; "*" lists every database. An allowlisted principal may call only
// the unified tools, with a `database` argument on the allowlist, because
// per-database tool names such as query_<id> carry the target in the name where
// it cannot be checked reliably. list_databases has no target and is always
// allowed.
func toolDatabaseAllowed(request *sdkmcp.CallToolRequest, arguments map[string]interface{}) bool {
	if request.Extra == nil || request.Extra.TokenInfo == nil {
		return true
	}
	raw, scoped := request.Extra.TokenInfo.Extra["databases"]
	if !scoped {
		return true
	}
	name := request.Params.Name
	if name == listDatabasesTool {
		return true
	}
	if !slices.Contains(unifiedDatabaseToolTypes, name) {
		return false
	}
	database, ok := arguments["database"].(string)
	if !ok || database == "" {
		return false
	}
	databases, ok := raw.([]string)
	if !ok {
		return false
	}
	return slices.Contains(databases, allDatabases) || slices.Contains(databases, database)
}

// readOnlyToolTypes are tool types whose every call is read-only.
var readOnlyToolTypes = []string{"describe", "health", "schema", "filter_tables", "list_databases"}

// readOnlyPerformanceActions are the performance_* actions that run no write.
// reset clears in-process history and validate_suggestions runs CREATE
// EXTENSION hypopg, so both need db:write.
var readOnlyPerformanceActions = map[string]bool{
	"stats":                true,
	"slow_queries":         true,
	"engine_slow_queries":  true,
	"suggest":              true,
	"suggest_indexes":      true,
	"workload_suggestions": true,
	"index_health":         true,
	"db_health":            true,
}

// readOnlyTimescaleOperations are the TimescaleDB operations that only read.
// time_series_query and analyze_time_series are classified by their SQL.
var readOnlyTimescaleOperations = map[string]bool{
	"list_hypertables":              true,
	"get_compression_settings":      true,
	"get_retention_policy":          true,
	"list_continuous_aggregates":    true,
	"get_continuous_aggregate_info": true,
}

// requiredToolScope returns the database scope a call needs. Query and
// explain tools are classified by the SQL they run, performance and
// TimescaleDB tools by their action or operation, and execute and transaction
// tools always need db:write. Unknown tools and arguments that cannot be
// classified fail closed to db:write; db:admin is checked by the caller.
func requiredToolScope(name string, arguments map[string]interface{}) string {
	switch {
	case hasToolPrefix(name, "execute"), hasToolPrefix(name, "transaction"):
		return "db:write"
	case hasToolPrefix(name, "query"):
		return sqlScope(getStringParam(arguments, "query"))
	case hasToolPrefix(name, "explain"):
		return sqlScope(getStringParam(arguments, "statement"))
	case hasToolPrefix(name, "performance"):
		action := getStringParam(arguments, "action")
		if readOnlyPerformanceActions[action] {
			return "db:read"
		}
		return "db:write"
	case strings.HasPrefix(name, "timescaledb_"):
		return timescaleScope(arguments)
	}
	for _, base := range readOnlyToolTypes {
		if hasToolPrefix(name, base) {
			return "db:read"
		}
	}
	return "db:write"
}

// sqlScope classifies a statement with the shared write guard. A blank
// statement is treated as missing and fails closed.
func sqlScope(sql string) string {
	if strings.TrimSpace(sql) == "" || usecase.IsWriteStatement(sql) {
		return "db:write"
	}
	return "db:read"
}

// timescaleScope dispatches on the operation argument, which the handler
// uses regardless of the tool name. The operation is matched case-insensitively,
// as the handler does.
func timescaleScope(arguments map[string]interface{}) string {
	operation := strings.ToLower(getStringParam(arguments, "operation"))
	var sql string
	var err error
	switch operation {
	case "time_series_query":
		sql, err = buildTimeSeriesQuerySQL(arguments)
	case "analyze_time_series":
		sql, err = buildTimeSeriesAnalyzeSQL(arguments)
	default:
		if readOnlyTimescaleOperations[operation] {
			return "db:read"
		}
		return "db:write"
	}
	if err != nil {
		return "db:write"
	}
	return sqlScope(sql)
}

// hasToolPrefix reports whether name is the tool type base, optionally
// followed by "_<database id>".
func hasToolPrefix(name, base string) bool {
	return name == base || strings.HasPrefix(name, base+"_")
}

func convertCortexResult(result interface{}) (*sdkmcp.CallToolResult, error) {
	if response, ok := result.(*Response); ok {
		content := make([]sdkmcp.Content, 0, len(response.Content))
		for _, item := range response.Content {
			content = append(content, &sdkmcp.TextContent{Text: item.Text})
		}
		return &sdkmcp.CallToolResult{
			Content: content,
			Meta:    sdkmcp.Meta(response.Metadata),
		}, nil
	}
	if result == nil {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{}}, nil
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal tool result: %w", err)
	}
	var converted sdkmcp.CallToolResult
	if err := json.Unmarshal(encoded, &converted); err != nil {
		return nil, fmt.Errorf("decode tool result: %w", err)
	}
	// Legacy handlers expose metadata under `metadata`; the SDK reserves
	// `_meta` for the wire representation. Preserve transaction IDs and other
	// handler metadata explicitly during the bridge.
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &wire); err == nil {
		if metadata, ok := wire["metadata"]; ok {
			var values map[string]interface{}
			if err := json.Unmarshal(metadata, &values); err == nil {
				converted.Meta = sdkmcp.Meta(values)
			}
		}
	}
	if converted.Content == nil {
		converted.Content = []sdkmcp.Content{}
	}
	return &converted, nil
}

// callerFromRequest maps the authenticated token and MCP session of a tool
// call onto the audit caller. Stdio and unauthenticated calls carry neither.
func callerFromRequest(request *sdkmcp.CallToolRequest) usecase.Caller {
	var caller usecase.Caller
	if request.Extra != nil && request.Extra.TokenInfo != nil {
		caller.Principal = request.Extra.TokenInfo.UserID
	}
	if request.Session != nil {
		caller.Session = request.Session.ID()
	}
	return caller
}
