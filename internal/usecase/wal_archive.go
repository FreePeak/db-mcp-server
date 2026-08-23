package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// WAL archiver health: on archive_command deployments a failing
// archiver accumulates WAL segments locally until the disk fills and
// silently breaks point-in-time recovery — pg_stat_archiver's failure
// counters are the only warning, and nothing on the tool surface read
// them.

// archiverQuery returns the archiver-statistics SELECT, or "" when
// unsupported.
func archiverQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT COALESCE(archived_count::bigint, 0) AS archived_count,
       COALESCE(failed_count::bigint, 0) AS failed_count,
       COALESCE(last_archived_wal::text, '') AS last_archived_wal,
       COALESCE(last_failed_wal::text, '') AS last_failed_wal
FROM pg_stat_archiver`
	default:
		return ""
	}
}

// archiverVerdict classifies archived/failed counts; the failed WAL
// name is echoed so the operator can inspect it.
func archiverVerdict(archivedCount, failedCount int64, lastArchivedWAL, lastFailedWAL string) string {
	switch {
	case failedCount > 0:
		return fmt.Sprintf("WAL archiver FAILING: %d archived, %d FAILED (last failed segment %s) — WAL accumulates until the disk fills and point-in-time recovery is broken; check archive_command and destination storage.",
			archivedCount, failedCount, walOrNone(lastFailedWAL))
	case archivedCount > 0:
		return fmt.Sprintf("WAL archiver healthy: %d segment(s) archived, no failures (last: %s).",
			archivedCount, walOrNone(lastArchivedWAL))
	default:
		return "WAL archiver has never archived or failed a segment — archive_mode may be off (PITR would not survive)."
	}
}

func walOrNone(wal string) string {
	if wal == "" {
		return "(none)"
	}
	return wal
}

// CheckWALArchive renders the archiver verdict for one database;
// engine-gated to Postgres.
func (uc *DatabaseUseCase) CheckWALArchive(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := archiverQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("WAL-archive introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("archiver catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing archiver rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		return "pg_stat_archiver returned no rows — archive_mode is likely off.", nil
	}
	var archivedCount, failedCount int64
	var lastArchivedWAL, lastFailedWAL string
	if err := rows.Scan(&archivedCount, &failedCount, &lastArchivedWAL, &lastFailedWAL); err != nil {
		return "", fmt.Errorf("failed to scan archiver row: %w", err)
	}
	return archiverVerdict(archivedCount, failedCount, lastArchivedWAL, lastFailedWAL), nil
}
