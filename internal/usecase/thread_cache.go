package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// thread_cache_size churn audit: every connection MySQL can't serve
// from the thread cache pays a thread-create/destroy cycle. The
// status counters make this evidence-driven — Threads_created vs
// Connections is the observed miss rate, so a small cache on a
// low-traffic box reads healthy while a large cache under a
// connect-storm still escalates.

const (
	threadCacheMissThreshold = 0.05 // >5% of connections created threads
)

// threadCacheProbe returns the probe joining the setting to its
// churn counters, or "" when unsupported.
func threadCacheProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT @@GLOBAL.thread_cache_size AS cache_size,
       (SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME = 'Connections') AS conns,
       (SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME = 'Threads_created') AS created`
	default:
		return ""
	}
}

// threadCacheVerdict classifies churn against the configured cache;
// healthy results render "" so reports stay actionable.
func threadCacheVerdict(cacheSize int64, conns int64, created int64) string {
	if conns == 0 {
		return fmt.Sprintf("No connections yet — thread_cache_size=%d has no churn evidence to judge; re-run after traffic.", cacheSize)
	}
	miss := float64(created) / float64(conns)
	if created > 0 && miss > threadCacheMissThreshold {
		suggested := cacheSize * 4
		if suggested < 16 {
			suggested = 16
		}
		if suggested > 100 {
			suggested = 100
		}
		return fmt.Sprintf("WARNING: Threads_created=%d of %d connections (%.1f%%) — new connections are paying thread-creation cost because the cache misses. Fix: SET GLOBAL thread_cache_size=%d (apply live) and persist in my.cnf; watch Threads_created flatten afterwards.",
			created, conns, miss*100, suggested)
	}
	return "" // low churn relative to traffic
}

// AuditThreadCache renders whether connection churn is being served
// from the thread cache; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditThreadCache(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := threadCacheProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("thread_cache_size introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("thread_cache_size query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing thread-cache rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read thread-cache counters: %w", rerr)
		}
		return "", fmt.Errorf("thread_cache_size query returned no rows")
	}

	var sizeRaw, connRaw, createdRaw string
	if scanErr := rows.Scan(&sizeRaw, &connRaw, &createdRaw); scanErr != nil {
		return "", fmt.Errorf("failed to scan thread-cache counters: %w", scanErr)
	}
	parse := func(s string) int64 {
		n, perr := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if perr != nil {
			logger.Error("unparseable thread-cache counter %q: %v", s, perr)
			return -1 // renders as unreadable, never guessed at
		}
		return n
	}
	size, conns, created := parse(sizeRaw), parse(connRaw), parse(createdRaw)
	switch {
	case size < 0:
		return "", fmt.Errorf("thread_cache_size unreadable (%q)", strings.TrimSpace(sizeRaw))
	case conns < 0 || created < 0:
		return "", fmt.Errorf("thread-cache status counters unreadable (conns=%q created=%q)",
			strings.TrimSpace(connRaw), strings.TrimSpace(createdRaw))
	}
	if verdict := threadCacheVerdict(size, conns, created); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("Thread cache healthy: thread_cache_size=%d, %d/%d connections needed a new thread (<%.0f%%).",
		size, created, conns, threadCacheMissThreshold*100), nil
}
