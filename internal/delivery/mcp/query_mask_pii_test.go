package mcp

import (
	"context"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nilProvider satisfies the parts of UseCaseProvider this test never calls,
// so the stubs below only have to define what the dispatch decision touches.
type nilProvider struct{}

func (nilProvider) ExecuteQuery(context.Context, string, string, []interface{}) (string, error) {
	return "", nil
}
func (nilProvider) ExecuteStatement(context.Context, string, string, []interface{}) (string, error) {
	return "", nil
}
func (nilProvider) ExecuteTransaction(context.Context, string, string, string, string, []interface{}, bool) (string, map[string]interface{}, error) {
	return "", nil, nil
}
func (nilProvider) GetDatabaseInfo(string) (map[string]interface{}, error) { return nil, nil }
func (nilProvider) ListDatabases() []string                                { return nil }
func (nilProvider) GetDatabaseType(string) (string, error)                 { return "sqlite", nil }
func (nilProvider) IsLazyLoading() bool                                    { return false }
func (nilProvider) ExecuteExplain(context.Context, string, string, bool) (string, error) {
	return "", nil
}
func (nilProvider) DescribeTable(context.Context, string, string) (map[string]interface{}, error) {
	return nil, nil
}
func (nilProvider) AnalyzePerformance(context.Context, string, string, string, int, int) (string, error) {
	return "", nil
}
func (nilProvider) HealthCheck(context.Context, string) (map[string]interface{}, error) {
	return nil, nil
}
func (nilProvider) RelationshipGraph(context.Context, string) (string, error) { return "", nil }

// stubMaskCapableUseCase implements UseCaseProvider plus ExecuteQueryMasked,
// recording which path the query tool dispatches to.
type stubMaskCapableUseCase struct {
	nilProvider
	maskedCalls int
	lastMask    bool
}

func (s *stubMaskCapableUseCase) ExecuteQuery(_ context.Context, _, _ string, _ []interface{}) (string, error) {
	return "", nil
}

func (s *stubMaskCapableUseCase) ExecuteQueryMasked(_ context.Context, _, _ string, _ []interface{}, mask bool) (string, error) {
	s.maskedCalls++
	s.lastMask = mask
	if mask {
		return "Results:\n\nbody\n----\n[EMAIL]\n\nTotal rows: 1", nil
	}
	return "Results:\n\nbody\n----\nalice@example.com\n\nTotal rows: 1", nil
}

// stubPlainUseCase implements only UseCaseProvider: the legacy shape, which
// must keep working (capability detection, not a hard interface change).
type stubPlainUseCase struct {
	nilProvider
	calls int
}

func (s *stubPlainUseCase) ExecuteQuery(_ context.Context, _, _ string, _ []interface{}) (string, error) {
	s.calls++
	return "plain path", nil
}

func TestQueryTool_MaskPIIRoutesToMaskedPath(t *testing.T) {
	uc := &stubMaskCapableUseCase{}
	tool := &QueryTool{BaseToolType: BaseToolType{name: "query"}}

	for _, maskParam := range []interface{}{true, false} {
		resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
			Name: "query_db1",
			Parameters: map[string]interface{}{
				"query":    "SELECT body FROM tickets",
				"mask_pii": maskParam,
			},
		}, "db1", uc)
		require.NoError(t, err)
		require.NotNil(t, resp)
	}

	assert.Equal(t, 2, uc.maskedCalls, "query tool must use the masking-capable path when available")
	assert.False(t, uc.lastMask, "the second call passed mask_pii=false")
}

func TestQueryTool_MaskPIIRedacts(t *testing.T) {
	uc := &stubMaskCapableUseCase{}
	tool := &QueryTool{BaseToolType: BaseToolType{name: "query"}}

	resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name: "query_db1",
		Parameters: map[string]interface{}{
			"query":    "SELECT body FROM tickets",
			"mask_pii": true,
		},
	}, "db1", uc)
	require.NoError(t, err)

	text := responseText(t, resp)
	assert.Contains(t, text, "[EMAIL]")
	assert.NotContains(t, text, "alice@example.com")
}

// TestQueryTool_LegacyProviderUnaffected pins the fallback: a provider
// without ExecuteQueryMasked still serves queries.
func TestQueryTool_LegacyProviderUnaffected(t *testing.T) {
	uc := &stubPlainUseCase{}
	tool := &QueryTool{BaseToolType: BaseToolType{name: "query"}}

	resp, err := tool.HandleRequest(context.Background(), server.ToolCallRequest{
		Name:       "query_db1",
		Parameters: map[string]interface{}{"query": "SELECT 1"},
	}, "db1", uc)
	require.NoError(t, err)
	assert.Equal(t, 1, uc.calls)
	assert.Contains(t, responseText(t, resp), "plain path")
}

// responseText extracts the text payload from whatever shape the delivery
// layer returns.
func responseText(t *testing.T, resp interface{}) string {
	t.Helper()
	switch v := resp.(type) {
	case string:
		return v
	case map[string]interface{}:
		// createTextResponse returns []map[string]interface{} under "content".
		switch c := v["content"].(type) {
		case []map[string]interface{}:
			if len(c) > 0 {
				if s, ok := c[0]["text"].(string); ok {
					return s
				}
			}
		case []interface{}:
			if len(c) > 0 {
				if m, ok := c[0].(map[string]interface{}); ok {
					if s, ok := m["text"].(string); ok {
						return s
					}
				}
			}
		}
	}
	t.Fatalf("unexpected response shape %T: %#v", resp, resp)
	return ""
}
