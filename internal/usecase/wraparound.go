package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Transaction-ID wraparound audit: PostgreSQL assigns every write a
// 32-bit XID; when the oldest un-frozen XID approaches ~2 billion the
// engine stops accepting writes and forces an emergency single-user
// vacuum — the most catastrophic silent failure Postgres has, and
// entirely preventable if the age is watched.

// wraparoundQuery returns the per-database frozen-XID-age SELECT, or ""
// when unsupported.
func wraparoundQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT datname,
       age(datfrozenxid) AS xid_age,
       datallowconn
FROM pg_database`
	default:
		return ""
	}
}

const (
	wraparoundWarn     = 200_000_000 // 10% of the ceiling: act soon
	wraparoundCritical = 500_000_000 // aggressive autovacuum territory
)

// wraparoundVerdict classifies one database's XID age.
func wraparoundVerdict(name string, age int64) string {
	switch {
	case age >= wraparoundCritical:
		return fmt.Sprintf("- %s: CRITICAL at %d — autovacuum is losing; freeze aggressively NOW (manual VACUUM FREEZE on hot tables) or writes stop at ~2.1B",
			name, age)
	case age >= wraparoundWarn:
		return fmt.Sprintf("- %s: WARNING at %d — investigate why autovacuum is not keeping up (long-running transactions pinning the horizon?)",
			name, age)
	default:
		return fmt.Sprintf("- %s: healthy at %d", name, age)
	}
}

// CheckWraparoundRisk renders every database's transaction-ID age with
// escalating verdicts, worst first.
func (uc *DatabaseUseCase) CheckWraparoundRisk(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := wraparoundQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("wraparound introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("wraparound catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing wraparound rows: %v", closeErr)
		}
	}()

	type row struct {
		name         string
		age          int64
		allowConn    bool
		allowConnSet bool
	}
	var rowsData []row
	for rows.Next() {
		var r row
		var allow any
		if scanErr := rows.Scan(&r.name, &r.age, &allow); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		if b, ok := allow.(bool); ok {
			r.allowConn, r.allowConnSet = b, true
		}
		rowsData = append(rowsData, r)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate wraparound rows: %w", err)
	}

	sort.Slice(rowsData, func(i, j int) bool { return rowsData[i].age > rowsData[j].age })

	lines := make([]string, 0, len(rowsData))
	for _, r := range rowsData {
		if r.allowConnSet && !r.allowConn {
			continue // template databases are frozen by design
		}
		lines = append(lines, wraparoundVerdict(r.name, r.age))
	}
	if len(lines) == 0 {
		return "No connectable databases found in pg_database.", nil
	}
	out := "Transaction-ID wraparound risk, worst first:\n" + strings.Join(lines, "\n")
	if strings.Contains(lines[0], "CRITICAL") || strings.Contains(lines[0], "WARNING") {
		out += "\nLong-running transactions hold back the freeze horizon — check idle-in-transaction sessions."
	}
	return out, nil
}
