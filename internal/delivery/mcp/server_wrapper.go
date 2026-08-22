package mcp

import (
	"context"
	"sort"
	"sync"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/FreePeak/cortex/pkg/types"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// ServerWrapper provides a wrapper around server.MCPServer to handle type assertions
type ServerWrapper struct {
	mcpServer *server.MCPServer

	mu              sync.Mutex
	registeredNames map[string]struct{} // every tool name that passed through AddTool
}

// NewServerWrapper creates a new ServerWrapper
func NewServerWrapper(mcpServer *server.MCPServer) *ServerWrapper {
	return &ServerWrapper{
		mcpServer:       mcpServer,
		registeredNames: map[string]struct{}{},
	}
}

// AddTool adds a tool to the server
func (sw *ServerWrapper) AddTool(ctx context.Context, tool interface{}, handler func(ctx context.Context, request server.ToolCallRequest) (interface{}, error)) error {
	// Log the operation for debugging
	logger.Debug("Adding tool: %T", tool)

	// Cast the tool to the expected type (*types.Tool)
	typedTool, ok := tool.(*types.Tool)
	if !ok {
		logger.Warn("Warning: Tool is not of type *types.Tool: %T", tool)
		return nil
	}

	sw.mu.Lock()
	sw.registeredNames[typedTool.Name] = struct{}{}
	sw.mu.Unlock()

	// Pass the tool to the MCPServer's AddTool method
	return sw.mcpServer.AddTool(ctx, typedTool, handler)
}

// ListRegisteredNames returns sorted names of tools that passed through
// AddTool — useful for tests and startup diagnostics.
func (sw *ServerWrapper) ListRegisteredNames() []string {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	out := make([]string, 0, len(sw.registeredNames))
	for n := range sw.registeredNames {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
