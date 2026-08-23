package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// open_files_limit audit: MySQL's OS-level file-descriptor ceiling
// must supply the table cache, InnoDB files, and every connection. If
// open_files_limit is below roughly 2x table_open_cache, the cache is
// silently capped — evictions continue no matter how high the cache is
// tuned, and busy schemas hit "Too many open files". Unlike most
// settings this one cannot be raised with SET GLOBAL; it needs the
// config file (or ulimit) plus a restart.

// openFilesLimitQuery returns the probe reading both settings in one
// round trip, or "" when unsupported.
func openFilesLimitQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT @@GLOBAL.open_files_limit AS fd_limit,
       @@GLOBAL.table_open_cache AS table_cache`
	default:
		return ""
	}
}

// openFilesLimitVerdict classifies the ceiling against what the table
// cache alone can demand; comfortable ceilings render "" so reports
// stay actionable.
func openFilesLimitVerdict(fdLimit, tableCache int64) string {
	if fdLimit <= 0 {
		return "open_files_limit is 0 or unreadable — check server configuration."
	}
	if tableCache > 0 && fdLimit < tableCache*2 {
		return fmt.Sprintf("WARNING: open_files_limit=%d cannot supply table_open_cache=%d (each cached table can need ~2 descriptors once partitioning and triggers are counted) — evictions and \"Too many open files\" follow no matter how high the cache is tuned. Fix: set open_files_limit in my.cnf (or raise the service ulimit) and restart — SET GLOBAL does not work for this one.",
			fdLimit, tableCache)
	}
	return ""
}

// AuditOpenFilesLimit renders whether the descriptor ceiling plausibly
// supplies the table cache; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditOpenFilesLimit(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := openFilesLimitQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("open_files_limit introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("open_files_limit query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing open_files_limit rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read open_files_limit: %w", rerr)
		}
		return "", fmt.Errorf("open_files_limit query returned no rows")
	}

	var rawLimit, rawCache string
	if scanErr := rows.Scan(&rawLimit, &rawCache); scanErr != nil {
		return "", fmt.Errorf("failed to scan open_files_limit settings: %w", scanErr)
	}
	parse := func(s string) int64 {
		n, perr := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if perr != nil {
			logger.Error("unparseable open_files_limit setting %q: %v", s, perr)
			return 0
		}
		return n
	}
	limit, cache := parse(rawLimit), parse(rawCache)
	if verdict := openFilesLimitVerdict(limit, cache); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("File-descriptor ceiling healthy: open_files_limit=%d vs table_open_cache=%d.",
		limit, cache), nil
}
