package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// AUTO_INCREMENT headroom audit: an auto-increment column that reaches
// its type ceiling breaks every insert with a cryptic "out of range"
// error. INT ids on high-write tables are the classic case — the fix
// is a pre-planned BIGINT migration, not an emergency at 3am. PG
// sequences have their own audit; this covers MySQL/MariaDB counters.

// autoIncrementQuery returns the SELECT joining auto_increment columns
// to their table's next value, or "" when unsupported.
func autoIncrementQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT c.TABLE_SCHEMA,
       c.TABLE_NAME,
       c.COLUMN_TYPE,
       COALESCE(t.AUTO_INCREMENT, 0) AS next_val
FROM information_schema.COLUMNS c
JOIN information_schema.TABLES t
  ON t.TABLE_SCHEMA = c.TABLE_SCHEMA AND t.TABLE_NAME = c.TABLE_NAME
WHERE c.EXTRA LIKE '%auto_increment%'
  AND t.AUTO_INCREMENT IS NOT NULL
ORDER BY t.AUTO_INCREMENT DESC`
	default:
		return ""
	}
}

// aiCeiling maps a COLUMN_TYPE to its numeric ceiling; unknown types
// return 0 and are skipped.
func aiCeiling(colType string) uint64 {
	t := strings.ToLower(strings.TrimSpace(colType))
	unsigned := strings.Contains(t, "unsigned")
	base := strings.Fields(t)[0]
	var signedMax uint64
	switch base {
	case "tinyint":
		signedMax = 127
	case "smallint":
		signedMax = 32767
	case "mediumint":
		signedMax = 8388607
	case "int", "integer":
		signedMax = 2147483647
	case "bigint":
		signedMax = 9223372036854775807
	default:
		return 0
	}
	if unsigned {
		return signedMax*2 + 1
	}
	return signedMax
}

// aiRiskLine renders one counter's headroom; comfortable tables render
// "" so the report stays actionable.
func aiRiskLine(table, colType string, next uint64) string {
	ceiling := aiCeiling(colType)
	if ceiling == 0 || next == 0 {
		return ""
	}
	switch {
	case next >= ceiling:
		return fmt.Sprintf("- %s (%s): AT CEILING — inserts will fail NOW; migrate to BIGINT UNSIGNED immediately",
			table, colType)
	case next*10 >= ceiling*9: // ≥90% of ceiling
		return fmt.Sprintf("- %s (%s): WARNING %d/%d used — plan the BIGINT migration before it fails mid-traffic",
			table, colType, next, ceiling)
	default:
		return ""
	}
}

// AuditAutoIncrement renders every auto-increment counter at or near
// its type ceiling; a clean result is stated explicitly.
func (uc *DatabaseUseCase) AuditAutoIncrement(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := autoIncrementQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("AUTO_INCREMENT introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("AUTO_INCREMENT catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing auto-increment rows: %v", closeErr)
		}
	}()

	var lines []string
	counters := 0
	for rows.Next() {
		var schema, name, colType string
		var next int64
		if scanErr := rows.Scan(&schema, &name, &colType, &next); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		counters++
		full := schema + "." + name
		if line := aiRiskLine(full, colType, uint64(next)); line != "" {
			lines = append(lines, line)
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate auto-increment rows: %w", err)
	}

	if len(lines) == 0 {
		out := fmt.Sprintf("No auto-increment counters at risk across %d audited.", counters)
		if counters == 0 {
			out = "No AUTO_INCREMENT columns found."
		}
		return out, nil
	}
	return fmt.Sprintf("%d of %d auto-increment counter(s) at risk:\n%s",
		len(lines), counters, strings.Join(lines, "\n")), nil
}
