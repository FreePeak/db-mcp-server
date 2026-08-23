package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Crash-safety audit: fsync=off and full_page_writes=off are commonly
// set for benchmarks and bulk loads, then forgotten in production.
// fsync=off lets the OS discard acknowledged commits on power loss;
// full_page_writes=off invites torn-page corruption after a crash —
// silent data corruption is the worst failure mode to discover late.

// crashSafetyQuery returns the probe reading both settings, or ""
// when unsupported.
func crashSafetyQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('fsync') AS fsync,
       current_setting('full_page_writes') AS full_page_writes`
	default:
		return ""
	}
}

// truthySetting parses a PostgreSQL GUC boolean ("on"/"off"/"t"/"f").
func truthySetting(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "on", "true", "t", "yes", "1":
		return true
	default:
		return false
	}
}

// crashSafetyVerdict classifies the pair; both-on renders "" so the
// report stays actionable.
func crashSafetyVerdict(fsync, fullPageWrites bool) string {
	switch {
	case !fsync && !fullPageWrites:
		return "WARNING: fsync=off AND full_page_writes=off — acknowledged commits can vanish on power loss and torn pages can corrupt data after a crash. Fix: ALTER SYSTEM SET fsync = on; ALTER SYSTEM SET full_page_writes = on; then restart. These are benchmark/load-test settings, never production."
	case !fsync:
		return "WARNING: fsync=off — acknowledged commits may be lost on power failure or kernel panic. Fix: ALTER SYSTEM SET fsync = on and restart."
	case !fullPageWrites:
		return "WARNING: full_page_writes=off — a crash mid-write can leave torn pages and silently corrupt data after checkpoint. Fix: ALTER SYSTEM SET full_page_writes = on and restart."
	default:
		return ""
	}
}

// AuditCrashSafety renders whether the engine persists acknowledged
// writes durably; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditCrashSafety(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := crashSafetyQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("fsync introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("crash-safety query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing crash-safety rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read crash-safety settings: %w", rerr)
		}
		return "", fmt.Errorf("crash-safety query returned no rows")
	}

	var fsyncRaw, fpwRaw string
	if scanErr := rows.Scan(&fsyncRaw, &fpwRaw); scanErr != nil {
		return "", fmt.Errorf("failed to scan crash-safety settings: %w", scanErr)
	}
	if verdict := crashSafetyVerdict(truthySetting(fsyncRaw), truthySetting(fpwRaw)); verdict != "" {
		return verdict, nil
	}
	return "Crash safety healthy: fsync=on, full_page_writes=on.", nil
}
