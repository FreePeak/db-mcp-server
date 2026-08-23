package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Trigger listing: enumerate triggers so agents can see hidden behavior
// (audit writes, denormalization) behind plain INSERTs and UPDATEs.

// triggersQuery returns the engine's trigger catalog SELECT rendering
// name, target table, and definition as the first three columns.
func triggersQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT t.tgname, c.relname, pg_get_triggerdef(t.oid)
FROM pg_trigger t JOIN pg_class c ON c.oid = t.tgrelid
WHERE NOT t.tgisinternal ORDER BY t.tgname`
	case "mysql", "mariadb":
		return `SELECT TRIGGER_NAME, EVENT_OBJECT_TABLE, ACTION_STATEMENT
FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA = DATABASE() ORDER BY TRIGGER_NAME`
	case "sqlite":
		return `SELECT name, tbl_name, sql FROM sqlite_master WHERE type = 'trigger' AND sql IS NOT NULL ORDER BY name`
	default:
		return ""
	}
}

// ListTriggers renders every trigger's name, target table, and definition.
func (uc *DatabaseUseCase) ListTriggers(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := triggersQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("trigger listing is not supported for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("trigger catalog query failed: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing trigger rows: %v", cerr)
		}
	}()

	var lines []string
	for rows.Next() {
		var name, table, def interface{}
		if scanErr := rows.Scan(&name, &table, &def); scanErr != nil {
			continue
		}
		defStr := strings.Join(strings.Fields(renderScalar(def)), " ")
		if len(defStr) > 300 {
			defStr = defStr[:300] + "…(+N)"
		}
		lines = append(lines, fmt.Sprintf("%s on %s: %s",
			renderScalar(name), renderScalar(table), defStr))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate failed: %w", err)
	}
	sort.Strings(lines)
	if len(lines) == 0 {
		return "No triggers defined.", nil
	}
	return fmt.Sprintf("%d trigger(s):\n- %s", len(lines), strings.Join(lines, "\n- ")), nil
}
