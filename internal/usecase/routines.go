package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Routine listing: enumerate stored functions and procedures — the last
// behavior layer agents couldn't see (business logic hiding in the engine).

// routinesQuery returns the engine's routine catalog SELECT rendering
// name, type, and signature as the first three columns.
func routinesQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT p.proname,
CASE WHEN prokind = 'f' THEN 'function' WHEN prokind = 'p' THEN 'procedure' ELSE 'other' END,
pg_get_function_identity_arguments(p.oid)
FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname NOT IN ('pg_catalog','information_schema') ORDER BY p.proname`
	case "mysql", "mariadb":
		return `SELECT ROUTINE_NAME, ROUTINE_TYPE,
COALESCE(DTD_IDENTIFIER, '') || ' params: ' || COALESCE(ROUTINE_DEFINITION, '')
FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = DATABASE() ORDER BY ROUTINE_NAME`
	case "oracle":
		return `SELECT object_name, object_type, '' FROM user_objects
WHERE object_type IN ('FUNCTION','PROCEDURE') ORDER BY object_name`
	default:
		return ""
	}
}

// ListRoutines renders every stored function/procedure's name, type, and
// signature. Engines without stored routines report a clean empty list.
func (uc *DatabaseUseCase) ListRoutines(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := routinesQuery(dbType)
	if q == "" {
		return "No stored routines (engine does not support them).", nil
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("routine catalog query failed: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing routine rows: %v", cerr)
		}
	}()

	var lines []string
	for rows.Next() {
		var name, kind, sig interface{}
		if scanErr := rows.Scan(&name, &kind, &sig); scanErr != nil {
			continue
		}
		sigStr := strings.Join(strings.Fields(renderScalar(sig)), " ")
		if len(sigStr) > 200 {
			sigStr = sigStr[:200] + "…(+N)"
		}
		lines = append(lines, fmt.Sprintf("%s (%s): %s",
			renderScalar(name), renderScalar(kind), sigStr))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate failed: %w", err)
	}
	sort.Strings(lines)
	if len(lines) == 0 {
		return "No stored routines defined.", nil
	}
	return fmt.Sprintf("%d stored routine(s):\n- %s", len(lines), strings.Join(lines, "\n- ")), nil
}
