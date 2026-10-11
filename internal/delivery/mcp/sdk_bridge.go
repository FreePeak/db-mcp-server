package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	cortexServer "github.com/FreePeak/cortex/pkg/server"
	cortexTypes "github.com/FreePeak/cortex/pkg/types"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
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
		if !toolScopeAllowed(request) {
			return &sdkmcp.CallToolResult{
				IsError: true,
				Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "the authenticated principal lacks the required database scope"}},
			}, nil
		}
		arguments := map[string]interface{}{}
		if len(request.Params.Arguments) > 0 && string(request.Params.Arguments) != "null" {
			if err := json.Unmarshal(request.Params.Arguments, &arguments); err != nil {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "invalid tool arguments: " + err.Error()}
			}
		}
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

func toolScopeAllowed(request *sdkmcp.CallToolRequest) bool {
	if request.Extra == nil || request.Extra.TokenInfo == nil {
		return true
	}
	required := requiredToolScope(request.Params.Name)
	for _, scope := range request.Extra.TokenInfo.Scopes {
		if scope == required || scope == "db:admin" {
			return true
		}
	}
	return false
}

func requiredToolScope(name string) string {
	if name == "execute" || strings.HasPrefix(name, "execute_") || name == "transaction" || strings.HasPrefix(name, "transaction_") {
		return "db:write"
	}
	return "db:read"
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
