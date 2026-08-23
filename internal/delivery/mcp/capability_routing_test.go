package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
)

// stubCaps satisfies UseCaseProvider plus the cycle 75-86 capability
// interfaces; each method records its route.
type stubCaps struct {
	stubMaskingUseCase
	route string
}

func (s *stubCaps) SearchValues(_ context.Context, dbID string, needle string) (string, error) {
	s.route = "search:" + needle
	return "ok", nil
}
func (s *stubCaps) RelatedRows(_ context.Context, dbID string, table string, key string) (string, error) {
	s.route = "related:" + table + ":" + key
	return "ok", nil
}
func (s *stubCaps) FindDuplicates(_ context.Context, dbID string, table string, col string) (string, error) {
	s.route = "dupes:" + table + ":" + col
	return "ok", nil
}
func (s *stubCaps) ExecuteScript(_ context.Context, dbID string, script string) (string, error) {
	s.route = "script:" + script
	return "ok", nil
}
func (s *stubCaps) ImportCSV(_ context.Context, _, table, _ string) (string, error) {
	s.route = "csv:" + table
	return "ok", nil
}
func (s *stubCaps) RunMigrations(_ context.Context, _, dir string) (string, error) {
	s.route = "migrate:" + dir
	return "ok", nil
}
func (s *stubCaps) ListViews(_ context.Context, dbID string) (string, error) {
	s.route = "views"
	return "ok", nil
}
func (s *stubCaps) ListTriggers(_ context.Context, dbID string) (string, error) {
	s.route = "triggers"
	return "ok", nil
}
func (s *stubCaps) ListRoutines(_ context.Context, dbID string) (string, error) {
	s.route = "routines"
	return "ok", nil
}
func (s *stubCaps) ListCustomTypes(_ context.Context, dbID string) (string, error) {
	s.route = "types"
	return "ok", nil
}
func (s *stubCaps) DumpDDL(_ context.Context, dbID string) (string, error) {
	s.route = "ddl"
	return "ok", nil
}

func handle(t *testing.T, tool ToolType, params map[string]interface{}) string {
	t.Helper()
	uc := &stubCaps{}
	if _, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name:       tool.GetName() + "_db1",
		Parameters: params,
	}, "", uc); err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	return uc.route
}

// TestCapabilityRouting proves cycles 75-86 delivery wiring end to end.
func TestCapabilityRouting(t *testing.T) {
	cases := []struct {
		name   string
		tool   func() ToolType
		params map[string]interface{}
		want   string
	}{
		{"value_search", func() ToolType { return NewFilterTablesTool() },
			map[string]interface{}{"pattern": "x", "value": "alice"}, "search:alice"},
		{"related_key", func() ToolType { return NewDescribeTool() },
			map[string]interface{}{"table": "books", "related_key": "10"}, "related:books:10"},
		{"duplicates_column", func() ToolType { return NewDescribeTool() },
			map[string]interface{}{"table": "users", "duplicates_column": "email"}, "dupes:users:email"},
		{"script", func() ToolType { return NewExecuteTool() },
			map[string]interface{}{"script": "A; B"}, "script:A; B"},
		{"csv_import", func() ToolType { return NewExecuteTool() },
			map[string]interface{}{"csv_data": "h\n1\n", "csv_table": "t"}, "csv:t"},
		{"migrate_dir", func() ToolType { return NewExecuteTool() },
			map[string]interface{}{"migrate_dir": "/migrations"}, "migrate:/migrations"},
		{"views_format", func() ToolType { return NewSchemaTool() },
			map[string]interface{}{"format": "views"}, "views"},
		{"triggers_format", func() ToolType { return NewSchemaTool() },
			map[string]interface{}{"format": "triggers"}, "triggers"},
		{"routines_format", func() ToolType { return NewSchemaTool() },
			map[string]interface{}{"format": "routines"}, "routines"},
		{"types_format", func() ToolType { return NewSchemaTool() },
			map[string]interface{}{"format": "types"}, "types"},
		{"ddl_format", func() ToolType { return NewSchemaTool() },
			map[string]interface{}{"format": "ddl"}, "ddl"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := handle(t, tc.tool(), tc.params)
			if !strings.HasPrefix(got, tc.want) {
				t.Fatalf("route = %q, want prefix %q", got, tc.want)
			}
		})
	}
}
