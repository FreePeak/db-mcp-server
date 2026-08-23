package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// autovacuum_max_workers audit: the PostgreSQL default of 3 workers is
// shared across every database in the cluster, and each worker costs a
// connection slot — on write-heavy or many-database clusters vacuum
// passes queue up behind each other while dead tuples accumulate
// (feeding the bloat this server's CheckTableBloat reports). The fix is
// a deliberate bump, capped by max_worker_processes.

const avWorkersQuietFloor = 4 // at or above this the cluster has real headroom

// avWorkersProbe returns the probe reading the setting, or "" when
// unsupported.
func avWorkersProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('autovacuum_max_workers') AS workers`
	default:
		return ""
	}
}

// avWorkersVerdict classifies the worker count; healthy values render ""
// so reports stay actionable.
func avWorkersVerdict(workers int) string {
	if workers <= 0 {
		return "autovacuum_max_workers is unreadable on this server — verify with SHOW autovacuum_max_workers."
	}
	if workers >= avWorkersQuietFloor {
		return "" // healthy; the audit adds the explicit clean line
	}
	return fmt.Sprintf("WARNING: autovacuum_max_workers=%d — only %d worker(s) are shared across every database in the cluster, so busy tables queue for vacuum while dead tuples pile up (each worker also holds a connection slot). Fix: ALTER SYSTEM SET autovacuum_max_workers = 5 (or higher for many-database clusters), then restart; raise max_worker_processes to match if needed.",
		workers, workers)
}

// AuditAVWorkers renders whether the cluster has enough vacuum workers;
// a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditAVWorkers(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := avWorkersProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("autovacuum_max_workers introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("autovacuum_max_workers probe failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing autovacuum_max_workers rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		return "", fmt.Errorf("autovacuum_max_workers probe returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan autovacuum_max_workers: %w", scanErr)
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		n = 0 // unparseable renders as unreadable, never a false all-clear
	}

	if verdict := avWorkersVerdict(n); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("autovacuum_max_workers=%d — sufficient vacuum headroom for the cluster.", n), nil
}
