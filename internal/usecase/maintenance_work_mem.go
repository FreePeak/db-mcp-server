package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// maintenance_work_mem audit: the 64MB default makes VACUUM,
// CREATE INDEX, and ANALYZE grind on large tables — index builds and
// dead-tuple scans spill to temp disk once the working set exceeds
// the budget. Unlike work_mem this is per maintenance operation, not
// per sort node, so raising it is low-risk on typical servers.

// maintenanceWorkMemProbe returns the probe reading the setting, or
// "" when unsupported.
func maintenanceWorkMemProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('maintenance_work_mem') AS mwm`
	default:
		return ""
	}
}

// maintenanceWorkMemVerdict classifies the parsed byte value; at or
// above 256MB renders "" so reports stay actionable.
func maintenanceWorkMemVerdict(bytes int64) string {
	if bytes <= 0 {
		return "maintenance_work_mem is empty or unreadable — verify with SHOW maintenance_work_mem."
	}
	if bytes >= 256*1024*1024 {
		return ""
	}
	return fmt.Sprintf(
		"WARNING: maintenance_work_mem=%s — VACUUM, CREATE INDEX, and ANALYZE spill to temp disk on large tables, stretching maintenance windows. Fix: ALTER SYSTEM SET maintenance_work_mem = '256MB' then SELECT pg_reload_conf(); it is per-operation, not per-connection, so the raise is safe.",
		humanBytes(bytes))
}

// AuditMaintenanceWorkMem renders whether maintenance operations have
// a sane memory budget; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditMaintenanceWorkMem(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := maintenanceWorkMemProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("maintenance_work_mem introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("maintenance_work_mem query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing maintenance-work-mem rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read maintenance_work_mem: %w", rerr)
		}
		return "", fmt.Errorf("maintenance_work_mem query returned no rows")
	}

	var raw interface{}
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan maintenance_work_mem: %w", scanErr)
	}
	bytes := parsePrettySize(strings.TrimSpace(fmt.Sprintf("%v", raw)))
	if verdict := maintenanceWorkMemVerdict(bytes); verdict != "" {
		return verdict, nil
	}
	return "maintenance_work_mem healthy: " + humanBytes(bytes) + " — maintenance operations keep their working set in memory.", nil
}
