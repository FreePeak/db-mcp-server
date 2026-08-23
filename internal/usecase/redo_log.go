package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Redo-log sizing audit: an undersized redo log forces aggressive
// checkpointing — every write burst re-dirties pages the flusher just
// wrote, multiplying physical I/O. The historical default was ~48MB×2
// files (5.x era); servers migrated forward keep paying it. MySQL
// 8.0.30+ replaced innodb_log_file_size with innodb_redo_log_capacity,
// which is SET GLOBAL-able live; older versions need a restart.

const redoLogFloorBytes = 512 << 20 // 512 MiB: below this, checkpointing dominates writes

// redoLogQueries returns candidate SELECTs in preference order:
// modern capacity first, then legacy file-size math.
func redoLogQueries(dbType string) []string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return []string{
			`SELECT @@GLOBAL.innodb_redo_log_capacity AS capacity`,
			`SELECT @@GLOBAL.innodb_log_file_size * @@GLOBAL.innodb_log_files_in_group AS capacity`,
		}
	default:
		return nil
	}
}

// redoLogVerdict classifies total redo capacity; comfortable sizes
// render "" so reports stay actionable.
func redoLogVerdict(capacity int64) string {
	if capacity <= 0 {
		return "Redo-log capacity is 0 or unreadable — check server configuration."
	}
	if capacity < redoLogFloorBytes {
		return fmt.Sprintf("WARNING: redo log totals %d MB — undersized logs force aggressive checkpointing and write amplification under load. Fix on MySQL 8.0.30+: SET GLOBAL innodb_redo_log_capacity='2G' (applies live); older versions need innodb_log_file_size in my.cnf plus a restart.",
			capacity/(1<<20))
	}
	return ""
}

// AuditRedoLog renders whether the redo log plausibly matches write
// volume; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditRedoLog(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	probes := redoLogQueries(dbType)
	if len(probes) == 0 {
		return "", fmt.Errorf("redo-log introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	var capacity int64
	for _, q := range probes {
		rows, err := db.Query(ctx, q)
		if err != nil {
			continue // e.g. pre-8.0.30 server without the capacity variable
		}
		if rows.Next() {
			var raw string
			scanErr := rows.Scan(&raw)
			if cerr := rows.Close(); cerr != nil {
				logger.Error("error closing redo-log rows: %v", cerr)
			}
			if scanErr != nil {
				continue
			}
			n, perr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
			if perr == nil && n > 0 {
				capacity = n
				break
			}
		} else if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing redo-log rows: %v", cerr)
		}
	}

	if verdict := redoLogVerdict(capacity); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("Redo log healthy: capacity %s.", humanBytes(capacity)), nil
}
