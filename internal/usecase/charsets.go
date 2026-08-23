package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Charset audit: utf8mb3 columns are deprecated on MySQL 8 and a
// migration landmine — no supplementary-plane characters (emoji, CJK
// ext), tighter index-length limits, and every modern default expects
// utf8mb4. Invisible from the tool surface until a migration or index
// build fails.

// charsetQuery returns the deprecated-charset column SELECT scoped to
// the current schema, or "" when unsupported.
func charsetQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT TABLE_NAME,
       COLUMN_NAME,
       CHARACTER_SET_NAME,
       COALESCE(COLLATION_NAME, '') AS collation
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND CHARACTER_SET_NAME IN ('utf8mb3', 'utf8')
ORDER BY TABLE_NAME, COLUMN_NAME`
	default:
		return ""
	}
}

// AuditCharsets renders every column still on deprecated utf8mb3 with
// its table, charset, and collation; a clean result is stated
// explicitly.
func (uc *DatabaseUseCase) AuditCharsets(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := charsetQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("charset introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("charset catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing charset rows: %v", closeErr)
		}
	}()

	var lines []string
	for rows.Next() {
		var table, col, cs, collation string
		if scanErr := rows.Scan(&table, &col, &cs, &collation); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		line := fmt.Sprintf("- %s.%s: charset %s", table, col, cs)
		if collation != "" {
			line += fmt.Sprintf(", collation %s", collation)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate charset rows: %w", err)
	}

	if len(lines) == 0 {
		return "No deprecated utf8mb3 columns — all text columns are on utf8mb4.", nil
	}
	out := fmt.Sprintf("%d column(s) still on deprecated utf8mb3:\n%s\n",
		len(lines), strings.Join(lines, "\n"))
	out += "Convert per table: ALTER TABLE <t> CONVERT TO CHARACTER SET utf8mb4 " +
		"(check unique index lengths first — mb4 quadruples byte widths)."
	return strings.TrimRight(out, "\n"), nil
}
