package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// DDL dump: emit the engine's stored CREATE statements verbatim — the
// fastest path from a live schema to migration authoring or environment
// cloning. SQLite stores the original text; server engines reconstruct
// it, so only SQLite is first-class for now.

// DumpDDL renders every stored CREATE statement (tables, indexes, views,
// triggers) in definition order.
func (uc *DatabaseUseCase) DumpDDL(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	switch strings.ToLower(dbType) {
	case "sqlite":
		// handled below
	default:
		return "", fmt.Errorf("ddl dump is not yet supported for engine %q (sqlite only)", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	rows, err := db.Query(ctx,
		`SELECT sql FROM sqlite_master WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY rowid`)
	if err != nil {
		return "", fmt.Errorf("ddl catalog query failed: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing ddl rows: %v", cerr)
		}
	}()

	var stmts []string
	for rows.Next() {
		var sql interface{}
		if scanErr := rows.Scan(&sql); scanErr != nil {
			continue
		}
		s := strings.TrimRight(strings.TrimSpace(renderScalar(sql)), ";")
		stmts = append(stmts, s+";")
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate failed: %w", err)
	}
	if len(stmts) == 0 {
		return "No schema objects to dump.", nil
	}
	return fmt.Sprintf("-- %d object(s)\n%s", len(stmts), strings.Join(stmts, "\n\n")), nil
}
