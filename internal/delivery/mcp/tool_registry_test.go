package mcp

import (
	"context"
	"log"
	"strings"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
)

func newTestRegistry(t *testing.T, unified bool) (*ToolRegistry, UseCaseProvider) {
	t.Helper()
	mcpServer := server.NewMCPServer("test", "1.0.0", log.Default())
	tr := NewToolRegistry(mcpServer, unified)
	uc := &stubMaskingUseCase{} // implements full UseCaseProvider surface
	return tr, uc
}

// TestRegisterAllTools_PerDatabaseMode proves every base tool type gets a
// per-database tool for each listed database.
func TestRegisterAllTools_PerDatabaseMode(t *testing.T) {
	tr, uc := newTestRegistry(t, false)
	if err := tr.RegisterAllTools(context.Background(), uc); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	registered := tr.server.ListRegisteredNames()
	want := []string{
		"query_db1", "execute_db1", "transaction_db1",
		"performance_db1", "explain_db1", "describe_db1",
		"health_db1", "schema_db1",
	}
	for _, w := range want {
		if !containsName(registered, w) {
			t.Errorf("missing per-db tool %q; registered: %v", w, registered)
		}
	}
}

// TestRegisterAllTools_UnifiedMode proves unified mode registers the shared
// tool set instead of per-database duplicates.
func TestRegisterAllTools_UnifiedMode(t *testing.T) {
	tr, uc := newTestRegistry(t, true)
	if err := tr.RegisterAllTools(context.Background(), uc); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	registered := tr.server.ListRegisteredNames()
	for _, w := range []string{"query", "execute", "transaction", "performance"} {
		if !containsExact(registered, w) {
			t.Errorf("missing unified tool %q; registered: %v", w, registered)
		}
	}
	for _, n := range registered {
		if strings.HasSuffix(n, "_db1") && !strings.HasPrefix(n, "filter") {
			t.Errorf("per-db suffix leaked into unified mode: %q", n)
		}
	}
}

// TestExtractAndValidateDatabase covers the unified-mode database parameter
// validation: known IDs pass, unknown/missing fail.
func TestExtractAndValidateDatabase(t *testing.T) {
	dbList := []string{"db1", "db2"}
	req := func(params map[string]interface{}) server.ToolCallRequest {
		return server.ToolCallRequest{Parameters: params}
	}

	got, err := extractAndValidateDatabase(req(map[string]interface{}{"database": "db1"}), dbList)
	if err != nil || got != "db1" {
		t.Fatalf("valid database rejected: %q %v", got, err)
	}
	if _, err := extractAndValidateDatabase(req(map[string]interface{}{"database": "nope"}), dbList); err == nil {
		t.Fatal("unknown database must be rejected")
	}
	if _, err := extractAndValidateDatabase(req(map[string]interface{}{}), dbList); err == nil {
		t.Fatal("missing database must be rejected")
	}
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

func containsExact(names []string, want string) bool { return containsName(names, want) }
