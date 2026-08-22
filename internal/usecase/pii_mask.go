package usecase

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

// PII masking at the tool boundary prevents agents from pulling raw personal
// data into LLM context. Two layers:
//  1. Column-name heuristics — columns named like PII carriers are masked in
//     full regardless of content shape.
//  2. Content patterns — emails, SSNs, cards, phones, IPs and long numeric
//     identifiers are redacted wherever they appear inside any text value.
//
// Masking is opt-in per query (`mask_pii` parameter) so legitimate admin
// workflows keep full fidelity by default.

var (
	emailRe    = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	ssnRe      = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	cardRe     = regexp.MustCompile(`\b(?:\d[ -]?){12,18}\d\b`)
	phoneRe    = regexp.MustCompile(`(?:\+?\d[\d\s\-().]{7,}\d|\(\d{3}\)\s*\d{3}[-.\s]?\d{4})`)
	ipv4Re     = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	longNumRe  = regexp.MustCompile(`\b\d{19,}\b`)
	columnPiiK = map[string]string{
		"email": "[EMAIL]", "e_mail": "[EMAIL]", "mail": "[EMAIL]",
		"phone": "[PHONE]", "mobile": "[PHONE]", "tel": "[PHONE]",
		"fax": "[PHONE]", "contact_number": "[PHONE]",
		"ssn": "[SSN]", "social_security": "[SSN]",
		"credit_card": "[CREDIT_CARD]", "card_number": "[CREDIT_CARD]",
		"card_no": "[CREDIT_CARD]", "pan": "[CREDIT_CARD]",
		"iban": "[IBAN]", "swift": "[IBAN]",
	}
)

// maskPIIInText masks one rendered cell. The column name drives the
// whole-value heuristic; content patterns run on every value.
func maskPIIInText(value, column string) string {
	col := strings.ToLower(strings.Trim(column, "\"`[] "))
	for frag, marker := range columnPiiK {
		if strings.Contains(col, frag) {
			return marker
		}
	}
	out := value
	out = emailRe.ReplaceAllString(out, "[EMAIL]")
	out = ssnRe.ReplaceAllString(out, "[SSN]")
	// Order matters: cards (long digit runs) before phones (shorter, looser),
	// and IPs before phones (dots are part of the phone charset).
	out = cardRe.ReplaceAllStringFunc(out, func(m string) string {
		digits := len(digitCountRe.FindAllString(m, -1))
		if digits >= 13 && digits <= 19 && !strings.Contains(m, ":") && luhnValid(m) {
			return "[CREDIT_CARD]"
		}
		return m
	})
	out = ipv4Re.ReplaceAllString(out, "[IP_ADDRESS]")
	out = longNumRe.ReplaceAllString(out, "[LONG_NUMBER]")
	out = phoneRe.ReplaceAllString(out, "[PHONE]")
	return out
}

var digitCountRe = regexp.MustCompile(`\d`)

// luhnValid checks a candidate card number's checksum, filtering digit runs
// that merely look like cards (order numbers, timestamps). Non-digit
// separators are ignored; fewer than two digits fails.
func luhnValid(s string) bool {
	digits := make([]int, 0, 19)
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits = append(digits, int(r-'0'))
		}
	}
	if len(digits) < 2 {
		return false
	}
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := digits[i]
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

// formatQueryResultsMasked renders query results with PII masking applied to
// every data cell (headers stay visible).
func formatQueryResultsMasked(rows domain.Rows, maxRows int) (string, error) {
	out, _, err := renderQueryResults(rows, maxRows, true)
	return out, err
}

// renderQueryResults is the single result-rendering path; masking toggles
// the cell transformer without duplicating scan/format logic.
// renderQueryResults returns the rendered text plus the number of cells
// that were redacted (0 when masking is off or nothing matched).
func renderQueryResults(rows domain.Rows, maxRows int, mask bool) (string, int, error) {
	columns, err := rows.Columns()
	if err != nil {
		return "", 0, fmt.Errorf("failed to get column names: %w", err)
	}

	cellsMasked := 0

	var resultText strings.Builder
	resultText.WriteString("Results:\n\n")
	resultText.WriteString(strings.Join(columns, "\t") + "\n")
	resultText.WriteString(strings.Repeat("-", 80) + "\n")

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	rowCount := 0
	truncated := false
	for rows.Next() {
		if maxRows > 0 && rowCount >= maxRows {
			truncated = true
			break
		}
		rowCount++
		if scanErr := rows.Scan(valuePtrs...); scanErr != nil {
			return "", 0, fmt.Errorf("failed to scan row: %w", scanErr)
		}

		var rowText []string
		for i := range columns {
			val := values[i]
			if val == nil {
				rowText = append(rowText, "NULL")
				continue
			}
			var s string
			switch v := val.(type) {
			case []byte:
				s = string(v)
			default:
				s = fmt.Sprintf("%v", v)
			}
			if mask {
				if masked := maskPIIInText(s, columns[i]); masked != s {
					s = masked
					cellsMasked++
				}
			}
			rowText = append(rowText, s)
		}
		resultText.WriteString(strings.Join(rowText, "\t") + "\n")
	}
	if err = rows.Err(); err != nil {
		return "", 0, fmt.Errorf("error reading rows: %w", err)
	}

	if truncated {
		resultText.WriteString(fmt.Sprintf("\nTruncated: showing first %d rows (max_rows=%d). Refine the query with LIMIT or tighter filters to see more.", rowCount, maxRows))
		resultText.WriteString(fmt.Sprintf("\nTotal rows shown: %d", rowCount))
	} else {
		resultText.WriteString(fmt.Sprintf("\nTotal rows: %d", rowCount))
	}
	return resultText.String(), cellsMasked, nil
}
