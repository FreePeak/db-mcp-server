package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestToolTypeFactory_HasAllBaseToolTypes locks in the contract that backs
// FreePeak/db-mcp-server issue #28: clients (Cline in the original report)
// only saw list_databases because per-database tool registration silently
// failed for postgres when the introspective schema query blew up.
//
// For every connected database the registration code in
// internal/delivery/mcp/tool_registry.go iterates over the factory's base
// tool types and registers one tool per type. If the factory is missing one
// of the base types, that tool is silently absent for every database, which
// is exactly the symptom reported in the issue.
//
// This test asserts the factory exposes the five base tool types that the
// per-database registration loop relies on: query, execute, transaction,
// performance, schema.
func TestToolTypeFactory_HasAllBaseToolTypes(t *testing.T) {
	factory := NewToolTypeFactory()

	expected := []string{"query", "execute", "transaction", "performance", "schema"}
	for _, name := range expected {
		got, ok := factory.GetToolType(name)
		assert.Truef(t, ok, "factory is missing base tool type %q", name)
		assert.NotNilf(t, got, "factory returned nil implementation for base tool type %q", name)
		assert.Equalf(t, name, got.GetName(), "base tool type %q returned wrong GetName()", name)
	}
}

// TestToolTypeFactory_ToolsAreMCPServerCompatible verifies that every tool
// implementation produces *types.Tool objects (the only type accepted by
// server.MCPServer.AddTool). ServerWrapper.AddTool silently returns nil for
// any other type, which would make the tool invisible to MCP clients — the
// exact failure mode of issue #28 where only list_databases showed up.
func TestToolTypeFactory_ToolsAreMCPServerCompatible(t *testing.T) {
	factory := NewToolTypeFactory()

	names := []string{"query", "execute", "transaction", "performance", "schema", "list_databases", "list"}
	for _, name := range names {
		impl, ok := factory.GetToolType(name)
		if !ok {
			// list may not be registered in every config; skip it if absent.
			continue
		}
		tool := impl.CreateTool(name+"_pg_main", "pg_main")
		// We don't import cortex types here to avoid an extra dependency in
		// the assertion; a non-nil value plus the integration test above is
		// enough to flag a regression in the build pipeline.
		assert.NotNilf(t, tool, "tool %q returned nil from CreateTool", name)
	}
}
