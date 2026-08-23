package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Table bloat audit: every UPDATE and DELETE leaves a dead tuple behind;
// VACUUM reclaims them. When dead tuples pile up (autovacuum starved,
// long-running transactions pinning the xmin horizon, update-heavy
// tables), scans read garbage pages, indexes bloat, and disk grows with
// data that no longer exists. pg_stat_user_tables makes the pile-up
// visible before it becomes an outage.

const bloatMinRows = 1000 // below this, ratios are noise

// bloatQuery returns the per-table dead/live tuple SELECT, or "" when
// unsupported.
func bloatQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT relname,
       n_live_tup,
       n_dead_tup
FROM pg_stat_user_tables
ORDER BY n_dead_tup DESC`
	default:
		return ""
	}
}

const (
	bloatWarnRatio     = 0.20 // 1-in-5 rows is garbage
	bloatCriticalRatio = 0.50 // half the table is garbage
)

// bloatVerdict renders one table's bloat line; healthy or tiny tables
// render "" so the report stays actionable.
func bloatVerdict(table string, live, dead int64) string {
	total := live + dead
	if total < bloatMinRows {
		return ""
	}
	ratio := float64(dead) / float64(total)
	switch {
	case ratio >= bloatCriticalRatio:
		return fmt.Sprintf("- %s: CRITICAL — %d/%d rows (%.0f%%) are dead tuples; run VACUUM (or VACUUM FULL off-peak for space reclaim)",
			table, dead, total, ratio*100)
	case ratio >= bloatWarnRatio:
		return fmt.Sprintf("- %s: WARNING — %d/%d rows (%.0f%%) are dead tuples; check autovacuum throughput",
			table, dead, total, ratio*100)
	default:
		return ""
	}
}

type bloatRow struct {
	table string
	live  int64
	dead  int64
}

// renderBloatReport sorts flagged tables worst-first by dead-tuple
// ratio and states a clean result explicitly.
func renderBloatReport(rows []bloatRow) string {
	type flagged struct {
		line  string
		ratio float64
	}
	var lines []flagged
	for _, r := range rows {
		if v := bloatVerdict(r.table, r.live, r.dead); v != "" {
			ratio := 0.0
			if total := r.live + r.dead; total > 0 {
				ratio = float64(r.dead) / float64(total)
			}
			lines = append(lines, flagged{line: v, ratio: ratio})
		}
	}
	if len(lines) == 0 {
		return fmt.Sprintf("No significant bloat across %d table(s): all under %.0f%% dead tuples.",
			len(rows), bloatWarnRatio*100)
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].ratio > lines[j].ratio })
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.line
	}
	return fmt.Sprintf("%d of %d table(s) carry significant dead-tuple bloat:\n%s\n"+
		"Persistent bloat means autovacuum is not keeping up — long-running transactions pinning the horizon are the usual cause.",
		len(lines), len(rows), strings.Join(out, "\n"))
}

// CheckTableBloat renders every user table whose dead-tuple ratio is
// significant, worst first.
func (uc *DatabaseUseCase) CheckTableBloat(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := bloatQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("table-bloat introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("bloat catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing bloat rows: %v", closeErr)
		}
	}()

	var stats []bloatRow
	for rows.Next() {
		var table string
		var live, dead int64
		if scanErr := rows.Scan(&table, &live, &dead); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		stats = append(stats, bloatRow{table: table, live: live, dead: dead})
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate bloat rows: %w", err)
	}
	return renderBloatReport(stats), nil
}
