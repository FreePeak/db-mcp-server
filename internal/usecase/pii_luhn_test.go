package usecase

import (
	"testing"
)

func TestLuhnValid(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"4111111111111111", true}, // classic Visa test number
		{"5500005555555559", true}, // Mastercard test number
		{"4111 1111 1111 1111", true},
		{"4111111111111112", false}, // checksum off by one
		{"1234567890123456", false}, // random 16 digits
		{"", false},                 // empty
	}
	for _, tt := range tests {
		if got := luhnValid(tt.in); got != tt.want {
			t.Errorf("luhnValid(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
	// Note: callers gate on 13-19 digits before consulting the checksum,
	// so short strings that happen to satisfy Luhn arithmetic are harmless.
}

// TestMaskPII_LuhnFiltersCardFalsePositives proves digit runs without valid
// checksums are not branded [CREDIT_CARD].
func TestMaskPII_LuhnFiltersCardFalsePositives(t *testing.T) {
	got := maskPIIInText("order ref 1234567890123456 shipped", "")
	if got == "order ref [CREDIT_CARD] shipped" {
		t.Fatalf("invalid-checksum run must not be labeled credit card: %q", got)
	}
	valid := maskPIIInText("pay with 4111111111111111 please", "")
	if valid != "pay with [CREDIT_CARD] please" {
		t.Fatalf("valid card should still be masked: %q", valid)
	}
	spaced := maskPIIInText("card on file 4111 1111 1111 1111 ok", "")
	if spaced != "card on file [CREDIT_CARD] ok" {
		t.Fatalf("spaced valid card should be masked: %q", spaced)
	}
}
