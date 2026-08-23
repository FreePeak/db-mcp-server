package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// synchronous_commit audit: PostgreSQL's counterpart to MySQL's
// innodb_flush_log_at_trx_commit. With synchronous_commit=off a COMMIT
// is acknowledged before WAL is flushed — an OS crash can lose
// recently committed transactions (roughly wal_writer_delay×3 of
// work). Frequently flipped off for write-heavy batch jobs and never
// turned back on; the loss only shows up after the first crash.

// syncCommitQuery returns the probe for the current setting, or ""
// when unsupported.
func syncCommitQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT COALESCE(current_setting('synchronous_commit'), 'on') AS mode`
	default:
		return ""
	}
}

// syncCommitVerdict classifies the setting against commit-loss risk;
// the fully-durable default ("on") renders "" so reports stay
// actionable.
func syncCommitVerdict(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "":
		return "synchronous_commit is unreadable on this server — verify with SHOW synchronous_commit."
	case "off":
		return "WARNING: synchronous_commit=off — COMMITs are acknowledged before WAL is flushed, so an OS crash means recently committed transactions can be lost (~wal_writer_delay×3 of work). Fix: ALTER SYSTEM SET synchronous_commit = 'on' (or SET per session for batch jobs that accept the tradeoff)."
	case "on":
		return "" // fully-durable default; the audit adds the explicit clean line
	default:
		return fmt.Sprintf("synchronous_commit=%s: commits are durable against local crashes%s.", strings.TrimSpace(mode), durabilityNote(mode))
	}
}

// durabilityNote adds standby context where relevant.
func durabilityNote(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "local":
		return " (local flush only — no standby guarantee)"
	case "remote_write":
		return " (standby receives WAL but may not have flushed it)"
	default:
		return "" // on / remote_apply are the strongest settings
	}
}

// AuditSyncCommit renders whether acknowledged commits survive a
// crash; the healthy default is stated explicitly.
func (uc *DatabaseUseCase) AuditSyncCommit(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := syncCommitQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("synchronous_commit introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("synchronous_commit query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing synchronous-commit rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read synchronous_commit: %w", rerr)
		}
		return "", fmt.Errorf("synchronous_commit query returned no rows")
	}

	var mode string
	if scanErr := rows.Scan(&mode); scanErr != nil {
		return "", fmt.Errorf("failed to scan synchronous_commit: %w", scanErr)
	}
	if verdict := syncCommitVerdict(mode); verdict != "" {
		return verdict, nil
	}
	return "synchronous_commit=on: acknowledged commits survive crashes (PostgreSQL default).", nil
}
