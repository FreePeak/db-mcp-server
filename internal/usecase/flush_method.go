package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// innodb_flush_method audit: on Linux the default (empty = fsync)
// double-buffers every write — data lands in both the InnoDB buffer
// pool and the OS page cache, wasting RAM and adding checkpoint
// stalls. O_DIRECT bypasses the page cache; O_DIRECT_NO_FSYNC is its
// variant. This is a classic forgotten tuning: servers migrated from
// defaults keep paying the double-buffer tax forever. Windows is
// unaffected (the setting only matters there as unbuffered), so the
// warning phrases itself as Linux-specific.

// flushMethodQuery returns the probe for the setting, or "" when
// unsupported.
func flushMethodQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT @@GLOBAL.innodb_flush_method AS method`
	default:
		return ""
	}
}

// flushMethodVerdict classifies the method; direct I/O renders "" so
// reports stay actionable.
func flushMethodVerdict(method string) string {
	m := strings.ToUpper(strings.TrimSpace(method))
	switch m {
	case "O_DIRECT", "O_DIRECT_NO_FSYNC":
		return ""
	default:
		name := "fsync (default)"
		if m != "" {
			name = m
		}
		return fmt.Sprintf("WARNING: innodb_flush_method=%s — on Linux this double-buffers writes into both the buffer pool and the OS page cache, wasting memory and stalling checkpoints under load. Fix: set innodb_flush_method=O_DIRECT in my.cnf and restart (verify free memory rises after). No effect needed on Windows.", name)
	}
}

// AuditFlushMethod renders whether InnoDB writes bypass the OS page
// cache; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditFlushMethod(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := flushMethodQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("flush-method introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("flush-method query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing flush-method rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read flush method: %w", rerr)
		}
		return "", fmt.Errorf("flush-method query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan flush method: %w", scanErr)
	}
	if verdict := flushMethodVerdict(raw); verdict != "" {
		return verdict, nil
	}
	return "Write path healthy: innodb_flush_method uses direct I/O.", nil
}
