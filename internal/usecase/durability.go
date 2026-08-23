package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Durability audit: innodb_flush_log_at_trx_commit=1 flushes the redo
// log at every commit — the ACID default. Values 2 and 0 defer that
// flush, so an OS crash or MySQL crash can lose up to ~1 second of
// COMMITTED transactions. Often set by benchmark-chasing; rarely
// revisited. A silent durability downgrade nothing else reports.

// flushLogQuery returns the probe for the redo-log flush mode, or ""
// when unsupported.
func flushLogQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT COALESCE(@@GLOBAL.innodb_flush_log_at_trx_commit, 1) AS flush_mode`
	default:
		return ""
	}
}

// flushLogVerdict classifies the flush mode against crash-durability
// expectations.
func flushLogVerdict(mode int64) string {
	switch mode {
	case 1:
		return "Durability healthy: redo log flushes on every commit (innodb_flush_log_at_trx_commit=1)."
	case 2:
		return fmt.Sprintf("WARNING: innodb_flush_log_at_trx_commit=2 — an OS crash can lose up to ~1s of committed transactions. SET GLOBAL innodb_flush_log_at_trx_commit=1 unless the risk is explicitly accepted.")
	case 0:
		return fmt.Sprintf("WARNING: innodb_flush_log_at_trx_commit=0 — a MySQL crash can lose up to ~1s of committed transactions (worse than mode 2). SET GLOBAL innodb_flush_log_at_trx_commit=1.")
	default:
		return fmt.Sprintf("innodb_flush_log_at_trx_commit=%d — nonstandard value; verify intended durability semantics.", mode)
	}
}

// AuditDurability renders whether committed transactions survive a
// crash; a durable result is stated explicitly.
func (uc *DatabaseUseCase) AuditDurability(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := flushLogQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("durability introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("durability settings query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing durability rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read durability settings: %w", rerr)
		}
		return "", fmt.Errorf("durability query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan durability setting: %w", scanErr)
	}
	mode, perr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if perr != nil {
		logger.Error("unparseable flush mode %q: %v", raw, perr)
		return flushLogVerdict(-1), nil
	}
	return flushLogVerdict(mode), nil
}
