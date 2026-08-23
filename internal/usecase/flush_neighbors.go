package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// innodb_flush_neighbors audit: the default 1 groups flushing of
// adjacent dirty pages — calibrated for spinning disks where seeking
// dominates. On SSD/NVMe (nearly all modern deployments) there is no
// seek penalty to amortize, so neighbor coalescing only batches
// writes the storage didn't ask for and adds latency spikes. Zero it
// on flash; keep it only for genuine spinning-disk data drives.

// flushNeighborsQuery returns the probe for the setting, or "" when
// unsupported.
func flushNeighborsQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT COALESCE(@@GLOBAL.innodb_flush_neighbors, -1)`
	default:
		return ""
	}
}

// flushNeighborsVerdict classifies the setting; SSD-tuned renders ""
// so reports stay actionable.
func flushNeighborsVerdict(v int64) string {
	switch {
	case v == 0:
		return "" // SSD-appropriate
	case v < 0:
		return "innodb_flush_neighbors is unreadable — verify with SHOW GLOBAL VARIABLES LIKE 'innodb_flush_neighbors'."
	case v >= 1:
		return fmt.Sprintf("WARNING: innodb_flush_neighbors=%d — adjacent-page coalescing is a spinning-disk optimization; on SSD/NVMe it adds unnecessary write batching and latency spikes. Fix: SET GLOBAL innodb_flush_neighbors=0; persist via my.cnf (or SET PERSIST).", v)
	default:
		return ""
	}
}

// AuditFlushNeighbors renders whether page-flush coalescing matches
// modern flash storage; an SSD-tuned result is stated explicitly.
func (uc *DatabaseUseCase) AuditFlushNeighbors(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := flushNeighborsQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("flush-neighbors introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("flush-neighbors query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing flush-neighbors rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read innodb_flush_neighbors: %w", rerr)
		}
		return "", fmt.Errorf("flush-neighbors query returned no rows")
	}

	var rawv any
	if scanErr := rows.Scan(&rawv); scanErr != nil {
		return "", fmt.Errorf("failed to scan innodb_flush_neighbors: %w", scanErr)
	}
	var raw int64
	switch v := rawv.(type) {
	case int64:
		raw = v
	case []byte:
		raw = parseSettingInt(string(v))
	case string:
		raw = parseSettingInt(v)
	default:
		raw = -2 // unrecognized shape → unreadable verdict
	}
	if verdict := flushNeighborsVerdict(raw); verdict != "" {
		return verdict, nil
	}
	return "Page-flush coalescing disabled — appropriate for SSD/NVMe.", nil
}

// parseSettingInt parses a numeric setting string ("0", "2"),
// returning an out-of-range sentinel when unrecognizable.
func parseSettingInt(s string) int64 {
	s = strings.TrimSpace(s)
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return -2
	}
	return v
}
