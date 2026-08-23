package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// wal_sender_timeout audit: the walsender heartbeat that reaps dead
// standbys. 0 disables it — a crashed standby never gets noticed, its
// replication slot keeps pinning WAL, and retention grows until disk
// fills. Very low values kill healthy-but-slow standbys on flaky
// networks (reconnect storms); sane values stay quiet.

const (
	// walSenderTimeoutFloorSecs: below this, brief network blips or a
	// slow standby checkpoint gap terminate healthy sessions.
	walSenderTimeoutFloorSecs = 10
)

// walSenderTimeoutProbe returns the probe for the setting, or ""
// when unsupported.
func walSenderTimeoutProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT COALESCE(current_setting('wal_sender_timeout'), '') AS wst`
	default:
		return ""
	}
}

// parseTimeoutSecs converts PostgreSQL interval strings ("60s",
// "1min", "5000", "2 min") to seconds; ok=false when unparseable.
func parseTimeoutSecs(raw string) (int64, bool) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return 0, false
	}
	var num float64
	var unit string
	if _, err := fmt.Sscanf(s, "%f%s", &num, &unit); err != nil {
		if n, err2 := strconv.ParseFloat(s, 64); err2 == nil {
			return int64(n), true // bare number means milliseconds
		}
		return 0, false
	}
	switch {
	case strings.HasPrefix(unit, "ms"):
		return int64(num / 1000), true
	case strings.HasPrefix(unit, "s"):
		return int64(num), true
	case strings.HasPrefix(unit, "min"):
		return int64(num * 60), true
	case strings.HasPrefix(unit, "h"):
		return int64(num * 3600), true
	case strings.HasPrefix(unit, "d"):
		return int64(num * 86400), true
	default:
		return 0, false
	}
}

// walSenderTimeoutVerdict classifies the setting; sane values render
// "" so reports stay actionable. Unparseable values read as
// unreadable rather than guessed at.
func walSenderTimeoutVerdict(raw string) string {
	secs, ok := parseTimeoutSecs(raw)
	switch {
	case !ok:
		return "wal_sender_timeout is unreadable — verify with SHOW wal_sender_timeout."
	case secs == 0:
		return "WARNING: wal_sender_timeout=0 — dead-standby detection is disabled: a crashed replica's walsender never exits, its slot pins WAL, and pg_wal grows until the disk fills. Fix: ALTER SYSTEM SET wal_sender_timeout='60s' then SELECT pg_reload_conf(); existing walsenders pick it up on their next cycle."
	case secs < walSenderTimeoutFloorSecs:
		return fmt.Sprintf("WARNING: wal_sender_timeout=%ds — aggressive: brief network blips or a standby mid-checkpoint terminate healthy streaming sessions and trigger reconnect storms on flaky links. Consider ALTER SYSTEM SET wal_sender_timeout='60s' then SELECT pg_reload_conf().", secs)
	default:
		return "" // sane reap window
	}
}

// AuditWalSenderTimeout renders whether dead standbys get reaped;
// a sane result is stated explicitly.
func (uc *DatabaseUseCase) AuditWalSenderTimeout(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := walSenderTimeoutProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("wal_sender_timeout introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("wal_sender_timeout query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing wal_sender_timeout rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read wal_sender_timeout: %w", rerr)
		}
		return "", fmt.Errorf("wal_sender_timeout query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan wal_sender_timeout: %w", scanErr)
	}
	if verdict := walSenderTimeoutVerdict(raw); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("WAL sender timeout healthy: wal_sender_timeout=%s.", strings.TrimSpace(raw)), nil
}
