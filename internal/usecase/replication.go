package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Replication status: how far behind are the read replicas? Stale
// replica data looks like "my query returns old rows" and nothing in
// the tool surface could see it. Catalog reads only — pg_stat_replication
// on Postgres, SHOW REPLICA STATUS on MySQL 8+.

// replicationStatusQuery returns the engine's replication SELECT, or ""
// when unsupported.
func replicationStatusQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT client_addr, usename, state,
       sent_lsn, replay_lsn, replay_lag
FROM pg_stat_replication`
	case "mysql", "mariadb":
		return "SHOW REPLICA STATUS"
	default:
		return ""
	}
}

// ListReplication renders the engine's replication status: one row per
// attached replica with its replay position/lag. An empty result is a
// finding too — it means no replicas are attached (or this is not the
// primary's view).
func (uc *DatabaseUseCase) ListReplication(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := replicationStatusQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("replication status is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("replication catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing replication rows: %v", closeErr)
		}
	}()

	out, total, err := renderQueryResults(rows, 50, false, VerbosityFull)
	if err != nil {
		return "", fmt.Errorf("failed to render replication rows: %w", err)
	}
	if total == 0 {
		return fmt.Sprintf("No replicas attached to %s (or this connection cannot see them).", dbID), nil
	}
	return out, nil
}
