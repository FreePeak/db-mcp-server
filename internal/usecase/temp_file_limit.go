package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// temp_file_limit audit: the per-session cap on temporary files
// (sorts, hashes, materializations that spill). The default -1 means
// unlimited — one runaway query can fill the disk and take down
// every database on the host. A finite limit converts that blast
// radius into a per-query error instead.

// tempFileLimitProbe returns the probe reading the setting in
// kilobytes, or "" when unsupported.
func tempFileLimitProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('temp_file_limit') AS limit_kb`
	default:
		return ""
	}
}

// tempFileLimitVerdict classifies the setting in KB; a bounded
// limit renders "" so reports stay actionable.
func tempFileLimitVerdict(kb int64) string {
	if kb <= 0 {
		return "WARNING: temp_file_limit is unlimited — one runaway sort/hash can fill the disk and take down every database on this host. Fix: ALTER SYSTEM SET temp_file_limit = '10GB' then SELECT pg_reload_conf(); offending queries fail with a clear error instead of eating the disk."
	}
	return "" // bounded: worst case is one session's quota, not the disk
}

// AuditTempFileLimit renders whether runaway temp-file usage is
// capped; a bounded result is stated explicitly.
func (uc *DatabaseUseCase) AuditTempFileLimit(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := tempFileLimitProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("temp_file_limit introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("temp_file_limit query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing temp-file-limit rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read temp_file_limit: %w", rerr)
		}
		return "", fmt.Errorf("temp_file_limit query returned no rows")
	}

	var raw interface{}
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan temp_file_limit: %w", scanErr)
	}
	s := strings.TrimSpace(fmt.Sprintf("%v", raw))
	kb, perr := strconv.ParseInt(s, 10, 64)
	if perr != nil {
		logger.Error("unparseable temp_file_limit %q: %v", s, perr)
		kb = -1 // renders as unlimited warning, never guessed at
	}
	if verdict := tempFileLimitVerdict(kb); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("temp_file_limit healthy: capped at %s per session.", humanBytes(kb*1024)), nil
}
