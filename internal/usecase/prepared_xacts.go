package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Orphaned two-phase-commit audit: a forgotten PREPARE TRANSACTION
// holds its locks forever and pins the vacuum horizon — invisible to
// list_sessions (no statement is running) and to long_transactions
// (the session may be gone entirely). Only the prepared-transaction
// registry itself reveals them.

// preparedXactQuery returns the prepared-transaction SELECT with age
// rendering, or "" when unsupported.
func preparedXactQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT gid,
       COALESCE(owner, '') AS owner,
       COALESCE(database::text, '') AS database,
       now() - prepared AS age
FROM pg_prepared_xacts
ORDER BY prepared`
	default:
		return ""
	}
}

// ListPreparedTransactions renders every in-doubt (two-phase)
// transaction with owner, database, and age; empty is stated
// explicitly.
func (uc *DatabaseUseCase) ListPreparedTransactions(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := preparedXactQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("prepared-transaction introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("prepared-transaction catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing prepared-xact rows: %v", closeErr)
		}
	}()

	var lines []string
	for rows.Next() {
		var gid, owner, dbname string
		var age any
		if scanErr := rows.Scan(&gid, &owner, &dbname, &age); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		lines = append(lines, fmt.Sprintf("- gid %q (db %s, owner %s): open for %s — holds locks and pins vacuum; ROLLBACK PREPARED %q if abandoned, COMMIT PREPARED to finish",
			gid, dbname, owner, strings.TrimSpace(fmt.Sprintf("%v", age)), gid))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate prepared-xact rows: %w", err)
	}

	if len(lines) == 0 {
		return "No prepared (two-phase) transactions in flight.", nil
	}
	return fmt.Sprintf("%d prepared (two-phase) transaction(s) in doubt:\n%s",
		len(lines), strings.Join(lines, "\n")), nil
}
