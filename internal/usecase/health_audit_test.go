package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

// TestRenderHealthAudit proves the combiner renders successful
// sections inline, collapses engine-unsupported checks silently,
// keeps genuine errors visible, and counts warnings in the summary.
func TestRenderHealthAudit(t *testing.T) {
	results := []auditResult{
		{name: "crash_safety", out: "WARNING: fsync off", err: nil},
		{name: "wal_level", out: "", err: errors.New("introspection is not available for engine \"mysql\"")},
		{name: "buffer_pool", out: "", err: errors.New("catalog query failed: connection refused")},
		{name: "slow_query_log", out: "WARNING: disabled", err: nil},
	}
	out := renderHealthAudit("db1", "mysql", results)

	if !strings.HasPrefix(out, "Health audit for db1 (mysql)") {
		t.Fatalf("missing header:\n%s", out)
	}
	if !strings.Contains(out, "2 warning(s)") {
		t.Fatalf("warning count wrong:\n%s", out)
	}
	if !strings.Contains(out, "== crash_safety ==") || !strings.Contains(out, "fsync off") {
		t.Fatalf("success section missing:\n%s", out)
	}
	if !strings.Contains(out, "== buffer_pool ==") || !strings.Contains(out, "connection refused") {
		t.Fatalf("genuine error must stay visible:\n%s", out)
	}
	if strings.Contains(out, "wal_level") {
		t.Fatalf("engine-unsupported check must be omitted:\n%s", out)
	}
}

// failingDB errors on every query, simulating an unreachable engine.
type failingDB struct{ domain.Database }

// TestRunHealthAudit_DegradesGracefully proves one broken catalog never
// fails the whole report: unreachable checks render as error sections,
// unsupported ones vanish, and the call returns a usable report.
func TestRunHealthAudit_DegradesGracefully(t *testing.T) {
	uc := NewDatabaseUseCase(&fakeRepo{db: failingDB{}, dbType: "postgres"})
	out, err := uc.RunHealthAudit(context.Background(), "db1")
	if err != nil {
		t.Fatalf("combined audit must not fail wholesale: %v", err)
	}
	if !strings.Contains(out, "Health audit for db1 (postgres)") {
		t.Fatalf("missing header:\n%s", out)
	}
	if !strings.Contains(out, "0 warning(s)") {
		t.Fatalf("expected zero-warning summary:\n%s", out)
	}
}
