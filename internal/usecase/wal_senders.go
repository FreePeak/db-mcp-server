package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// max_wal_senders capacity audit: 0 disables streaming replication
// entirely, and a server already using every sender slot rejects new
// standbys with "too many walsenders" during failover drills — the
// worst moment to discover it. Evidence-driven: live senders
// (pg_stat_replication) and slots (pg_replication_slots) counted
// against the setting.

// walSendersProbe returns the probe pairing the setting with its
// live usage, or "" when unsupported.
func walSendersProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT COALESCE(current_setting('max_wal_senders'), '') AS max_senders,
       (SELECT count(*) FROM pg_stat_replication WHERE pid IS NOT NULL) AS active,
       (SELECT count(*) FROM pg_replication_slots) AS slots`
	default:
		return ""
	}
}

// walSendersVerdict classifies capacity against live usage; healthy
// headroom renders "" so reports stay actionable.
func walSendersVerdict(maxSenders int64, active int64, slots int64) string {
	switch {
	case maxSenders < 0:
		return "max_wal_senders is unreadable — verify with SHOW max_wal_senders."
	case maxSenders == 0:
		return "WARNING: max_wal_senders=0 — streaming replication is disabled: no standby can attach and no logical decoding consumer can run. Fix: ALTER SYSTEM SET max_wal_senders = 5 (plus restart; this one cannot be reloaded)."
	}
	free := maxSenders - active
	if f2 := maxSenders - slots; f2 < free {
		free = f2 // whichever consumer is tighter defines headroom
	}
	if free <= 0 {
		return fmt.Sprintf("WARNING: at capacity — %d active sender(s)/%d slot(s) use all of max_wal_senders=%d: a replacement standby cannot attach during failover. Fix: ALTER SYSTEM SET max_wal_senders = %d plus restart.",
			active, slots, maxSenders, maxSenders+2)
	}
	return "" // headroom available
}

// AuditWalSenders renders whether a standby could still attach; a
// healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditWalSenders(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := walSendersProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("max_wal_senders introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("max_wal_senders query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing wal-senders rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read wal-sender counters: %w", rerr)
		}
		return "", fmt.Errorf("max_wal_senders query returned no rows")
	}

	var maxRaw string
	var activeRaw, slotRaw interface{}
	if scanErr := rows.Scan(&maxRaw, &activeRaw, &slotRaw); scanErr != nil {
		return "", fmt.Errorf("failed to scan wal-sender counters: %w", scanErr)
	}
	parse := func(v interface{}) int64 {
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		n, perr := strconv.ParseInt(s, 10, 64)
		if perr != nil {
			logger.Error("unparseable wal-sender counter %q: %v", s, perr)
			return -1 // renders as unreadable, never guessed at
		}
		return n
	}
	maxSenders := parse(maxRaw)
	active, slots := parse(activeRaw), parse(slotRaw)
	if verdict := walSendersVerdict(maxSenders, active, slots); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("WAL sender capacity healthy: max_wal_senders=%d, %d active, %d slot(s) reserved.",
		maxSenders, active, slots), nil
}
