package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// wait_timeout audit: MySQL closes connections idle longer than this.
// Both extremes hurt: too high (the 28800s default is 8 hours) and
// idle connections from crashed clients hold pool slots until
// "too many connections"; too low and pooled connections are dropped
// server-side mid-idle, surfacing as "server has gone away" errors.

const (
	waitTimeoutFloorSecs = 30       // below this, churn dominates
	waitTimeoutCeilSecs  = 8 * 3600 // above the default is suspicious
)

// waitTimeoutQuery returns the probe for the global setting, or ""
// when unsupported.
func waitTimeoutQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT COALESCE(@@GLOBAL.wait_timeout, 0) AS wait_timeout`
	default:
		return ""
	}
}

// humanHours renders seconds as a readable duration.
func humanHours(secs int64) string {
	if secs%3600 == 0 {
		return fmt.Sprintf("%dh", secs/3600)
	}
	if secs < 120 {
		return fmt.Sprintf("%ds", secs)
	}
	return fmt.Sprintf("%.1fh", float64(secs)/3600)
}

// waitTimeoutVerdict classifies the idle timeout two-sidedly; a value
// inside the healthy band renders "" so reports stay actionable.
func waitTimeoutVerdict(secs int64) string {
	switch {
	case secs <= 0:
		return "wait_timeout is 0 or unreadable — verify with SHOW GLOBAL VARIABLES LIKE 'wait_timeout'."
	case secs < waitTimeoutFloorSecs:
		return fmt.Sprintf("WARNING: wait_timeout=%ss drops connections idle only briefly — pooled connections die server-side and apps see 'server has gone away' errors. Fix: SET GLOBAL wait_timeout=600 (and match it in the client pool config).", strconv.FormatInt(secs, 10))
	case secs > waitTimeoutCeilSecs:
		return fmt.Sprintf("WARNING: wait_timeout=%s lets idle connections from crashed clients hold slots for %s before cleanup — expect 'too many connections' under churn. Fix: SET GLOBAL wait_timeout=600.", humanHours(secs), humanHours(secs))
	default:
		return ""
	}
}

// AuditWaitTimeout renders whether the idle timeout sits in a sane
// band; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditWaitTimeout(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := waitTimeoutQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("wait_timeout introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("wait_timeout query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing wait_timeout rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read wait_timeout: %w", rerr)
		}
		return "", fmt.Errorf("wait_timeout query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan wait_timeout: %w", scanErr)
	}
	secs, perr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if perr != nil {
		logger.Error("unparseable wait_timeout %q: %v", raw, perr)
		secs = 0
	}
	if verdict := waitTimeoutVerdict(secs); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("wait_timeout=%s: idle timeout sits in a sane band (30s–8h).", humanHours(secs)), nil
}
