package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Binary-log growth audit: binlogs are what makes point-in-time
// recovery and replication possible — and, with retention disabled,
// what silently fills the disk. binlog_expire_logs_seconds=0 means
// files accumulate until something breaks. Nothing else on the tool
// surface reports their size or expiry.

// binlogRetentionQuery returns the probe for binlog_expire_logs_seconds,
// or "" when unsupported.
func binlogRetentionQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT COALESCE(@@GLOBAL.binlog_expire_logs_seconds, 0) AS expire_secs`
	default:
		return ""
	}
}

// binlogVerdict classifies retention against the binlog inventory;
// zero retention is always a warning regardless of current size.
func binlogVerdict(files int, expireSecs int64, totalBytes uint64) string {
	if files == 0 {
		return "No binary log file present — logging is disabled or just rotated, so replication and point-in-time recovery have nothing to work from; verify log_bin before relying on either."
	}
	if expireSecs <= 0 {
		return fmt.Sprintf("WARNING: %d binary log file(s) totalling %s never expire — retention is unset or unreadable (possibly logging recently disabled, leaving a rotation-lag file). Set binlog_expire_logs_seconds (e.g. 604800 for 7 days) before the disk fills.",
			files, humanBytes(int64(totalBytes)))
	}
	return fmt.Sprintf("Binary logs healthy: %d file(s), %s retained, expiring after %d day(s).",
		files, humanBytes(int64(totalBytes)), expireSecs/86400)
}

// AuditBinaryLogs renders total binary-log size against the configured
// retention so disk-fill risk is visible before it becomes an outage.
func (uc *DatabaseUseCase) AuditBinaryLogs(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	retentionQ := binlogRetentionQuery(dbType)
	if retentionQ == "" {
		return "", fmt.Errorf("binary-log introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	var expireSecs int64
	rows, err := db.Query(ctx, retentionQ)
	if err != nil {
		return "", fmt.Errorf("retention probe failed (binary logging may be off): %w", err)
	}
	if rows.Next() {
		if scanErr := rows.Scan(&expireSecs); scanErr != nil {
			expireSecs = 0
		}
	}
	if cerr := rows.Close(); cerr != nil {
		logger.Error("error closing retention rows: %v", cerr)
	}

	logRows, err := db.Query(ctx, "SHOW BINARY LOGS")
	if err != nil {
		return "", fmt.Errorf("SHOW BINARY LOGS failed: %w", err)
	}
	defer func() {
		if closeErr := logRows.Close(); closeErr != nil {
			logger.Error("error closing binlog rows: %v", closeErr)
		}
	}()

	files := 0
	var total uint64
	for logRows.Next() {
		cols, colErr := logRows.Columns()
		if colErr != nil {
			continue // unidentifiable columns: can't locate File_size, skip row
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(vals))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if scanErr := logRows.Scan(ptrs...); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		for i, name := range cols {
			if !strings.EqualFold(name, "File_size") {
				continue
			}
			switch v := vals[i].(type) {
			case int64:
				total += uint64(v)
				files++
			case []byte:
				var n uint64
				if _, perr := fmt.Sscanf(string(v), "%d", &n); perr == nil {
					total += n
					files++
				}
			}
		}
	}
	if rerr := logRows.Err(); rerr != nil {
		return "", fmt.Errorf("failed to iterate binlog rows: %w", rerr)
	}

	return binlogVerdict(files, expireSecs, total), nil
}
