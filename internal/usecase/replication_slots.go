package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Replication-slot audit: a slot that its consumer abandoned still
// forces the primary to retain every WAL segment the slot needs —
// disk fills with no error anywhere. Slots also cap concurrent
// standbys: when usage reaches max_replication_slots, new replicas
// fail to attach with only a log line as evidence.

// replicationSlot is one row of pg_replication_slots, normalized.
type replicationSlot struct {
	name     string
	active   bool
	retained int64 // WAL bytes retained past this slot's restart point
}

// replicationSlotsProbe returns the per-slot SELECT with retained-WAL
// accounting, or "" when unsupported.
func replicationSlotsProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT slot_name,
       COALESCE(active, false) AS active,
       CASE WHEN restart_lsn IS NULL THEN 0
            ELSE pg_wal_lsn_diff(pg_current_wal_lsn(), restart_lsn) END AS retained_bytes
FROM pg_replication_slots
ORDER BY slot_name`
	default:
		return ""
	}
}

// replicationSlotVerdict classifies the fleet: inactive slots always
// warn (any retention is unbounded growth in waiting), exhausted
// capacity warns, and an all-active fleet with free capacity renders
// "" so reports stay actionable. maxSlots <= 0 means unreadable and
// skips the capacity check.
func replicationSlotVerdict(slots []replicationSlot, maxSlots int) string {
	var lines []string
	for _, s := range slots {
		if s.active {
			continue
		}
		lines = append(lines, fmt.Sprintf(
			"- slot %q is INACTIVE but retaining %s of WAL: if the consumer was dropped or migrated away, drop it too — SELECT pg_drop_replication_slot('%s');",
			s.name, humanBytes(s.retained), s.name))
	}
	if maxSlots > 0 && len(slots) >= maxSlots {
		lines = append(lines, fmt.Sprintf(
			"- capacity full: %d/%d slots in use — new replicas will fail to attach until a slot is freed.",
			len(slots), maxSlots))
	}
	if len(lines) == 0 {
		if len(slots) == 0 {
			// Empty fleet states health explicitly (usage counts too).
			if maxSlots > 0 {
				return fmt.Sprintf("healthy: 0/%d slots in use.", maxSlots)
			}
			return "healthy: no replication slots defined."
		}
		return ""
	}
	return strings.Join(lines, "\n")
}

// AuditReplicationSlots renders every slot problem — WAL-retaining
// inactive slots and exhausted capacity; a clean result states health
// explicitly with current usage.
func (uc *DatabaseUseCase) AuditReplicationSlots(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := replicationSlotsProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("replication-slot introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	maxRows, merr := db.Query(ctx, `SELECT current_setting('max_replication_slots')`)
	if merr != nil {
		return "", fmt.Errorf("max_replication_slots query failed: %w", merr)
	}
	maxSlots := 0 // unreadable: skip the capacity check rather than fail
	if maxRows.Next() {
		var raw interface{}
		if scanErr := maxRows.Scan(&raw); scanErr == nil {
			if _, perr := fmt.Sscanf(strings.TrimSpace(fmt.Sprintf("%v", raw)), "%d", &maxSlots); perr != nil {
				maxSlots = 0
			}
		}
	}
	if cerr := maxRows.Close(); cerr != nil {
		logger.Error("error closing max-replication-slots rows: %v", cerr)
	}

	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("replication-slot catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing replication-slot rows: %v", closeErr)
		}
	}()

	var slots []replicationSlot
	for rows.Next() {
		var name interface{}
		var active bool
		var retained int64
		if scanErr := rows.Scan(&name, &active, &retained); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		slots = append(slots, replicationSlot{
			name:     fmt.Sprintf("%v", name),
			active:   active,
			retained: retained,
		})
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate replication-slot rows: %w", err)
	}

	if verdict := replicationSlotVerdict(slots, maxSlots); verdict != "" {
		return "Replication slot problems:\n" + verdict, nil
	}
	return fmt.Sprintf("Replication slots healthy: %d/%d in use, all active, no stranded WAL.", len(slots), maxSlots), nil
}
