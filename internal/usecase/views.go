package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// View listing: enumerate views with their SQL definitions so agents can
// understand derived tables and reuse them in queries. Engine catalogs
// differ; one query per engine family.

// viewsQuery returns (query, nameCol, defCol) for the engine, or "" when
// unsupported.
func viewsQuery(dbType string) (query, nameCol, defCol string) {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT viewname, definition FROM pg_views WHERE schemaname NOT IN ('pg_catalog','information_schema') ORDER BY viewname`,
			"viewname", "definition"
	case "mysql", "mariadb":
		return `SELECT TABLE_NAME, VIEW_DEFINITION FROM information_schema.VIEWS WHERE TABLE_SCHEMA = DATABASE() ORDER BY TABLE_NAME`,
			"TABLE_NAME", "VIEW_DEFINITION"
	case "sqlite":
		return `SELECT name, sql FROM sqlite_master WHERE type = 'view' ORDER BY name`, "name", "sql"
	default:
		return "", "", ""
	}
}

// ListViews renders every view's name and definition.
func (uc *DatabaseUseCase) ListViews(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q, _, _ := viewsQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("view listing is not supported for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("view catalog query failed: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing view rows: %v", cerr)
		}
	}()

	var lines []string
	for rows.Next() {
		var name, def interface{}
		if scanErr := rows.Scan(&name, &def); scanErr != nil {
			continue
		}
		defStr := renderScalar(def)
		defStr = strings.Join(strings.Fields(defStr), " ")
		if len(defStr) > 300 {
			defStr = defStr[:300] + "…(+N)"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", renderScalar(name), defStr))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate failed: %w", err)
	}
	sort.Strings(lines)
	if len(lines) == 0 {
		return "No views defined.", nil
	}
	return fmt.Sprintf("%d view(s):\n- %s", len(lines), strings.Join(lines, "\n- ")), nil
}
