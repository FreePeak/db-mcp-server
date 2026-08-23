package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Temp-spill detection: sorts and hash joins that exceed the engine's
// memory budget spill to disk — the classic "queries feel slow but CPU
// is idle" signal of undersized work_mem / tmp_table_size. Cumulative
// counters from the engine catalogs make the problem visible before
// anyone profiles a single query.

// tempSpillQuery returns the engine's temp-spill counters SELECT
// (disk spills, total internal tmp tables where applicable), or "" when
// unsupported.
func tempSpillQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT temp_files,
       COALESCE(pg_size_pretty(temp_bytes), '0 bytes') AS spilled
FROM pg_stat_database
WHERE datname = current_database()`
	case "mysql", "mariadb":
		return `SELECT
  (SELECT VARIABLE_VALUE FROM performance_schema.global_status
    WHERE VARIABLE_NAME = 'Created_tmp_disk_tables') AS disk_tables,
  (SELECT VARIABLE_VALUE FROM performance_schema.global_status
    WHERE VARIABLE_NAME = 'Created_tmp_tables') AS total_tables`
	default:
		return ""
	}
}

// CheckTempSpills renders cumulative disk-spill counters with a verdict:
// for MySQL a high on-disk ratio among internal temp tables points at
// undersized tmp_table_size/max_heap_table_size; Postgres reports raw
// volume since stats reset.
func (uc *DatabaseUseCase) CheckTempSpills(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := tempSpillQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("temp-spill introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("temp-spill catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing temp-spill rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		return "", fmt.Errorf("temp-spill catalog returned no rows")
	}

	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		var files int64
		var pretty string
		if scanErr := rows.Scan(&files, &pretty); scanErr != nil {
			return "", fmt.Errorf("failed to scan spill row: %w", scanErr)
		}
		if files == 0 {
			return "No queries have spilled to disk (0 temp files).", nil
		}
		return fmt.Sprintf("%d temp file(s) totaling %s spilled to disk since stats reset — consider raising work_mem for sort-heavy workloads.", files, pretty), nil
	default: // mysql/mariadb
		var rawDisk, rawTotal any
		if scanErr := rows.Scan(&rawDisk, &rawTotal); scanErr != nil {
			return "", fmt.Errorf("failed to scan spill row: %w", scanErr)
		}
		disk, derr := toInt(rawDisk)
		total, terr := toInt(rawTotal)
		if derr != nil || terr != nil || total < 0 {
			return "", fmt.Errorf("unparseable spill values (disk=%v total=%v)", rawDisk, rawTotal)
		}
		if total == 0 {
			return "No internal temp tables created yet — nothing has spilled.", nil
		}
		pct := disk * 100 / total
		if pct >= 20 {
			return fmt.Sprintf("%d of %d internal temp table(s) hit disk (%d%%) — consider raising tmp_table_size/max_heap_table_size.", disk, total, pct), nil
		}
		return fmt.Sprintf("Temp-table spill ratio healthy: %d of %d hit disk (%d%%).", disk, total, pct), nil
	}
}
