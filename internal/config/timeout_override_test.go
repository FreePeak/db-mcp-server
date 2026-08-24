package config

import (
	"testing"

	"github.com/FreePeak/db-mcp-server/pkg/db"
)

func TestApplyQueryTimeoutOverride(t *testing.T) {
	warn := func(string, ...interface{}) {}

	t.Run("fills unset connections and keeps explicit values", func(t *testing.T) {
		conns := []db.DatabaseConnectionConfig{
			{ID: "a"},
			{ID: "b", QueryTimeout: 60},
		}
		applyQueryTimeoutOverride(conns, "15", warn)
		if conns[0].QueryTimeout != 15 {
			t.Errorf("expected override applied to a, got %d", conns[0].QueryTimeout)
		}
		if conns[1].QueryTimeout != 60 {
			t.Errorf("expected explicit value kept for b, got %d", conns[1].QueryTimeout)
		}
	})

	t.Run("negative one disables explicitly", func(t *testing.T) {
		conns := []db.DatabaseConnectionConfig{{ID: "a"}}
		applyQueryTimeoutOverride(conns, "-1", warn)
		if conns[0].QueryTimeout != -1 {
			t.Errorf("expected -1 propagated, got %d", conns[0].QueryTimeout)
		}
	})

	t.Run("invalid input ignored with warning", func(t *testing.T) {
		conns := []db.DatabaseConnectionConfig{{ID: "a"}}
		called := false
		applyQueryTimeoutOverride(conns, "abc", func(string, ...interface{}) { called = true })
		if !called {
			t.Error("expected warning for invalid input")
		}
		if conns[0].QueryTimeout != 0 {
			t.Errorf("expected no change, got %d", conns[0].QueryTimeout)
		}
	})

	t.Run("below negative one rejected", func(t *testing.T) {
		conns := []db.DatabaseConnectionConfig{{ID: "a"}}
		applyQueryTimeoutOverride(conns, "-5", warn)
		if conns[0].QueryTimeout != 0 {
			t.Errorf("expected no change, got %d", conns[0].QueryTimeout)
		}
	})
}
