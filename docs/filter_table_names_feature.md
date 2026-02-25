# Implementation Plan: filter_table_names Tool

## Issue Reference
[GitHub Issue #54](https://github.com/FreePeak/db-mcp-server/issues/54) - Add filter_table_names tool for substring-based table name search

## Scope

### Problem
Users need to search for tables by substring matching without retrieving all tables and manually scanning. When working with databases that have many tables (especially those with common prefixes like `wp_`, `pre_`, `django_`), it's often necessary to quickly find tables related to a specific domain or feature.

### Files
- `internal/delivery/mcp/tool_types.go` - add new tool type
- `internal/delivery/mcp/tool_registry.go` - register tool
- `internal/usecase/database_usecase.go` - add filtering logic
- `pkg/dbtools/schema.go` - optional: extend strategy pattern

### Out-of-scope
- Changes to existing schema tool
- Database connection management
- Performance optimization for large table lists

## Acceptance Criteria

- [ ] AC1: Tool `filter_table_names_<db_id>` is registered for each database
- [ ] AC2: Tool accepts `pattern` parameter for substring matching (case-insensitive)
- [ ] AC3: Tool returns matching table names with their schema info
- [ ] AC4: Tool works across all supported databases (MySQL, PostgreSQL, SQLite, Oracle)
- [ ] AC5: Tool returns empty list with message if no matches found
- [ ] AC6: Unit tests for the new tool

---

## Implementation Steps

### 1. Add FilterTableNamesTool in `internal/delivery/mcp/tool_types.go`

**Location**: After `SchemaTool` implementation (line 477)

**Changes**:

```go
//------------------------------------------------------------------------------
// FilterTableNamesTool implementation
//------------------------------------------------------------------------------

// FilterTableNamesTool handles table name filtering by substring
type FilterTableNamesTool struct {
    BaseToolType
}

// NewFilterTableNamesTool creates a new filter table names tool type
func NewFilterTableNamesTool() *FilterTableNamesTool {
    return &FilterTableNamesTool{
        BaseToolType: BaseToolType{
            name:        "filter_table_names",
            description: "Filter table names by substring pattern",
        },
    }
}

// CreateTool creates a filter table names tool
func (t *FilterTableNamesTool) CreateTool(name string, dbID string) interface{} {
    return tools.NewTool(
        name,
        tools.WithDescription(t.GetDescription(dbID)),
        tools.WithString("pattern",
            tools.Description("Substring pattern to search for in table names (case-insensitive)"),
            tools.Required(),
        ),
    )
}

// HandleRequest handles filter table names tool requests
func (t *FilterTableNamesTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
    if dbID == "" {
        dbID = extractDatabaseIDFromName(request.Name)
    }

    pattern, ok := request.Parameters["pattern"].(string)
    if !ok || pattern == "" {
        return nil, fmt.Errorf("pattern parameter is required")
    }

    matchingTables, err := useCase.FilterTableNames(ctx, dbID, pattern)
    if err != nil {
        return nil, err
    }

    var output strings.Builder
    if len(matchingTables) == 0 {
        output.WriteString(fmt.Sprintf("No tables found matching pattern '%s'\n", pattern))
    } else {
        output.WriteString(fmt.Sprintf("Found %d tables matching '%s':\n\n", len(matchingTables), pattern))
        for i, table := range matchingTables {
            output.WriteString(fmt.Sprintf("%d. %s\n", i+1, table))
        }
    }

    return createTextResponse(output.String()), nil
}
```

### 2. Add FilterTableNames to UseCaseProvider Interface

**Location**: `internal/delivery/mcp/tool_types.go:64-72`

**Changes**: Add new method to interface

```go
type UseCaseProvider interface {
    ExecuteQuery(ctx context.Context, dbID, query string, params []interface{}) (string, error)
    ExecuteStatement(ctx context.Context, dbID, statement string, params []interface{}) (string, error)
    ExecuteTransaction(ctx context.Context, dbID, action string, txID string, statement string, params []interface{}, readOnly bool) (string, map[string]interface{}, error)
    GetDatabaseInfo(dbID string) (map[string]interface{}, error)
    ListDatabases() []string
    GetDatabaseType(dbID string) (string, error)
    IsLazyLoading() bool
    FilterTableNames(ctx context.Context, dbID, pattern string) ([]string, error)  // NEW
}
```

### 3. Implement FilterTableNames in DatabaseUseCase

**Location**: `internal/usecase/database_usecase.go` (after `GetDatabaseInfo` method)

**Changes**: Add new method

```go
// FilterTableNames returns table names matching the given pattern
func (uc *DatabaseUseCase) FilterTableNames(ctx context.Context, dbID, pattern string) ([]string, error) {
    info, err := uc.GetDatabaseInfo(dbID)
    if err != nil {
        return nil, fmt.Errorf("failed to get database info: %w", err)
    }

    tables, ok := info["tables"].([]map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("invalid tables data format")
    }

    patternLower := strings.ToLower(pattern)
    var matches []string

    for _, table := range tables {
        tableName, ok := table["table_name"].(string)
        if !ok {
            tableName, ok = table["TABLE_NAME"].(string)
        }
        if ok && strings.Contains(strings.ToLower(tableName), patternLower) {
            matches = append(matches, tableName)
        }
    }

    sort.Strings(matches)
    return matches, nil
}
```

### 4. Register Tool in ToolTypeFactory

**Location**: `internal/delivery/mcp/tool_types.go:535-549`

**Changes**: Add registration in `NewToolTypeFactory`

```go
func NewToolTypeFactory() *ToolTypeFactory {
    factory := &ToolTypeFactory{
        toolTypes: make(map[string]ToolType),
    }

    factory.Register(NewQueryTool())
    factory.Register(NewExecuteTool())
    factory.Register(NewTransactionTool())
    factory.Register(NewPerformanceTool())
    factory.Register(NewSchemaTool())
    factory.Register(NewListDatabasesTool())
    factory.Register(NewListDirectoryTool())
    factory.Register(NewFilterTableNamesTool())  // NEW

    return factory
}
```

### 5. Add Tool to Registration List

**Location**: `internal/delivery/mcp/tool_registry.go:72-74`

**Changes**: Add to `toolTypeNames` slice

```go
toolTypeNames := []string{
    "query", "execute", "transaction", "performance", "schema", "filter_table_names",
}
```

### 6. Add Unit Tests

**Location**: Create new file `internal/delivery/mcp/filter_table_names_test.go`

```go
package mcp

import (
    "context"
    "testing"

    "github.com/FreePeak/cortex/pkg/server"
)

// MockUseCase for testing
type mockFilterUseCase struct {
    tables []string
    err    error
}

func (m *mockFilterUseCase) FilterTableNames(_ context.Context, _, _ string) ([]string, error) {
    return m.tables, m.err
}

// ... implement other interface methods

func TestFilterTableNamesTool_HandleRequest(t *testing.T) {
    tests := []struct {
        name       string
        pattern    string
        tables     []string
        wantCount  int
        wantErr    bool
    }{
        {
            name:      "match found",
            pattern:   "wp_",
            tables:    []string{"wp_users", "wp_posts", "users"},
            wantCount: 2,
            wantErr:   false,
        },
        {
            name:      "no match",
            pattern:   "xyz",
            tables:    []string{"users", "posts"},
            wantCount: 0,
            wantErr:   false,
        },
        {
            name:      "case insensitive",
            pattern:   "WP_",
            tables:    []string{"wp_users", "WP_Posts", "users"},
            wantCount: 2,
            wantErr:   false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

---

## Estimated Changes

| File | Lines Changed | Description |
|------|---------------|-------------|
| `internal/delivery/mcp/tool_types.go` | +60 | New tool type + factory registration |
| `internal/delivery/mcp/tool_registry.go` | +1 | Add to tool type list |
| `internal/usecase/database_usecase.go` | +25 | Add FilterTableNames method |
| `internal/delivery/mcp/filter_table_names_test.go` | +100 | Unit tests (new file) |

**Total**: ~186 lines added

---

## API Examples

### Request Example

```json
{
  "name": "filter_table_names_postgres1",
  "parameters": {
    "pattern": "wp_"
  }
}
```

### Response Example (Matches Found)

```json
{
  "content": [
    {
      "type": "text",
      "text": "Found 3 tables matching 'wp_':\n\n1. wp_comments\n2. wp_posts\n3. wp_users\n"
    }
  ]
}
```

### Response Example (No Matches)

```json
{
  "content": [
    {
      "type": "text",
      "text": "No tables found matching pattern 'xyz'\n"
    }
  ]
}
```

---

## Build Verification

After implementation, run:

```bash
# Build check
go build ./...

# Type check
go vet ./...

# Lint check
golangci-lint run ./...

# Run tests
go test -v ./internal/delivery/mcp -run TestFilter
go test -v ./internal/usecase -run TestFilter
```

---

## Status

- [x] Plan created
- [x] Implementation started
- [x] Unit tests passing
- [x] Build verification complete
- [x] Ready for review

## Completion Summary

Implementation completed successfully. The following changes were made:

| File | Lines Changed | Description |
|------|---------------|-------------|
| `internal/delivery/mcp/tool_types.go` | +62 | New FilterTableNamesTool + interface update + factory registration |
| `internal/delivery/mcp/tool_registry.go` | +1 | Add to tool type list |
| `internal/usecase/database_usecase.go` | +21 | Add FilterTableNames method |
| `internal/delivery/mcp/filter_table_names_test.go` | +124 | Unit tests (new file) |
| `internal/delivery/mcp/mock_test.go` | +5 | Add mock method |
| `internal/delivery/mcp/list_tool_test.go` | +5 | Add mock method |
| `internal/delivery/mcp/timescale_tools_test.go` | +5 | Add mock method |
| `internal/delivery/mcp/context/timescale_context_test.go` | +5 | Add mock method |
