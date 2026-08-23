package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Sequence exhaustion audit: integer key sequences silently fail once
// they hit their max — a classic production incident that surfaces as
// "inserts randomly failing" with no obvious cause. Read the engine's
// sequence catalogs and flag anything at ≥80% of its ceiling.

// sequenceCatalog returns the engine's sequence-usage SELECT, or ""
// when unsupported.
func sequenceCatalog(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT sequencename, last_value, max_value
FROM pg_sequences
WHERE last_value IS NOT NULL`
	default:
		return ""
	}
}

// sequenceExhausted reports whether a sequence has consumed ≥80% of
// its range (and is not fresh).
func sequenceExhausted(last, max float64) bool {
	if last <= 0 || max <= 0 {
		return false
	}
	return last/max >= 0.8
}

// ListSequences renders per-sequence usage against the ceiling.
func (uc *DatabaseUseCase) ListSequences(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := sequenceCatalog(dbType)
	if q == "" {
		return "", fmt.Errorf("sequence statistics are not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("sequence catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing sequence rows: %v", closeErr)
		}
	}()

	var lines []string
	total := 0
	for rows.Next() {
		var name string
		var lastV, maxV interface{}
		if scanErr := rows.Scan(&name, &lastV, &maxV); scanErr != nil {
			continue
		}
		total++
		lastN, lastOK := toFloat(lastV)
		maxN, maxOK := toFloat(maxV)
		if !lastOK || !maxOK {
			continue
		}
		if sequenceExhausted(lastN, maxN) {
			pct := 100 * lastN / maxN
			lines = append(lines, fmt.Sprintf(
				"- %s: %.0f%% of range used (%.0f of %.0f) — inserts fail when it hits the ceiling; consider nextval rebasing or a bigger column",
				name, pct, lastN, maxN))
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate sequence rows: %w", err)
	}
	if len(lines) == 0 {
		return fmt.Sprintf("No sequences near exhaustion across %d tracked sequence(s).", total), nil
	}
	out := fmt.Sprintf("%d sequence(s) near exhaustion:\n%s", len(lines), strings.Join(lines, "\n"))
	return out, nil
}

// toFloat converts a driver value that should be numeric.
func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float64:
		return n, true
	case []byte:
		var out float64
		if _, err := fmt.Sscanf(string(n), "%g", &out); err == nil {
			return out, true
		}
	}
	return 0, false
}
