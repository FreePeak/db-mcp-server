package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Custom type listing: user-defined enum and composite types matter when
// generating migrations or code against engines that support them.

// customTypesQuery returns the engine's type catalog SELECT rendering
// name, kind, and values/attributes as the first three columns.
func customTypesQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT t.typname, t.typtype::text,
COALESCE((SELECT string_agg(e.enumlabel, ', ' ORDER BY e.enumsortorder)
  FROM pg_enum e WHERE e.enumtypid = t.oid), '')
FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
WHERE t.typtype IN ('e','c') AND n.nspname NOT IN ('pg_catalog','information_schema')
ORDER BY t.typname`
	default:
		return ""
	}
}

// ListCustomTypes renders user-defined types (enums with their labels,
// composites noted). Engines without the concept report a clean empty list.
func (uc *DatabaseUseCase) ListCustomTypes(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := customTypesQuery(dbType)
	if q == "" {
		return "No custom types (engine does not support them).", nil
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("type catalog query failed: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing type rows: %v", cerr)
		}
	}()

	var lines []string
	for rows.Next() {
		var name, kind, vals interface{}
		if scanErr := rows.Scan(&name, &kind, &vals); scanErr != nil {
			continue
		}
		kindStr := renderScalar(kind)
		detail := renderScalar(vals)
		if detail == "" {
			detail = "(composite)"
		}
		lines = append(lines, fmt.Sprintf("%s (%s): %s", renderScalar(name), kindStr, detail))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate failed: %w", err)
	}
	sort.Strings(lines)
	if len(lines) == 0 {
		return "No custom types defined.", nil
	}
	return fmt.Sprintf("%d custom type(s):\n- %s", len(lines), strings.Join(lines, "\n- ")), nil
}
