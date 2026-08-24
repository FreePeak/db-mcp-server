package config

import (
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/pkg/db"
)

// TestValidateMaskingRules locks in cycle 37's fail-closed startup check:
// invalid patterns and unknown strategies abort config load instead of
// silently disabling a mask.
func TestValidateMaskingRules(t *testing.T) {
	valid := []db.DatabaseConnectionConfig{
		{ID: "db1", MaskingRules: []db.MaskingRule{
			{Pattern: "(?i)email", Strategy: "fixed_string", Value: "***"},
			{Pattern: "phone", Strategy: "partial", KeepLast: 4},
			{Pattern: "x", Strategy: "null"},
		}},
	}
	if err := validateMaskingRules(valid); err != nil {
		t.Errorf("valid rules must pass, got %v", err)
	}

	badPattern := []db.DatabaseConnectionConfig{
		{ID: "db1", MaskingRules: []db.MaskingRule{{Pattern: "([", Strategy: "null"}}},
	}
	err := validateMaskingRules(badPattern)
	if err == nil || !strings.Contains(err.Error(), `invalid pattern`) || !strings.Contains(err.Error(), "db1") {
		t.Errorf("invalid pattern must fail fast naming db and rule, got %v", err)
	}

	badStrategy := []db.DatabaseConnectionConfig{
		{ID: "pg1", MaskingRules: []db.MaskingRule{{Pattern: ".", Strategy: "hashish"}}},
	}
	err = validateMaskingRules(badStrategy)
	if err == nil || !strings.Contains(err.Error(), "unknown strategy") || !strings.Contains(err.Error(), "pg1") {
		t.Errorf("unknown strategy must fail fast, got %v", err)
	}
}
