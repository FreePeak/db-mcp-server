package usecase

import (
	"regexp"
	"strings"
)

// Content-based PII masking at the result boundary.
//
// masking_rules (name-based) only hide columns an operator thought to rule.
// This layer redacts by *shape*, so PII that lands in a free-text column —
// a support note, a JSON blob, an address line — is caught too. Two layers:
//
//  1. Column-name heuristics: a column named like a PII carrier is redacted
//     whole, whatever its content looks like.
//  2. Content patterns: emails, SSNs, cards (Luhn-checked), IPv4 and long
//     numeric identifiers are replaced wherever they appear in a value.
//
// Both are best-effort and deliberately conservative: a masking layer that
// mangles ordinary data gets switched off, and then it protects nothing.
var (
	emailRe = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	ssnRe   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	cardRe  = regexp.MustCompile(`\b(?:\d[ -]?){12,18}\d\b`)
	// phoneRe is a *candidate* matcher only; maskPIIInText applies shape
	// checks (digit count, not embedded in a longer token) before redacting,
	// because a loose pattern swallows dates, UUID fragments and order
	// numbers — and a masker that mangles ordinary data gets switched off.
	// The parenthesised US form is listed first so its leading "(" is part of
	// the match; the loose branch would otherwise start at the digit after it
	// and leave the "(" behind in the output.
	phoneRe   = regexp.MustCompile(`\(\d{3}\)\s*\d{3}[-.\s]?\d{4}|\+?\d[\d\s\-().]{7,}\d`)
	ipv4Re    = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	longNumRe = regexp.MustCompile(`\b\d{19,}\b`)
	digitRe   = regexp.MustCompile(`\d`)
	// dateRe guards the loose phone pattern against YYYY-MM-DD values.
	dateRe = regexp.MustCompile(`^(19|20)\d{2}[-/.]\d{1,2}[-/.]\d{1,2}$`)

	// piiColumnMarkers maps a substring of a column name to the marker used
	// when the whole column is redacted by name.
	piiColumnMarkers = map[string]string{
		"email": "[EMAIL]", "e_mail": "[EMAIL]", "mail": "[EMAIL]",
		"phone": "[PHONE]", "mobile": "[PHONE]", "tel": "[PHONE]",
		"fax": "[PHONE]", "contact_number": "[PHONE]",
		"ssn": "[SSN]", "social_security": "[SSN]",
		"credit_card": "[CREDIT_CARD]", "card_number": "[CREDIT_CARD]",
		"card_no": "[CREDIT_CARD]", "pan": "[CREDIT_CARD]",
		"iban": "[IBAN]", "swift": "[IBAN]",
	}
)

// maskPIIInText masks one rendered cell value. The column name drives the
// whole-value heuristic; the content patterns then run on every value.
func maskPIIInText(value, column string) string {
	col := strings.ToLower(strings.Trim(column, "\"`[] "))
	for frag, marker := range piiColumnMarkers {
		if strings.Contains(col, frag) {
			return marker
		}
	}

	out := value
	out = emailRe.ReplaceAllString(out, "[EMAIL]")
	out = ssnRe.ReplaceAllString(out, "[SSN]")
	// Order matters: cards (long digit runs, Luhn-checked) before the looser
	// phone pattern, and IPv4 before phones (dots are in the phone charset).
	out = cardRe.ReplaceAllStringFunc(out, func(m string) string {
		if n := len(digitRe.FindAllString(m, -1)); n >= 13 && n <= 19 && luhnValid(m) {
			return "[CREDIT_CARD]"
		}
		return m
	})
	out = ipv4Re.ReplaceAllString(out, "[IP_ADDRESS]")
	out = longNumRe.ReplaceAllString(out, "[LONG_NUMBER]")
	// Phones are the loosest pattern, so they get a shape check: 7–15 digits
	// (the E.164 ceiling), no letter or further digit adjacent, and not a
	// date. Without this a date, a UUID fragment or an order number renders
	// as [PHONE], which is the kind of false positive that gets a masking
	// layer switched off.
	out = phoneRe.ReplaceAllStringFunc(out, func(m string) string {
		if !phoneShapedInContext(m, out) {
			return m
		}
		return "[PHONE]"
	})
	return out
}

// phoneShaped reports whether a candidate looks like a phone number rather
// than a date, an id fragment or an order number.
func phoneShapedInContext(s, full string) bool {
	if !phoneShaped(s) {
		return false
	}
	idx := strings.Index(full, s)
	if idx < 0 {
		return true
	}
	isHex := func(b byte) bool {
		return (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F') || (b >= '0' && b <= '9')
	}
	if idx > 0 && isHex(full[idx-1]) {
		return false
	}
	if end := idx + len(s); end < len(full) && isHex(full[end]) {
		return false
	}
	return true
}

func phoneShaped(s string) bool {
	digits := digitRe.FindAllString(s, -1)
	if len(digits) < 7 || len(digits) > 15 {
		return false
	}
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "20") || strings.HasPrefix(trimmed, "19") {
		// YYYY-... dates: 4-digit year followed by separators and two more
		// two-digit groups (2026-08-25).
		if dateRe.MatchString(trimmed) {
			return false
		}
	}
	// A phone must not be part of a longer hex/number token.
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}

// luhnValid reports whether a candidate card number passes the Luhn
// checksum, which filters out digit runs that merely look like cards
// (order numbers, timestamps, sequential ids). Separators are ignored.
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
