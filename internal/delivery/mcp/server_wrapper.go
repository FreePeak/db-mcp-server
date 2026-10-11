package mcp

import (
	"context"
	"fmt"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/FreePeak/cortex/pkg/types"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

const (
	serverName    = "DB MCP Server"
	serverVersion = "1.0.0"
)

// ServerWrapper provides a wrapper around server.MCPServer to handle type assertions
// and dual-registers tools with the maintained Streamable HTTP SDK.
type ServerWrapper struct {
	mcpServer *server.MCPServer
	sdkServer *sdkmcp.Server
}

// NewServerWrapper creates a new ServerWrapper
func NewServerWrapper(mcpServer *server.MCPServer) *ServerWrapper {
	sdkServer := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: serverName, Version: serverVersion},
		&sdkmcp.ServerOptions{Capabilities: &sdkmcp.ServerCapabilities{
			Tools:     &sdkmcp.ToolCapabilities{ListChanged: true},
			Prompts:   &sdkmcp.PromptCapabilities{ListChanged: true},
			Resources: &sdkmcp.ResourceCapabilities{ListChanged: true},
			Logging:   &sdkmcp.LoggingCapabilities{},
		}},
	)
	return &ServerWrapper{
		mcpServer: mcpServer,
		sdkServer: sdkServer,
	}
}

// AddTool adds a tool to the server
func (sw *ServerWrapper) AddTool(ctx context.Context, tool interface{}, handler func(ctx context.Context, request server.ToolCallRequest) (interface{}, error)) error {
	// Log the operation for debugging
	logger.Debug("Adding tool: %T", tool)

	// Cast the tool to the expected type (*types.Tool)
	typedTool, ok := tool.(*types.Tool)
	if !ok {
		return fmt.Errorf("tool %q has unexpected type %T", nameFromTool(tool), tool)
	}

	// Pass the tool to the MCPServer's AddTool method
	if err := sw.mcpServer.AddTool(ctx, typedTool, handler); err != nil {
		return err
	}
	sw.sdkServer.AddTool(convertSDKTool(typedTool), adaptCortexHandler(handler))
	return nil
}

// SDKServer returns the maintained Streamable HTTP server populated with the
// same tools and handlers as the legacy Cortex server.
func (sw *ServerWrapper) SDKServer() *sdkmcp.Server {
	return sw.sdkServer
}

func nameFromTool(tool interface{}) string {
	if typedTool, ok := tool.(*types.Tool); ok {
		return typedTool.Name
	}
	return "<unknown>"
}
