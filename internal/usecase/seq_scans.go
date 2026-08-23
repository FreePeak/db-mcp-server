package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Sequential-scan workload audit: the static audits (fk_indexes,
// suggest_indexes) reason about shape; pg_stat_user_tables.seq_scan and
// MySQL's null-index row counts record what actually happened. Tables
// dominated by sequential scans are where missing indexes hurt for real.

// seqScanQuery returns the engine's per-table scan-counter SELECT
// (table, sequential scans, index scans), or "" when unsupported.
func seqScanQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT relname, seq_scan, idx_scan
FROM pg_stat_user_tables
ORDER BY seq_scan DESC`
	case "mysql", "mariadb":
		return `SELECT OBJECT_NAME,
       SUM(CASE WHEN INDEX_NAME IS NULL THEN COUNT_READ ELSE 0 END) AS seq_scan,
       SUM(CASE WHEN INDEX_NAME IS NOT NULL THEN COUNT_READ ELSE 0 END) AS idx_scan
FROM performance_schema.table_io_waits_summary_by_index_usage
WHERE OBJECT_TYPE = 'TABLE'
GROUP BY OBJECT_NAME
ORDER BY seq_scan DESC`
	default:
		return ""
	}
}

// seqScanVerdict classifies one table's counters.
func seqScanVerdict(seq, idx int64) string {
	switch {
	case seq+idx == 0:
		return "no scans recorded since stats reset"
	case seq >= 10 && seq > 3*idx:
		return "indexing candidate — sequential scans dominate"
	case idx > seq*3:
		return "index access dominates"
	default:
		return "mixed access"
	}
}

// FindSeqScanHeavy renders every tracked table's scan counters with a
// per-table verdict, hottest sequential scans first.
func (uc *DatabaseUseCase) FindSeqScanHeavy(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := seqScanQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("sequential-scan introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("scan-counter catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing seq-scan rows: %v", closeErr)
		}
	}()

	var lines []string
	candidates := 0
	for rows.Next() {
		var table string
		var seq, idx int64
		if scanErr := rows.Scan(&table, &seq, &idx); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		verdict := seqScanVerdict(seq, idx)
		if strings.Contains(verdict, "indexing candidate") {
			candidates++
		}
		lines = append(lines, fmt.Sprintf("- %s: %d seq / %d index — %s", table, seq, idx, verdict))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate scan-counter rows: %w", err)
	}

	if len(lines) == 0 {
		return "No scan statistics recorded yet.", nil
	}
	out := fmt.Sprintf("%d table(s) tracked, %d indexing candidate(s):\n%s",
		len(lines), candidates, strings.Join(lines, "\n"))
	if candidates == 0 {
		out += "\nNo table shows sequential-scan dominance."
	}
	return out, nil
}
