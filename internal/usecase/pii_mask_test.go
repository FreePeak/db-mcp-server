package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

// Content-based PII masking (ported from the hackathon branch, cycle 19):
// name-based masking_rules only catch columns whose *name* an operator
// thought to configure. This layer redacts PII by *shape* wherever it
// appears, including in free-text columns nobody would think to rule.
func TestMaskPIIInText_ContentPatterns(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"email", "contact jane.doe@acme.com today", "contact [EMAIL] today"},
		{"email in sentence", "reply to a+b@sub.example.co.uk please", "reply to [EMAIL] please"},
		{"ssn", "ssn 123-45-6789 on file", "ssn [SSN] on file"},
		{"card with spaces", "card 4111 1111 1111 1111 charged", "card [CREDIT_CARD] charged"},
		{"card plain", "4111111111111111", "[CREDIT_CARD]"},
		{"card with dashes", "4111-1111-1111-1111", "[CREDIT_CARD]"},
		{"us phone", "call (555) 123-4567 now", "call [PHONE] now"},
		{"intl phone", "tel +1-555-123-4567", "tel [PHONE]"},
		{"ipv4", "client 192.168.1.42 connected", "client [IP_ADDRESS] connected"},
		{"long identifier", "ref 123456789012345678901", "ref [LONG_NUMBER]"},

		// Benign values must survive: a masking layer that mangles ordinary
		// data gets switched off, and then it protects nothing.
		{"plain text", "hello world", "hello world"},
		{"small number", "invoice 42 paid", "invoice 42 paid"},
		{"date", "created 2026-08-25", "created 2026-08-25"},
		{"uuid-ish", "id 550e8400-e29b-41d4-a716-446655440000", "id 550e8400-e29b-41d4-a716-446655440000"},
		{"non-luhn digit run", "order 1234 5678 9012 3456", "order 1234 5678 9012 3456"},
		{"version string", "v1.13.0 released", "v1.13.0 released"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskPIIInText(tt.value, ""); got != tt.want {
				t.Fatalf("maskPIIInText(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestMaskPIIInText_PIIColumnNames(t *testing.T) {
	for _, col := range []string{
		"email", "email_addr", "user_email", "phone", "mobile_no", "tel",
		"ssn", "social_security", "credit_card", "card_number", "pan",
		"iban", "swift", "fax", "contact_number",
	} {
		val := "whatever-the-value-is"
		got := maskPIIInText(val, col)
		if got == val {
			t.Fatalf("column %q must be redacted by name, got %q", col, got)
		}
		if !strings.HasPrefix(got, "[") {
			t.Fatalf("column %q should get a bracketed marker, got %q", col, got)
		}
	}
}

func TestMaskPIIInText_BenignColumnNames(t *testing.T) {
	for _, col := range []string{"id", "name", "total", "created_at", "status", "region"} {
		val := "ordinary value"
		if got := maskPIIInText(val, col); got != val {
			t.Fatalf("benign column %q must be untouched, got %q", col, got)
		}
	}
}

func TestLuhnValid(t *testing.T) {
	valid := []string{"4111111111111111", "4111 1111 1111 1111", "5500005555555559"}
	for _, s := range valid {
		if !luhnValid(s) {
			t.Errorf("luhnValid(%q) = false, want true", s)
		}
	}
	invalid := []string{"1234567890123456", "4111111111111112", "", "1"}
	for _, s := range invalid {
		if luhnValid(s) {
			t.Errorf("luhnValid(%q) = true, want false", s)
		}
	}
}

// TestExecuteQueryMasked_ContentMaskingE2E drives the whole path against a
// real SQLite database: a free-text column nobody wrote a rule for must
// still come back with its PII redacted.
func TestExecuteQueryMasked_ContentMaskingE2E(t *testing.T) {
	raw := openSQLiteForTest(t)
	for _, s := range []string{
		`CREATE TABLE tickets (id INTEGER PRIMARY KEY, subject TEXT, body TEXT)`,
		`INSERT INTO tickets VALUES (1, 'login issue', 'write to alice@example.com or call (555) 123-4567')`,
	} {
		if _, err := raw.Exec(s); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.ExecuteQueryMasked(context.Background(), "db", "SELECT * FROM tickets", nil, true)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	for _, leak := range []string{"alice@example.com", "555) 123-4567"} {
		if strings.Contains(out, leak) {
			t.Errorf("content masking leaked %q:\n%s", leak, out)
		}
	}
	if !strings.Contains(out, "[EMAIL]") || !strings.Contains(out, "[PHONE]") {
		t.Errorf("expected [EMAIL] and [PHONE] markers, got:\n%s", out)
	}
	if !strings.Contains(out, "login issue") {
		t.Errorf("benign text must survive masking:\n%s", out)
	}
}

// TestExecuteQuery_ContentMaskingOffByDefault: content masking is opt-in per
// query (and enforced per database in the next PR), so the default path must
// stay byte-identical for callers that never asked for it.
func TestExecuteQuery_ContentMaskingOffByDefault(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (v TEXT)`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO t VALUES ('alice@example.com')`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.ExecuteQuery(context.Background(), "db", "SELECT v FROM t", nil)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if !strings.Contains(out, "alice@example.com") {
		t.Errorf("masking must be opt-in on the plain path, got:\n%s", out)
	}
}

// maskPIIEnforcedDB wraps a database and reports server-side enforcement, the
// way a connection configured with "mask_pii": true does.
type maskPIIEnforcedDB struct {
	domain.Database
}

func (m *maskPIIEnforcedDB) MaskPII() bool { return true }

// TestExecuteQuery_ServerConfigForcesMasking: governance belongs to the
// operator, not the agent. A connection with mask_pii enabled must redact
// even when the caller never asked — including on the plain ExecuteQuery
// path, so omitting the parameter cannot bypass it.
func TestExecuteQuery_ServerConfigForcesMasking(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE tickets (body TEXT)`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO tickets VALUES ('mail alice@example.com')`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	enforced := &maskPIIEnforcedDB{Database: &sqliteDB{db: raw}}
	uc := NewDatabaseUseCase(&fakeRepo{db: enforced, dbType: "sqlite"})

	for _, path := range []string{"ExecuteQuery", "ExecuteQueryMasked"} {
		var out string
		var err error
		if path == "ExecuteQuery" {
			out, err = uc.ExecuteQuery(context.Background(), "db", "SELECT body FROM tickets", nil)
		} else {
			// Explicitly asking for no masking must still not win.
			out, err = uc.ExecuteQueryMasked(context.Background(), "db", "SELECT body FROM tickets", nil, false)
		}
		if err != nil {
			t.Fatalf("%s failed: %v", path, err)
		}
		if strings.Contains(out, "alice@example.com") {
			t.Errorf("%s: server-side mask_pii was bypassed:\n%s", path, out)
		}
		if !strings.Contains(out, "[EMAIL]") {
			t.Errorf("%s: expected [EMAIL] marker:\n%s", path, out)
		}
	}
}
