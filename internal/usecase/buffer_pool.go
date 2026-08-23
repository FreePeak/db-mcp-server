package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// innodb_buffer_pool_size sizing audit: the health tool reports the
// cache hit *ratio*, but a low ratio is the symptom — this audit
// compares the configured pool against total data+index bytes so an
// undersized pool is visible before hit rates degrade. When the pool
// holds only a small fraction of the data, every cold read hits disk;
// the classic fix is sizing it to ~60% of host RAM (dynamically settable
// since MySQL 5.7).

const bufferPoolWarnRatio = 25 // % of data+indexes the pool must hold

// bufferPoolQuery returns pool size and user-data volume in one round
// trip, or "" when unsupported.
func bufferPoolQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT 'pool' AS k, @@GLOBAL.innodb_buffer_pool_size AS v
UNION ALL
SELECT 'data', COALESCE(SUM(data_length + index_length), 0)
FROM information_schema.TABLES
WHERE table_schema NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')`
	default:
		return ""
	}
}

// bufferPoolVerdict classifies the pool against the data volume; a
// pool that can hold everything renders "" so reports stay actionable.
func bufferPoolVerdict(poolBytes, dataBytes int64) string {
	if poolBytes <= 0 || dataBytes <= 0 {
		return ""
	}
	pct := 100 * poolBytes / dataBytes
	if pct >= bufferPoolWarnRatio {
		return ""
	}
	return fmt.Sprintf("WARNING: innodb_buffer_pool_size=%s holds only ~%d%% of your %s in data+indexes — cold reads hit disk and the hit ratio will suffer. Fix: size to ~60%% of host RAM, e.g. SET GLOBAL innodb_buffer_pool_size=4294967296 (dynamic on MySQL 5.7+).",
		humanBytes(int64(poolBytes)), pct, humanBytes(dataBytes))
}

// AuditBufferPool renders whether the buffer pool plausibly fits the
// working set; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditBufferPool(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := bufferPoolQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("innodb_buffer_pool_size introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("buffer-pool query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing buffer-pool rows: %v", closeErr)
		}
	}()

	var pool, data int64
	parse := func(s string) int64 {
		n, perr := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if perr != nil {
			logger.Error("unparseable buffer-pool value %q: %v", s, perr)
			return 0
		}
		return n
	}
	for rows.Next() {
		var k, v string
		if scanErr := rows.Scan(&k, &v); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		switch strings.TrimSpace(k) {
		case "pool":
			pool = parse(v)
		case "data":
			data = parse(v)
		}
	}
	if rerr := rows.Err(); rerr != nil {
		return "", fmt.Errorf("failed to iterate buffer-pool rows: %w", rerr)
	}
	if verdict := bufferPoolVerdict(pool, data); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("Buffer pool healthy: %s pool vs %s data+indexes (≥%d%% coverage).",
		humanBytes(int64(pool)), humanBytes(data), bufferPoolWarnRatio), nil
}
