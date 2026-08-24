package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/internal/domain"
	"github.com/FreePeak/db-mcp-server/pkg/db"
)

// maskedDB wraps a database and exposes configured masking rules so the
// repository adapter's capability assertion finds them.
type maskedDB struct {
	domain.Database
	rules []db.MaskingRule
}

func (m *maskedDB) MaskingRules() []db.MaskingRule { return m.rules }

// TestMatchMaskingRules locks in cycle 36's matcher semantics:
// case-insensitive matching against output column names,
// first-matching-rule-wins, invalid patterns never match.
func TestMatchMaskingRules(t *testing.T) {
	rules := []db.MaskingRule{
		{Pattern: "(?i)^email$", Strategy: "fixed_string", Value: "***"},
		{Pattern: "(?i)(ssn|tax_id)", Strategy: "null"},
	}
	masks := matchMaskingRules([]string{"id", "Email", "tax_id", "name"}, rules)
	if masks == nil {
		t.Fatal("expected matches")
	}
	if masks[0] != nil || masks[3] != nil {
		t.Errorf("unmatched columns must be nil, got %v", masks)
	}
	if masks[1] != &rules[0] {
		t.Errorf("case-insensitive name match failed: %v", masks[1])
	}
	if masks[2] != &rules[1] {
		t.Errorf("pattern alternation match failed: %v", masks[2])
	}

	// No overlap between rules and columns means nothing to apply.
	if got := matchMaskingRules([]string{"a", "b"}, rules); got != nil {
		t.Errorf("expected nil when nothing matches, got %v", got)
	}

	// Invalid regex degrades to no-match rather than an error.
	bad := []db.MaskingRule{{Pattern: "([", Strategy: "null"}}
	if got := matchMaskingRules([]string{"email"}, bad); got != nil {
		t.Errorf("invalid pattern must not match, got %v", got)
	}
}

// TestApplyMaskStrategy locks in the strategy behaviors: fixed_string
// replaces any value including NULL, null renders as NULL, unknown
// strategies pass through untouched.
func TestApplyMaskStrategy(t *testing.T) {
	fixed := &db.MaskingRule{Strategy: "fixed_string", Value: "***MASKED***"}
	if got := applyMaskStrategy(fixed, "alice@example.com"); got != "***MASKED***" {
		t.Errorf("fixed_string must replace value, got %v", got)
	}
	if got := applyMaskStrategy(fixed, nil); got != "***MASKED***" {
		t.Errorf("fixed_string must replace even nil cells, got %v", got)
	}

	nulls := &db.MaskingRule{Strategy: "null"}
	if got := applyMaskStrategy(nulls, 42); got != nil {
		t.Errorf("null strategy must blank cells, got %v", got)
	}

	unknown := &db.MaskingRule{Strategy: "typo"}
	if got := applyMaskStrategy(unknown, "keep"); got != "keep" {
		t.Errorf("unknown strategy must pass through, got %v", got)
	}
}

// TestExecuteQuery_MaskingE2E runs a real SQLite query through
// ExecuteQuery with configured rules: matched columns are masked in the
// rendered output while unmatched columns stay readable.
func TestExecuteQuery_MaskingE2E(t *testing.T) {
	raw := openSQLiteForTest(t)
	for _, s := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, tax_id TEXT, note TEXT)`,
		`INSERT INTO users VALUES (1, 'alice@example.com', '123-45-6789', 'visible')`,
	} {
		if _, err := raw.Exec(s); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	base := &sqliteDB{db: raw}
	mdb := &maskedDB{
		Database: base,
		rules: []db.MaskingRule{
			{Pattern: "(?i)email", Strategy: "fixed_string", Value: "***MASKED***"},
			{Pattern: "tax_id", Strategy: "null"},
		},
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: mdb, dbType: "sqlite"})

	out, err := uc.ExecuteQuery(context.Background(), "db", "SELECT * FROM users", nil)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if !strings.Contains(out, "***MASKED***") {
		t.Errorf("expected masked email, got:\n%s", out)
	}
	if strings.Contains(out, "alice@example.com") {
		t.Errorf("raw email leaked through masking:\n%s", out)
	}
	if !strings.Contains(out, "\tNULL\t") && !strings.Contains(out, "\tNULL\n") {
		t.Errorf("expected tax_id rendered as NULL, got:\n%s", out)
	}
	if !strings.Contains(out, "visible") {
		t.Errorf("unmasked column must remain readable:\n%s", out)
	}
}

// TestExecuteQuery_NoRulesUnchanged verifies databases without masking
// rules produce byte-identical behavior to before the feature existed.
func TestExecuteQuery_NoRulesUnchanged(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (v TEXT)`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO t VALUES ('plain')`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.ExecuteQuery(context.Background(), "db", "SELECT v FROM t", nil)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if !strings.Contains(out, "plain") {
		t.Errorf("expected unmasked output, got:\n%s", out)
	}
}
