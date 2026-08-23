package usecase

import (
	"context"
	"strings"
	"testing"
)

func TestMaskPIIValue_Patterns(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"email", "contact via jane.doe@acme.com today", "contact via [EMAIL] today"},
		{"ssn", "SSN: 123-45-6789", "SSN: [SSN]"},
		{"credit_card_spaced", "card 4111 1111 1111 1111 on file", "card [CREDIT_CARD] on file"},
		{"credit_card_plain", "4111111111111111", "[CREDIT_CARD]"},
		{"us_phone", "call (555) 123-4567 now", "call [PHONE] now"},
		{"intl_phone", "+1-555-123-4567", "[PHONE]"},
		{"ipv4", "client 192.168.1.42 connected", "client [IP_ADDRESS] connected"},
		{"iban_like_long_number", "ref 123456789012345678901", "ref [LONG_NUMBER]"},
		{"plain_text_untouched", "hello world order #123", "hello world order #123"},
		{"short_number_untouched", "invoice 42 paid", "invoice 42 paid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskPIIInText(tt.value, "")
			if got != tt.want {
				t.Fatalf("maskPIIInText(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestMaskPII_SensitiveColumnNames(t *testing.T) {
	sensitive := map[string]string{
		"email":       "jane@acme.com",
		"email_addr":  "x@y.org",
		"phone":       "5551234567",
		"mobile_no":   "555-867-5309",
		"ssn":         "123456789",
		"credit_card": "4111111111111111",
		"card_number": "4111-1111-1111-1111",
		"iban":        "DE89370400440532013000",
	}
	for col, val := range sensitive {
		got := maskPIIInText(val, col)
		if got == val || !strings.Contains(got, "[") {
			t.Fatalf("column %q value %q should be fully masked, got %q", col, val, got)
		}
	}
}

func TestMaskPII_BenignColumnsUntouched(t *testing.T) {
	for _, col := range []string{"name", "order_id", "total", "created_at", "status"} {
		val := "ordinary-value-123"
		if got := maskPIIInText(val, col); got != val {
			t.Fatalf("column %q should be untouched: got %q", col, got)
		}
	}
}

// TestFormatQueryResultsWithMasking proves the end-to-end render path masks
// PII cells while leaving headers and benign data intact.
func TestFormatQueryResultsWithMasking(t *testing.T) {
	rows := &staticRows{
		columns: []string{"id", "email", "notes"},
		data: [][]interface{}{
			{int64(1), "jane@acme.com", "met at 10.0.0.7"},
			{int64(2), nil, "nothing here"},
		},
	}
	out, err := formatQueryResultsMasked(rows, 0)
	if err != nil {
		t.Fatalf("format failed: %v", err)
	}
	if strings.Contains(out, "jane@acme.com") {
		t.Fatalf("raw email leaked:\n%s", out)
	}
	if !strings.Contains(out, "[EMAIL]") {
		t.Fatalf("expected [EMAIL] marker:\n%s", out)
	}
	if !strings.Contains(out, "[IP_ADDRESS]") {
		t.Fatalf("expected [IP_ADDRESS] marker:\n%s", out)
	}
	if !strings.Contains(out, "NULL") {
		t.Fatalf("NULL handling broken:\n%s", out)
	}
	if !strings.Contains(out, "email") { // header intact
		t.Fatalf("headers must remain visible:\n%s", out)
	}
}

func TestFormatQueryResults_UnmaskedByDefault(t *testing.T) {
	rows := &staticRows{
		columns: []string{"email"},
		data:    [][]interface{}{{"jane@acme.com"}},
	}
	out, err := formatQueryResults(rows, 0)
	if err != nil {
		t.Fatalf("format failed: %v", err)
	}
	if !strings.Contains(out, "jane@acme.com") {
		t.Fatalf("masking must be opt-in; raw value expected:\n%s", out)
	}
}

// staticRows is a deterministic domain.Rows stub for renderer tests.
type staticRows struct {
	columns []string
	data    [][]interface{}
	pos     int
}

func (s *staticRows) Columns() ([]string, error) { return s.columns, nil }
func (s *staticRows) Close() error               { return nil }
func (s *staticRows) Err() error                 { return nil }
func (s *staticRows) Next() bool {
	if s.pos >= len(s.data) {
		return false
	}
	s.pos++
	return true
}
func (s *staticRows) Scan(dest ...interface{}) error {
	row := s.data[s.pos-1]
	for i := range dest {
		d, ok := dest[i].(*interface{})
		if !ok {
			continue
		}
		if row[i] == nil {
			*d = nil
		} else {
			*d = row[i]
		}
	}
	return nil
}

// TestExecuteQueryMasked_EndToEnd proves PII masking flows through the full
// query path on a real SQLite database.
func TestExecuteQueryMasked_EndToEnd(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE customers (id INTEGER PRIMARY KEY, email TEXT, note TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO customers (email, note) VALUES ('jane@acme.com', 'ip 10.1.2.3')`); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	masked, err := uc.ExecuteQueryMasked(context.Background(), "db1", "SELECT email, note FROM customers", nil, true, VerbosityFull)
	if err != nil {
		t.Fatalf("masked query failed: %v", err)
	}
	if strings.Contains(masked, "jane@acme.com") || strings.Contains(masked, "10.1.2.3") {
		t.Fatalf("PII leaked through masked path:\n%s", masked)
	}
	if !strings.Contains(masked, "[EMAIL]") || !strings.Contains(masked, "[IP_ADDRESS]") {
		t.Fatalf("expected markers:\n%s", masked)
	}

	rawOut, err := uc.ExecuteQueryMasked(context.Background(), "db1", "SELECT email FROM customers", nil, false, VerbosityFull)
	if err != nil {
		t.Fatalf("unmasked query failed: %v", err)
	}
	if !strings.Contains(rawOut, "jane@acme.com") {
		t.Fatalf("mask=false must return raw data:\n%s", rawOut)
	}
}

// TestExecuteQueryMasked_ServerConfigForcesMasking proves operator-level
// MaskPII config wins over the agent's per-request opt-out.
func TestExecuteQueryMasked_ServerConfigForcesMasking(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE people (id INTEGER PRIMARY KEY, email TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO people (email) VALUES ('bob@corp.io')`); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	wrapper := &sqliteDB{db: raw, maskPII: true}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	// Agent explicitly asks for raw data; server config must still mask.
	out, err := uc.ExecuteQueryMasked(context.Background(), "db1", "SELECT email FROM people", nil, false, VerbosityFull)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if strings.Contains(out, "bob@corp.io") || !strings.Contains(out, "[EMAIL]") {
		t.Fatalf("server-enforced masking bypassed:\n%s", out)
	}

	// Legacy path must honor server config too.
	legacy, err := uc.ExecuteQuery(context.Background(), "db1", "SELECT email FROM people", nil)
	if err != nil {
		t.Fatalf("legacy query failed: %v", err)
	}
	if strings.Contains(legacy, "bob@corp.io") {
		t.Fatalf("legacy path bypasses server-enforced masking:\n%s", legacy)
	}
}
