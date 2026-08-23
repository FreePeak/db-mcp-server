package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// max_slot_wal_keep_size audit: with the default -1, any replication
// slot — including one nobody remembers creating — retains WAL in
// pg_wal without bound until the disk fills. A cap converts that
// failure mode into bounded slot invalidation: a lagging slot is
// dropped rather than taking the primary down. Stale-slot detection
// (action=stale_slots) watches existing slots; this checks that the
// global safety net exists at all.

// slotWalCapQuery returns the probe for the setting, or "" when
// unsupported.
func slotWalCapQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('max_slot_wal_keep_size') AS cap`
	default:
		return ""
	}
}

// slotWalCapVerdict classifies the safety net; a positive cap renders
// "" so reports stay actionable.
func slotWalCapVerdict(v string) string {
	s := strings.TrimSpace(v)
	switch {
	case s == "":
		return "max_slot_wal_keep_size is empty or unreadable — verify with SHOW max_slot_wal_keep_size."
	case s == "-1":
		return "WARNING: max_slot_wal_keep_size=-1 — every replication slot retains WAL unboundedly, so one forgotten slot can fill pg_wal and take the primary down. Fix: ALTER SYSTEM SET max_slot_wal_keep_size='100GB' (tune to headroom above normal lag) then SELECT pg_reload_conf(); lagging slots beyond the cap are invalidated instead of filling the disk."
	default:
		return ""
	}
}

// AuditSlotWalCap renders whether the WAL-retention safety net exists;
// a capped result is stated explicitly.
func (uc *DatabaseUseCase) AuditSlotWalCap(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := slotWalCapQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("slot-WAL-cap introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("slot-WAL-cap query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing slot-WAL-cap rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read max_slot_wal_keep_size: %w", rerr)
		}
		return "", fmt.Errorf("slot-WAL-cap query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan max_slot_wal_keep_size: %w", scanErr)
	}
	if verdict := slotWalCapVerdict(raw); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("WAL retention capped: max_slot_wal_keep_size=%s.", strings.TrimSpace(raw)), nil
}
