package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// binlog_format audit: STATEMENT-based replication silently diverges on
// non-deterministic functions (NOW(), UUID(), LIMIT without ORDER BY);
// MIXED still falls back to statement format for many shapes. ROW is
// the MySQL default since 5.7.7 and required by most managed platforms.
// Drift between primary and replica is invisible until a failover
// serves wrong data — one of the nastiest failure modes to discover.

// binlogFormatQuery returns the probe for the global setting, or ""
// when unsupported.
func binlogFormatQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT @@GLOBAL.binlog_format AS binlog_format`
	default:
		return ""
	}
}

// binlogFormatVerdict classifies the format; ROW renders "" so reports
// stay actionable.
func binlogFormatVerdict(format string) string {
	switch strings.ToUpper(strings.TrimSpace(format)) {
	case "ROW":
		return ""
	case "":
		return "binlog_format is empty or unreadable — verify with SHOW GLOBAL VARIABLES LIKE 'binlog_format'."
	case "STATEMENT":
		return "WARNING: binlog_format=STATEMENT — non-deterministic statements (NOW(), UUID(), LIMIT without ORDER BY) can make replicas silently diverge from the primary. Fix: SET GLOBAL binlog_format='ROW' (then restart existing replica IO/SQL threads)."
	default: // MIXED and anything unrecognized
		return fmt.Sprintf("WARNING: binlog_format=%s — MIXED still routes many statement shapes through unsafe STATEMENT logging. Fix: SET GLOBAL binlog_format='ROW' for deterministic replication.", format)
	}
}

// AuditBinlogFormat renders whether replication uses row-based
// logging; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditBinlogFormat(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := binlogFormatQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("binlog_format introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("binlog_format query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing binlog_format rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read binlog_format: %w", rerr)
		}
		return "", fmt.Errorf("binlog_format query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan binlog_format: %w", scanErr)
	}
	if verdict := binlogFormatVerdict(raw); verdict != "" {
		return verdict, nil
	}
	return "Replication format healthy: binlog_format=ROW.", nil
}
