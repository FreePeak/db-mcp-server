package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Stale replication-slot audit: replication_status shows attached
// replicas, but an INACTIVE slot is a different failure mode — its
// consumer died, and PostgreSQL keeps retaining WAL on the slot's
// behalf until the disk fills, silently. One catalog read names the
// slot-holders before they become a disk-full incident.

// staleSlotQuery returns the inactive-slot SELECT (slot name, type,
// retained WAL), or "" when unsupported.
func staleSlotQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT slot_name,
       slot_type,
       COALESCE(pg_size_pretty(pg_wal_lsn_diff(pg_current_wal_lsn(), restart_lsn)), 'unknown') AS retained
FROM pg_replication_slots
WHERE active = false
ORDER BY restart_lsn NULLS FIRST`
	default:
		return ""
	}
}

// ListStaleSlots renders every inactive replication slot with its
// retained WAL volume; active slots are summarized so "all healthy" is
// explicit.
func (uc *DatabaseUseCase) ListStaleSlots(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	if staleSlotQuery(dbType) == "" {
		return "", fmt.Errorf("replication-slot introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	var total int
	rows, err := db.Query(ctx, `SELECT count(*) FROM pg_replication_slots`)
	if err != nil {
		return "", fmt.Errorf("slot-count query failed: %w", err)
	}
	if rows.Next() {
		var n int64
		if scanErr := rows.Scan(&n); scanErr == nil {
			total = int(n)
		}
	}
	if cerr := rows.Close(); cerr != nil {
		logger.Error("error closing slot-count rows: %v", cerr)
	}
	if total == 0 {
		return "No replication slots defined.", nil
	}

	rows, err = db.Query(ctx, staleSlotQuery(dbType))
	if err != nil {
		return "", fmt.Errorf("stale-slot catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing stale-slot rows: %v", closeErr)
		}
	}()

	var lines []string
	for rows.Next() {
		var name, slotType, retained string
		if scanErr := rows.Scan(&name, &slotType, &retained); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		lines = append(lines, fmt.Sprintf("- slot %q (%s): INACTIVE, retaining %s of WAL — drop it or fix its consumer before the disk fills",
			name, slotType, retained))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate stale-slot rows: %w", err)
	}

	if len(lines) == 0 {
		return fmt.Sprintf("All %d replication slot(s) active — no WAL retention risk.", total), nil
	}
	return fmt.Sprintf("%d of %d replication slot(s) are INACTIVE:\n%s",
		len(lines), total, strings.Join(lines, "\n")), nil
}
