package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// sync_binlog durability audit: 0 disables fsync of the binary log —
// the OS page cache holds unflushed transactions, and a crash can
// lose commits replicas already received, breaking failover
// consistency. Values >0 group-commit every N binlogs: a deliberate,
// bounded tradeoff we don't second-guess; only full disablement is
// flagged.

// syncBinlogProbe returns the probe for the setting, or "" when
// unsupported.
func syncBinlogProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT @@GLOBAL.sync_binlog AS sb`
	default:
		return ""
	}
}

// syncBinlogVerdict classifies the setting; durable or deliberate
// group-commit values render "" so reports stay actionable.
func syncBinlogVerdict(v int64) string {
	switch {
	case v < 0:
		return "sync_binlog is unreadable — verify with SHOW GLOBAL VARIABLES LIKE 'sync_binlog'."
	case v == 0:
		return "WARNING: sync_binlog=0 — binary-log fsync is disabled: a crash can lose committed transactions that replicas already received, silently diverging failover targets. Fix: SET GLOBAL sync_binlog=1 (apply live) and persist in my.cnf. If write latency forces a tradeoff, use a small N (e.g. 100, bounds loss to N binlogs) rather than disabling entirely."
	default:
		return "" // durable (=1) or bounded group commit (>1)
	}
}

// AuditSyncBinlog renders whether the binary log survives crashes; a
// safe result is stated explicitly.
func (uc *DatabaseUseCase) AuditSyncBinlog(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := syncBinlogProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("sync_binlog introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("sync_binlog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing sync_binlog rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read sync_binlog: %w", rerr)
		}
		return "", fmt.Errorf("sync_binlog query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan sync_binlog: %w", scanErr)
	}
	var v int64
	if _, perr := fmt.Sscanf(strings.TrimSpace(raw), "%d", &v); perr != nil {
		v = -1 // unparseable renders as unreadable, never guessed at
	}
	if verdict := syncBinlogVerdict(v); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("Binlog durability healthy: sync_binlog=%s.", strings.TrimSpace(raw)), nil
}
