package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSequenceCatalog proves per-engine sequence-usage SELECTs exist.
func TestSequenceCatalog(t *testing.T) {
	pg := sequenceCatalog("postgres")
	if !strings.Contains(pg, "pg_sequences") || !strings.Contains(pg, "max_value") {
		t.Fatalf("pg catalog wrong:\n%s", pg)
	}
	if sequenceCatalog("sqlite") != "" {
		t.Fatal("sqlite should have no sequence catalog")
	}
}

// TestSequenceRatio proves the exhaustion threshold math.
func TestSequenceRatio(t *testing.T) {
	tests := []struct {
		last, max float64
		want      bool
	}{
		{80, 100, true},  // at the 80% line
		{79, 100, false}, // just under
		{2147483000, 2147483647, true},
		{0, 100, false},  // fresh sequence
		{100, 100, true}, // exhausted
	}
	for _, tt := range tests {
		if got := sequenceExhausted(tt.last, tt.max); got != tt.want {
			t.Fatalf("sequenceExhausted(%v, %v) = %v, want %v", tt.last, tt.max, got, tt.want)
		}
	}
}

// TestListSequences_Unsupported proves engines without catalogs get an
// explicit error rather than fabricated advice.
func TestListSequences_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListSequences(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
