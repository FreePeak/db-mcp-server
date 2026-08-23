package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// MyISAM audit: tables still on the MyISAM engine have no transactions,
// table-level write locks (every writer queues behind every other), and
// crash-unsafe storage — an unclean shutdown can corrupt indexes and
// lose in-flight rows. InnoDB has been the default since MySQL 5.5; a
// MyISAM survivor is almost always an accident. Only
// information_schema.TABLES reveals which tables never migrated.

// myISAMQuery returns the SELECT for BASE TABLEs on the MyISAM engine,
// or "" when unsupported.
func myISAMQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT TABLE_SCHEMA,
       TABLE_NAME,
       COALESCE(TABLE_ROWS, 0) AS row_estimate
FROM information_schema.TABLES
WHERE ENGINE = 'MyISAM'
  AND TABLE_TYPE = 'BASE TABLE'
ORDER BY TABLE_SCHEMA, TABLE_NAME`
	default:
		return ""
	}
}

// ListMyISAMTables renders every MyISAM table with its migration cost;
// a clean result is stated explicitly.
func (uc *DatabaseUseCase) ListMyISAMTables(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := myISAMQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("engine introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("engine catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing MyISAM rows: %v", closeErr)
		}
	}()

	var lines []string
	for rows.Next() {
		var schema, table string
		var rowEst int64
		if scanErr := rows.Scan(&schema, &table, &rowEst); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		lines = append(lines, fmt.Sprintf(
			"- %s.%s (~%d row(s)): MyISAM — no transactions, table-level locks, corruption-prone on unclean shutdown",
			schema, table, rowEst))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate MyISAM rows: %w", err)
	}

	if len(lines) == 0 {
		return "No MyISAM tables — all BASE TABLEs run a transactional engine.", nil
	}
	out := fmt.Sprintf("%d MyISAM table(s):\n%s\n", len(lines), strings.Join(lines, "\n"))
	out += "Migrate with ALTER TABLE <t> ENGINE=InnoDB (locks the table while copying)."
	return strings.TrimRight(out, "\n"), nil
}
