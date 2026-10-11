package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FreePeak/cortex/pkg/server"
)

// stubToolType is a minimal ToolType registered under a chosen name; the
// resolution tests only need identity, never a live handler.
type stubToolType struct {
	name string
}

func (s *stubToolType) GetName() string              { return s.name }
func (s *stubToolType) GetDescription(string) string { return s.name }
func (s *stubToolType) CreateTool(string, string) interface{} {
	return nil
}
func (s *stubToolType) CreateUnifiedTool(string, []string) interface{} { return nil }
func (s *stubToolType) HandleRequest(context.Context, server.ToolCallRequest, string, UseCaseProvider) (interface{}, error) {
	return nil, nil
}

// Regression coverage for FreePeak/db-mcp-server#89.
//
// GetToolType split the name on "_" and looked up the FIRST segment, so any
// registered tool type whose name starts with another registered type's name
// plus an underscore ("widget_list" vs "widget") resolved to the shorter one.
// With "list_databases" and "list" both registered that silently mapped
// list_databases onto the directory-listing tool.
//
// The stub pair below reproduces the collision against the live factory, so
// this test fails on the pre-fix resolution order regardless of which
// built-in tool types happen to be registered.
func TestGetToolType_ExactNameWinsOverPrefixCollision(t *testing.T) {
	factory := NewToolTypeFactory()
	factory.Register(&stubToolType{name: "widget"})
	factory.Register(&stubToolType{name: "widget_list"})

	got, ok := factory.GetToolType("widget_list")
	require.True(t, ok, "widget_list must resolve")
	assert.Equal(t, "widget_list", got.GetName(),
		"an exact name must win over a shorter registered prefix")

	// The <tooltype>_<dbID> form still resolves through the prefix split.
	suffixed, ok := factory.GetToolType("widget_db1")
	require.True(t, ok, "widget_db1 must resolve to the widget tool type")
	assert.Equal(t, "widget", suffixed.GetName())
}

// The built-in registry keeps its documented resolution: a bare tool type
// name, the <tooltype>_<dbID> form, and the multi-word global tool.
func TestGetToolType_BuiltinNames(t *testing.T) {
	factory := NewToolTypeFactory()

	for _, tt := range []struct {
		lookup   string
		wantType string
	}{
		{lookup: "list_databases", wantType: "list_databases"},
		{lookup: "filter_tables", wantType: "filter_tables"},
		{lookup: "query", wantType: "query"},
		{lookup: "query_db1", wantType: "query"},
	} {
		t.Run(tt.lookup, func(t *testing.T) {
			got, ok := factory.GetToolType(tt.lookup)
			require.True(t, ok, "expected %q to resolve", tt.lookup)
			assert.Equal(t, tt.wantType, got.GetName())
		})
	}

	_, ok := factory.GetToolType("not_a_registered_tool")
	assert.False(t, ok)
}
