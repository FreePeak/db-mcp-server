package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// wal_compression audit: with the default 'off', every checkpoint
// floods WAL with raw 8KB full-page images — replica shipping lags,
// archive storage balloons, and backup windows all pay. PG14+ offers
// cheap lz4/zstd codecs; pglz works everywhere but costs more CPU.
// The setting is user-context, so the fix needs no restart.

// walCompressionQuery returns the probe for the setting, or "" when
// unsupported.
func walCompressionQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('wal_compression') AS wc`
	default:
		return ""
	}
}

// walCompressionVerdict classifies the setting; any enabled codec
// renders "" so reports stay actionable.
func walCompressionVerdict(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "on", "pglz", "lz4", "zstd":
		return ""
	case "":
		return "wal_compression is empty or unreadable — verify with SHOW wal_compression."
	default:
		return fmt.Sprintf("WARNING: wal_compression=%s — every checkpoint writes full-page images into WAL uncompressed: replicas lag on shipping volume and archives balloon. Fix: ALTER SYSTEM SET wal_compression='lz4' (PG14+; 'zstd' from 15) then SELECT pg_reload_conf(); no restart needed. CPU cost is far below the I/O saved.", v)
	}
}

// AuditWalCompression renders whether checkpoint full-page images are
// compressed; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditWalCompression(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := walCompressionQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("wal_compression introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("wal_compression query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing wal_compression rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read wal_compression: %w", rerr)
		}
		return "", fmt.Errorf("wal_compression query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan wal_compression: %w", scanErr)
	}
	if verdict := walCompressionVerdict(raw); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("WAL compression healthy: wal_compression=%s.", strings.TrimSpace(raw)), nil
}
