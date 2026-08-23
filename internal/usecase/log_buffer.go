package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// innodb_log_buffer_size audit: transactions buffer their WAL in the
// log buffer and flush at commit. When a transaction's writes exceed
// the buffer (16MB default), InnoDB must flush mid-transaction —
// visible as Innodb_log_waits, the engine's own evidence that the
// buffer is too small for the workload.

// logBufferProbe returns the setting + wait-counter SELECT, or ""
// when unsupported.
func logBufferProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT COALESCE(@@GLOBAL.innodb_log_buffer_size, 0) AS buf_bytes,
(SELECT COALESCE(MAX(VARIABLE_VALUE), '0') FROM performance_schema.global_status WHERE VARIABLE_NAME = 'Innodb_log_waits') AS waits`
	default:
		return ""
	}
}

// logBufferVerdict classifies buffer size against observed waits;
// healthy configs render "" so reports stay actionable. sizeBytes=0
// means unreadable; negative values read as unparseable evidence.
func logBufferVerdict(sizeBytes, waits int64) string {
	if sizeBytes <= 0 || waits < 0 {
		return "innodb_log_buffer_size / Innodb_log_waits unreadable — verify with SHOW GLOBAL VARIABLES LIKE 'innodb_log_buffer_size'; SHOW GLOBAL STATUS LIKE 'Innodb_log_waits'."
	}
	switch {
	case waits > 0:
		return fmt.Sprintf("WARNING: Innodb_log_waits=%d with innodb_log_buffer_size=%s — the log buffer overflowed %d time(s) since restart, forcing mid-transaction WAL flushes under write bursts. Fix if large/bursty transactions are common: SET GLOBAL innodb_log_buffer_size='64M' (dynamic on MySQL 8.0; restart required before 8.0); persist via my.cnf or SET PERSIST.",
			waits, humanBytes(sizeBytes), waits)
	case sizeBytes < 16*1024*1024:
		return fmt.Sprintf("innodb_log_buffer_size=%s is below the 16M default with no recorded waits yet — fine until large transactions appear.",
			humanBytes(sizeBytes))
	default:
		return "" // adequately sized, no evidence of spills
	}
}

// AuditLogBuffer renders whether the WAL log buffer has been too
// small for the workload; a clean result is stated explicitly.
func (uc *DatabaseUseCase) AuditLogBuffer(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := logBufferProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("log-buffer introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("log-buffer query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing log-buffer rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read log-buffer counters: %w", rerr)
		}
		return "", fmt.Errorf("log-buffer query returned no rows")
	}

	var sizeRaw, waitsRaw string
	if scanErr := rows.Scan(&sizeRaw, &waitsRaw); scanErr != nil {
		return "", fmt.Errorf("failed to scan log-buffer counters: %w", scanErr)
	}
	size := parseSettingInt(strings.TrimSpace(sizeRaw))
	waits := parseSettingInt(strings.TrimSpace(waitsRaw))
	if verdict := logBufferVerdict(size, waits); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("Log buffer healthy: innodb_log_buffer_size=%s, no recorded waits.",
		humanBytes(size)), nil
}
