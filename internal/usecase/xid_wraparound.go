package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Transaction-ID wraparound audit: PostgreSQL assigns every transaction
// a 32-bit XID; unfrozen rows older than ~2^31 force the engine to stop
// accepting writes to avoid wraparound corruption. Anti-wraparound
// vacuum prevents this, but when it falls behind the failure is total
// and unannounced until the database refuses connections. One catalog
// read reports how close each database is.

// xidWarnAge matches PostgreSQL's default autovacuum_freeze_max_age:
// past this point the engine itself has started aggressive
// anti-wraparound vacuums, so operators should know about it.
const xidWarnAge = 200_000_000

// xidCriticalAge is halfway to the hard 2^31 shutdown limit — enough
// headroom that a planned intervention still fits, but not something
// to discover by accident.
const xidCriticalAge = 1_000_000_000

// xidWraparoundQuery returns the per-database frozen-XID-age SELECT,
// or "" when unsupported.
func xidWraparoundQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT datname, age(datfrozenxid)::bigint AS xid_age
FROM pg_database
WHERE datallowconn
ORDER BY xid_age DESC`
	default:
		return ""
	}
}

// xidVerdict renders one database's risk line; healthy ages render ""
// so the report stays actionable.
func xidVerdict(datname string, age int64) string {
	switch {
	case age >= xidCriticalAge:
		return fmt.Sprintf("- %s: CRITICAL %d transactions old — over halfway to the 2^31 wraparound limit; "+
			"verify anti-wraparound vacuum progress NOW", datname, age)
	case age >= xidWarnAge:
		return fmt.Sprintf("- %s: WARNING %d transactions old (>= autovacuum_freeze_max_age) — "+
			"aggressive vacuums should be running; watch their progress", datname, age)
	default:
		return ""
	}
}

// AuditXIDWraparound renders every database's transaction-ID age against
// the wraparound thresholds; a clean result is stated explicitly.
func (uc *DatabaseUseCase) AuditXIDWraparound(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := xidWraparoundQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("transaction-ID introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("XID-age catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing xid-wraparound rows: %v", closeErr)
		}
	}()

	var lines []string
	databases := 0
	for rows.Next() {
		var datname string
		var age int64
		if scanErr := rows.Scan(&datname, &age); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		databases++
		if line := xidVerdict(datname, age); line != "" {
			lines = append(lines, line)
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate xid-age rows: %w", err)
	}
	if len(lines) == 0 {
		return fmt.Sprintf("No transaction-ID wraparound risk across %d database(s): all under %d XIDs old.",
			databases, xidWarnAge), nil
	}
	return fmt.Sprintf("%d of %d database(s) approaching XID wraparound:\n%s",
		len(lines), databases, strings.Join(lines, "\n")), nil
}
