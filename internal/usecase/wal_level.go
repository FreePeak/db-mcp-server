package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// wal_level audit: minimal disables streaming replication,
// point-in-time recovery, and logical decoding entirely. It is another
// benchmark-era leftover that surfaces only when a replica fails to
// start or a PITR restore is needed. replica (the default) and
// logical are both healthy.

// walLevelQuery returns the probe for the setting, or "" when
// unsupported.
func walLevelQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('wal_level') AS wal_level`
	default:
		return ""
	}
}

// walLevelVerdict classifies the level; replica/logical render "" so
// reports stay actionable.
func walLevelVerdict(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "replica", "logical":
		return ""
	case "":
		return "wal_level is empty or unreadable — verify with SHOW wal_level."
	default:
		return fmt.Sprintf("WARNING: wal_level=%s disables streaming replication, point-in-time recovery, and logical decoding — a primary crash past archived WAL cannot be recovered. Fix: ALTER SYSTEM SET wal_level = 'replica' and restart.", level)
	}
}

// AuditWALLevel renders whether the engine can support replication and
// recovery; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditWALLevel(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := walLevelQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("wal_level introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("wal_level query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing wal_level rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read wal_level: %w", rerr)
		}
		return "", fmt.Errorf("wal_level query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan wal_level: %w", scanErr)
	}
	if verdict := walLevelVerdict(raw); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("wal_level=%s: replication and point-in-time recovery are available.", strings.TrimSpace(raw)), nil
}
