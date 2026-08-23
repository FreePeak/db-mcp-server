package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// checkpoint_timeout audit: how often PostgreSQL force-checkpoints.
// Too short and every dirty page gets a full-page write into WAL
// each cycle — checkpoint storms inflate WAL volume and produce I/O
// spikes that look like random slowdowns. Too long and crash
// recovery replays more WAL, stretching restart time after a crash.
// The 5-minute default is conservative; 15–30 minutes suits most
// write-heavy workloads.

const (
	checkpointTimeoutFloorSecs = 300  // below: storm territory (5 min)
	checkpointTimeoutCeilSecs  = 3600 // above: recovery-replay territory (1 h)
)

// checkpointTimeoutProbe returns the probe reading the setting in
// seconds via EXTRACT(EPOCH FROM ...), or "" when unsupported.
func checkpointTimeoutProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT COALESCE(EXTRACT(EPOCH FROM current_setting('checkpoint_timeout')::interval), 0) AS timeout_secs`
	default:
		return ""
	}
}

// checkpointTimeoutVerdict classifies the setting; healthy values
// render "" so reports stay actionable.
func checkpointTimeoutVerdict(secs int64) string {
	switch {
	case secs <= 0:
		return "checkpoint_timeout is unreadable — verify with SHOW checkpoint_timeout."
	case secs < checkpointTimeoutFloorSecs:
		return fmt.Sprintf("WARNING: checkpoint_timeout=%ds (<%d) — checkpoints fire so often that every dirty page earns full-page writes into WAL each cycle: inflated WAL volume plus periodic I/O spikes. Fix: ALTER SYSTEM SET checkpoint_timeout = '15min' then SELECT pg_reload_conf().",
			secs, checkpointTimeoutFloorSecs)
	case secs > checkpointTimeoutCeilSecs:
		return fmt.Sprintf("WARNING: checkpoint_timeout=%ds (>%d) — crash recovery must replay up to this much WAL before the server accepts connections. Fix: ALTER SYSTEM SET checkpoint_timeout = '30min' then SELECT pg_reload_conf().",
			secs, checkpointTimeoutCeilSecs)
	default:
		return "" // balanced storm/recovery trade-off
	}
}

// AuditCheckpointTimeout renders whether checkpoints are storm-prone
// or recovery-heavy; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditCheckpointTimeout(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := checkpointTimeoutProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("checkpoint_timeout introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("checkpoint_timeout query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing checkpoint-timeout rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read checkpoint_timeout: %w", rerr)
		}
		return "", fmt.Errorf("checkpoint_timeout query returned no rows")
	}

	var secsRaw interface{}
	if scanErr := rows.Scan(&secsRaw); scanErr != nil {
		return "", fmt.Errorf("failed to scan checkpoint_timeout: %w", scanErr)
	}
	s := strings.TrimSpace(fmt.Sprintf("%v", secsRaw))
	secs, perr := strconv.ParseInt(s, 10, 64)
	if perr != nil {
		logger.Error("unparseable checkpoint_timeout %q: %v", s, perr)
		secs = -1 // renders as unreadable, never guessed at
	}
	if verdict := checkpointTimeoutVerdict(secs); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("checkpoint_timeout healthy: %d minute(s) — no checkpoint storms, bounded crash-recovery replay.",
		secs/60), nil
}
