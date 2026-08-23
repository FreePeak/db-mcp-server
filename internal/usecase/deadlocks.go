package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Deadlock-counter audit: lock_waits shows who blocks whom right now,
// but cumulative deadlock events are the durable evidence that a
// concurrency bug fires in production — the engine resolves each one by
// killing a victim query and moving on, leaving only a counter behind.

// deadlockQuery returns the engine's cumulative deadlock-count SELECT,
// or "" when unsupported.
func deadlockQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()`
	case "mysql", "mariadb":
		return `SELECT VARIABLE_VALUE FROM performance_schema.global_status
WHERE VARIABLE_NAME = 'Innodb_deadlocks'`
	default:
		return ""
	}
}

// CheckDeadlocks renders the cumulative deadlock count since stats
// reset; nonzero counts point at engine logs for victim queries.
func (uc *DatabaseUseCase) CheckDeadlocks(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := deadlockQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("deadlock introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("deadlock catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing deadlock rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		return "", fmt.Errorf("deadlock catalog returned no rows")
	}

	var raw any
	if err := rows.Scan(&raw); err != nil {
		return "", fmt.Errorf("failed to scan deadlock row: %w", err)
	}
	n, err := toInt(raw)
	if err != nil {
		return "", fmt.Errorf("unparseable deadlock value %v", raw)
	}

	switch {
	case n == 0:
		return "No deadlocks recorded since stats reset.", nil
	case n < 10:
		return fmt.Sprintf("%d deadlock(s) since stats reset — check engine logs for victim queries.", n), nil
	default:
		return fmt.Sprintf("%d deadlock(s) since stats reset — a recurring concurrency conflict is firing; check engine logs for victim queries and the statements involved.", n), nil
	}
}
