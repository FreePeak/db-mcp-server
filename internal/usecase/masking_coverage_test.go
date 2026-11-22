package usecase

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/internal/domain"
	"github.com/FreePeak/db-mcp-server/pkg/db"
)

// coverageDB exposes operator masking config the way the repository adapter
// does (MaskingRules and MaskPII) over a real SQLite database.
type coverageDB struct {
	domain.Database
	rules   []db.MaskingRule
	maskPII bool
}

func (c *coverageDB) MaskingRules() []db.MaskingRule { return c.rules }
func (c *coverageDB) MaskPII() bool                  { return c.maskPII }

// coverageRawValues are the sensitive values seeded by newCoverageUseCase.
var coverageRawValues = []string{"bob@example.com", "5551234567", "123-45-6789", "carol@example.org"}

// newCoverageUseCase seeds one customers row. The sensitive values sit in a
// name-matched column (email), a PII-named column (phone), an SSN-shaped
// column (tax_id) and a free-text column (note), behind the operator masking
// config under test.
func newCoverageUseCase(t *testing.T, rules []db.MaskingRule, maskPII bool) *DatabaseUseCase {
	t.Helper()
	raw := openSQLiteForTest(t)
	for _, s := range []string{
		`CREATE TABLE customers (id INTEGER PRIMARY KEY, email TEXT, phone TEXT, tax_id TEXT, note TEXT)`,
		`INSERT INTO customers VALUES (1, 'bob@example.com', '5551234567', '123-45-6789', 'write to carol@example.org')`,
	} {
		if _, err := raw.Exec(s); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	mdb := &coverageDB{Database: &sqliteDB{db: raw}, rules: rules, maskPII: maskPII}
	return NewDatabaseUseCase(&fakeRepo{db: mdb, dbType: "sqlite"})
}

func assertNoRawValues(t *testing.T, out string, raw ...string) {
	t.Helper()
	for _, v := range raw {
		if strings.Contains(out, v) {
			t.Errorf("raw value %q leaked through masking:\n%s", v, out)
		}
	}
}

// TestMasking_AppliesOnInternalExecuteQuery pins masking on the internal
// ExecuteQuery path used by explain, describe, performance and timescale
// tools: the request side is mask=false and the row cap is off, so only the
// operator config can mask the output.
func TestMasking_AppliesOnInternalExecuteQuery(t *testing.T) {
	const q = "SELECT id, email, phone, tax_id, note FROM customers"

	t.Run("masking_rules", func(t *testing.T) {
		rules := []db.MaskingRule{
			{Pattern: "(?i)^email$", Strategy: "fixed_string", Value: "***MASKED***"},
			{Pattern: "(?i)^tax_id$", Strategy: "null"},
		}
		uc := newCoverageUseCase(t, rules, false)
		out, err := uc.ExecuteQuery(context.Background(), "db", q, nil)
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if !strings.Contains(out, "***MASKED***") {
			t.Errorf("expected fixed_string mask on email, got:\n%s", out)
		}
		if !strings.Contains(out, "\tNULL\t") {
			t.Errorf("expected null strategy on tax_id, got:\n%s", out)
		}
		assertNoRawValues(t, out, "bob@example.com", "123-45-6789")
		if !strings.Contains(out, "Masked cells: 2") {
			t.Errorf("expected masked-cell footer for email and tax_id, got:\n%s", out)
		}
	})

	t.Run("mask_pii", func(t *testing.T) {
		uc := newCoverageUseCase(t, nil, true)
		out, err := uc.ExecuteQuery(context.Background(), "db", q, nil)
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		for _, want := range []string{"[EMAIL]", "[PHONE]", "[SSN]"} {
			if !strings.Contains(out, want) {
				t.Errorf("expected content marker %s, got:\n%s", want, out)
			}
		}
		assertNoRawValues(t, out, coverageRawValues...)
	})
}

// TestMasking_RequestCannotDisable pins that the request side cannot turn
// operator masking off: ExecuteQueryMasked with mask=false is what the query
// tool sends when mask_pii is absent or false.
func TestMasking_RequestCannotDisable(t *testing.T) {
	const q = "SELECT id, email, phone, tax_id, note FROM customers"
	ctx := context.Background()

	t.Run("operator mask_pii survives mask=false", func(t *testing.T) {
		uc := newCoverageUseCase(t, nil, true)
		out, err := uc.ExecuteQueryMasked(ctx, "db", q, nil, false)
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if !strings.Contains(out, "[PHONE]") {
			t.Errorf("expected content masking despite mask=false, got:\n%s", out)
		}
		assertNoRawValues(t, out, coverageRawValues...)
	})

	t.Run("operator masking_rules survive mask=false", func(t *testing.T) {
		rules := []db.MaskingRule{{Pattern: "(?i)^email$", Strategy: "fixed_string", Value: "***MASKED***"}}
		uc := newCoverageUseCase(t, rules, false)
		out, err := uc.ExecuteQueryMasked(ctx, "db", q, nil, false)
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if !strings.Contains(out, "***MASKED***") {
			t.Errorf("expected rule mask despite mask=false, got:\n%s", out)
		}
		assertNoRawValues(t, out, "bob@example.com")
	})
}

// TestExecuteStatement_ReturnsNoRowData pins that ExecuteStatement never
// returns row values, even for a SELECT sent through it or a mutation that
// touches the row. Masking is off so any leaked value would show raw.
func TestExecuteStatement_ReturnsNoRowData(t *testing.T) {
	ctx := context.Background()
	uc := newCoverageUseCase(t, nil, false)
	for _, stmt := range []string{
		"SELECT email, phone, tax_id, note FROM customers",
		"UPDATE customers SET note = 'reviewed' WHERE id = 1",
	} {
		out, err := uc.ExecuteStatement(ctx, "db", stmt, nil)
		if err != nil {
			t.Fatalf("%s failed: %v", stmt, err)
		}
		if !strings.Contains(out, "Statement executed successfully") {
			t.Errorf("%s: expected execution summary, got:\n%s", stmt, out)
		}
		assertNoRawValues(t, out, coverageRawValues...)
	}
}

// TestExecuteTransaction_ReturnsNoRowData pins that statements run inside a
// transaction return only the execution summary and transaction metadata,
// never row values.
func TestExecuteTransaction_ReturnsNoRowData(t *testing.T) {
	ctx := context.Background()
	uc := newCoverageUseCase(t, nil, false)

	_, meta, err := uc.ExecuteTransaction(ctx, "db", "begin", "", "", nil, false)
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	txID, _ := meta["transactionId"].(string)
	defer func() {
		if _, _, rerr := uc.ExecuteTransaction(ctx, "db", "rollback", txID, "", nil, false); rerr != nil {
			t.Errorf("rollback failed: %v", rerr)
		}
	}()

	for _, stmt := range []string{
		"SELECT email, phone, tax_id, note FROM customers",
		"UPDATE customers SET note = 'reviewed' WHERE id = 1",
	} {
		msg, execMeta, err := uc.ExecuteTransaction(ctx, "db", "execute", txID, stmt, nil, false)
		if err != nil {
			t.Fatalf("%s failed inside transaction: %v", stmt, err)
		}
		if !strings.Contains(msg, "Statement executed in transaction") {
			t.Errorf("%s: expected execution summary, got:\n%s", stmt, msg)
		}
		assertNoRawValues(t, msg+fmt.Sprint(execMeta), coverageRawValues...)
	}
}
