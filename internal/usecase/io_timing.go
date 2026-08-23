package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// track_io_timing audit: when off (the PostgreSQL default), EXPLAIN
// ANALYZE reports no I/O timings and pg_stat views lack block
// read/write time — silently degrading exactly the tooling agents rely
// on for tuning. The measurement overhead is small on modern kernels.

// trackIoTimingQuery returns the probe for the current setting, or ""
// when unsupported.
func trackIoTimingQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT COALESCE(current_setting('track_io_timing'), 'off') AS setting`
	default:
		return ""
	}
}

// trackIoTimingVerdict classifies the setting; "on" renders "" so
// reports stay actionable.
func trackIoTimingVerdict(setting string) string {
	switch strings.ToLower(strings.TrimSpace(setting)) {
	case "":
		return "track_io_timing is unreadable on this server — verify with SHOW track_io_timing."
	case "on":
		return "" // healthy default worth having; the audit adds the explicit clean line
	default:
		return "WARNING: track_io_timing=off — EXPLAIN ANALYZE and pg_stat views report no I/O timings, hiding whether queries are CPU- or disk-bound. Fix: ALTER SYSTEM SET track_io_timing = 'on' (overhead is small on modern kernels)."
	}
}

// AuditTrackIoTiming renders whether the engine measures block I/O;
// a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditTrackIoTiming(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := trackIoTimingQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("track_io_timing introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("track_io_timing query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing track_io_timing rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read track_io_timing: %w", rerr)
		}
		return "", fmt.Errorf("track_io_timing query returned no rows")
	}

	var setting string
	if scanErr := rows.Scan(&setting); scanErr != nil {
		return "", fmt.Errorf("failed to scan track_io_timing: %w", scanErr)
	}
	if verdict := trackIoTimingVerdict(setting); verdict != "" {
		return verdict, nil
	}
	return "track_io_timing=on: EXPLAIN ANALYZE and pg_stat views include block I/O timings.", nil
}
