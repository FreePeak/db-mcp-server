package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// table_open_cache pressure audit: MySQL keeps table definitions open
// per session; when the cache is saturated every access pays a
// close-and-reopen and Opened_tables climbs without bound. A saturated
// cache is one SET GLOBAL away from fixed — but only if someone looks.

// tableOpenCacheQuery returns the probe joining the cache size to both
// status counters, or "" when unsupported.
func tableOpenCacheQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT @@GLOBAL.table_open_cache AS cache_size,
       (SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME = 'Open_tables') AS open_now,
       (SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME = 'Opened_tables') AS opened_total`
	default:
		return ""
	}
}

// tableCacheVerdict classifies cache saturation: a full cache with a
// large churn count means entries are being evicted and reopened.
func tableCacheVerdict(cacheSize int64, openNow int64, openedTotal int64) string {
	if cacheSize <= 0 {
		return "table_open_cache is 0 or unreadable — check server configuration."
	}
	if openNow >= cacheSize && openedTotal > 2*cacheSize {
		return fmt.Sprintf("WARNING: table_open_cache saturated (%d/%d in use, %d opens since start) — entries are being evicted; SET GLOBAL table_open_cache=%d (or higher) and watch Opened_tables stop growing.",
			openNow, cacheSize, openedTotal, cacheSize*2)
	}
	return fmt.Sprintf("Table cache healthy: %d/%d slots used, %d opens since start.", openNow, cacheSize, openedTotal)
}

// AuditTableCache renders whether the table-definition cache keeps up;
// a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditTableCache(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := tableOpenCacheQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("table-cache introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("table-cache query failed (requires performance_schema): %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing table-cache rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read table-cache counters: %w", rerr)
		}
		return "", fmt.Errorf("table-cache query returned no rows")
	}

	var cacheRaw, openRaw, openedRaw string
	if scanErr := rows.Scan(&cacheRaw, &openRaw, &openedRaw); scanErr != nil {
		return "", fmt.Errorf("failed to scan table-cache counters: %w", scanErr)
	}
	// Status counters render as strings; unparseable values fall back to
	// zero, which tableCacheVerdict treats as unreadable.
	parse := func(s string) int64 {
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			logger.Error("unparseable table-cache counter %q: %v", s, err)
			return 0
		}
		return n
	}
	return tableCacheVerdict(parse(cacheRaw), parse(openRaw), parse(openedRaw)), nil
}
