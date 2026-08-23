package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// binlog_row_image audit: with row-based replication (the modern
// default) the FULL setting logs complete before+after row images on
// every UPDATE/DELETE even when one column changed — extra disk,
// I/O, and replica network proportional to table width. MINIMAL logs
// only the identifying columns plus what changed; NOBLOB is the
// middle ground that skips unchanged BLOBs.

// binlogRowImageQuery returns the probe for the setting, or "" when
// unsupported.
func binlogRowImageQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT COALESCE(@@GLOBAL.binlog_row_image, '')`
	default:
		return ""
	}
}

// binlogRowImageVerdict classifies the setting; MINIMAL and NOBLOB
// render "" so reports stay actionable.
func binlogRowImageVerdict(v string) string {
	s := strings.ToUpper(strings.TrimSpace(v))
	switch s {
	case "":
		return "binlog_row_image is empty or unreadable — verify with SHOW GLOBAL VARIABLES LIKE 'binlog_row_image'."
	case "FULL":
		return "WARNING: binlog_row_image=FULL — every UPDATE/DELETE writes full before+after row images to the binary log regardless of how few columns changed: extra disk, I/O, and replica network. Fix if replicas don't need full images: SET GLOBAL binlog_row_image='MINIMAL' (dynamic); persist via my.cnf or SET PERSIST. Caveat: flashback/audit tooling that reconstructs old rows requires FULL."
	default:
		return "" // MINIMAL / NOBLOB are sane
	}
}

// AuditBinlogRowImage renders whether binary-log row images are
// wider than needed; a lean result is stated explicitly.
func (uc *DatabaseUseCase) AuditBinlogRowImage(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := binlogRowImageQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("binlog_row_image introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("binlog_row_image query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing binlog-row-image rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read binlog_row_image: %w", rerr)
		}
		return "", fmt.Errorf("binlog_row_image query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan binlog_row_image: %w", scanErr)
	}
	if verdict := binlogRowImageVerdict(raw); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("Binary-log row images lean (%s).", strings.TrimSpace(raw)), nil
}
