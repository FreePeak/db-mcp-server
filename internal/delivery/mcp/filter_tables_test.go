package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/FreePeak/cortex/pkg/server"
)

// fakeFilterUseCase satisfies UseCaseProvider for the filter_tables tool by
// returning a canned schema with a mix of prefixed and non-prefixed tables
// — the exact shape WordPress wp_*, Drupal pre_*, and similar CMS installs
// produce.
type fakeFilterUseCase struct {
	mock.Mock
}

func (f *fakeFilterUseCase) ExecuteQuery(ctx context.Context, dbID, query string, params []interface{}) (string, error) {
	args := f.Called(ctx, dbID, query, params)
	return args.String(0), args.Error(1)
}
func (f *fakeFilterUseCase) ExecuteStatement(ctx context.Context, dbID, statement string, params []interface{}) (string, error) {
	args := f.Called(ctx, dbID, statement, params)
	return args.String(0), args.Error(1)
}
func (f *fakeFilterUseCase) ExecuteTransaction(ctx context.Context, dbID, action string, txID string, statement string, params []interface{}, readOnly bool) (string, map[string]interface{}, error) {
	args := f.Called(ctx, dbID, action, txID, statement, params, readOnly)
	return args.String(0), args.Get(1).(map[string]interface{}), args.Error(2)
}
func (f *fakeFilterUseCase) GetDatabaseInfo(dbID string) (map[string]interface{}, error) {
	args := f.Called(dbID)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}
func (f *fakeFilterUseCase) ListDatabases() []string {
	args := f.Called()
	return args.Get(0).([]string)
}
func (f *fakeFilterUseCase) GetDatabaseType(dbID string) (string, error) {
	args := f.Called(dbID)
	return args.String(0), args.Error(1)
}
func (f *fakeFilterUseCase) IsLazyLoading() bool { return true }

func sampleWPInfo() map[string]interface{} {
	return map[string]interface{}{
		"database": "wp_main",
		"tables": []map[string]interface{}{
			{"table_name": "wp_users"},
			{"table_name": "wp_posts"},
			{"table_name": "wp_postmeta"},
			{"table_name": "pre_custom"},
			{"table_name": "django_migrations"},
		},
	}
}

// TestFilterTablesTool_MatchesPrefix locks in the contract that backs
// FreePeak/db-mcp-server issue #54: substring matching must be
// case-insensitive and return every table whose name contains the pattern,
// regardless of CMS prefix style.
func TestFilterTablesTool_MatchesPrefix(t *testing.T) {
	uc := &fakeFilterUseCase{}
	uc.On("GetDatabaseInfo", "wp_main").Return(sampleWPInfo(), nil)

	tool := NewFilterTablesTool()
	resp, err := tool.HandleRequest(
		context.Background(),
		server.ToolCallRequest{Name: "filter_tables", Parameters: map[string]interface{}{"pattern": "wp_"}},
		"wp_main",
		uc,
	)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	respMap, ok := resp.(map[string]interface{})
	assert.True(t, ok, "response must be a content envelope")
	content, ok := respMap["content"].([]map[string]interface{})
	assert.True(t, ok, "content must be a slice of maps")
	assert.NotEmpty(t, content)
	text := content[0]["text"].(string)
	assert.Contains(t, text, "wp_users")
	assert.Contains(t, text, "wp_posts")
	assert.Contains(t, text, "wp_postmeta")
	assert.NotContains(t, text, "pre_custom")
	assert.NotContains(t, text, "django_migrations")
	assert.Contains(t, text, "Matched 3 of 5 tables")
}

// TestFilterTablesTool_CaseInsensitive confirms the matcher is case
// insensitive, which is the mcp-alchemy behaviour the reporter linked.
func TestFilterTablesTool_CaseInsensitive(t *testing.T) {
	uc := &fakeFilterUseCase{}
	uc.On("GetDatabaseInfo", "wp_main").Return(sampleWPInfo(), nil)

	tool := NewFilterTablesTool()
	resp, err := tool.HandleRequest(
		context.Background(),
		server.ToolCallRequest{Name: "filter_tables", Parameters: map[string]interface{}{"pattern": "DJANGO"}},
		"wp_main",
		uc,
	)
	assert.NoError(t, err)

	text := resp.(map[string]interface{})["content"].([]map[string]interface{})[0]["text"].(string)
	assert.True(t, strings.Contains(text, "django_migrations"), "expected DJANGO to match django_migrations case-insensitively")
}

// TestFilterTablesTool_MissingPattern covers the error path: a request
// without a pattern parameter must return a clear error rather than
// silently matching everything.
func TestFilterTablesTool_MissingPattern(t *testing.T) {
	uc := &fakeFilterUseCase{}
	tool := NewFilterTablesTool()
	_, err := tool.HandleRequest(
		context.Background(),
		server.ToolCallRequest{Name: "filter_tables", Parameters: map[string]interface{}{}},
		"wp_main",
		uc,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pattern")
}
