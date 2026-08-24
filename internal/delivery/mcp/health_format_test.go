package mcp

import (
	"strings"
	"testing"
)

// TestFormatHealthResult_RendersGuardrails locks in cycle 24's fix: the
// health text formatter must not silently drop guardrail keys that the
// use-case layer reports.
func TestFormatHealthResult_RendersGuardrails(t *testing.T) {
	out := formatHealthResult(map[string]interface{}{
		"database":                  "pg1",
		"healthy":                   true,
		"ping_ms":                   1.5,
		"read_only":                 true,
		"max_rows":                  1000,
		"statement_timeout_seconds": 30,
	})
	for _, want := range []string{"read_only: true", "max_rows: 1000", "statement_timeout_seconds: 30"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in health output, got:\n%s", want, out)
		}
	}
}

func TestFormatHealthResult_OmitsZeroValuedGuardrails(t *testing.T) {
	out := formatHealthResult(map[string]interface{}{
		"database": "db1",
		"healthy":  true,
	})
	if strings.Contains(out, "max_rows") || strings.Contains(out, "statement_timeout_seconds") {
		t.Errorf("unset guardrails should be omitted, got:\n%s", out)
	}
}
