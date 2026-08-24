package usecase

import (
	"context"
	"fmt"
	"strings"
)

// DbHealth is the unified maintenance report: everything index_health
// covers (structure, usage evidence, bloat) plus connection pressure. It
// is our counterpart of Postgres MCP Pro's analyze_db_health summary view.
func (uc *DatabaseUseCase) DbHealth(ctx context.Context, dbID string) (string, error) {
	out, err := uc.IndexHealth(ctx, dbID)
	if err != nil {
		return "", err
	}

	conn := uc.connectionReport(ctx, dbID)
	if conn == "" {
		return out, nil
	}
	var b strings.Builder
	b.WriteString(out)
	b.WriteString("\n" + conn)
	return b.String(), nil
}

// connPressureThreshold is the fraction of max_connections at which
// utilization becomes a warning rather than an observation.
const connPressureThreshold = 0.80

// connectionReport renders a connections section from engine catalogs;
// empty string when the engine exposes none or every read fails.
func (uc *DatabaseUseCase) connectionReport(ctx context.Context, dbID string) string {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return ""
	}
	switch strings.ToLower(dbType) {
	case "postgres", "timescale", "timescaledb":
		rows, qerr := uc.queryTableMetadata(ctx, dbID, []string{`
			SELECT
				count(*) FILTER (WHERE state = 'active')::float8 AS active,
				count(*)::float8 AS total,
				current_setting('max_connections')::float8 AS max_connections
			FROM pg_stat_activity WHERE datname = current_database()`})
		if qerr != nil || len(rows) == 0 {
			return ""
		}
		return formatConnectionReport(
			rowInt(rows[0], "active"),
			rowInt(rows[0], "total"),
			rowInt(rows[0], "max_connections"))
	case "mysql":
		connRows, cerr := uc.queryTableMetadata(ctx, dbID, []string{
			"SELECT CAST(variable_value AS UNSIGNED) AS connected FROM performance_schema.global_status WHERE variable_name = 'THREADS_CONNECTED'",
		})
		maxRows, merr := uc.queryTableMetadata(ctx, dbID, []string{
			"SELECT CAST(variable_value AS UNSIGNED) AS max_connections FROM performance_schema.global_variables WHERE variable_name = 'MAX_CONNECTIONS'",
		})
		if cerr != nil || merr != nil || len(connRows) == 0 || len(maxRows) == 0 {
			return ""
		}
		return formatConnectionReport(
			rowInt(connRows[0], "connected"),
			rowInt(connRows[0], "connected"), // MySQL has no active/open split here; threads_connected is the pool view
			rowInt(maxRows[0], "max_connections"))
	default:
		return "" // SQLite and unknown engines: no connection catalogs
	}
}

// formatConnectionReport renders utilization and warns past the pressure
// threshold. Pure so threshold math stays unit-testable without engines.
func formatConnectionReport(active, total, maxConns int64) string {
	if maxConns <= 0 {
		return ""
	}
	pct := float64(total) / float64(maxConns) * 100
	var b strings.Builder
	fmt.Fprintf(&b, "Connections: %d active of %d open (%.0f%% of %d max)", active, total, pct, maxConns)
	if pct >= connPressureThreshold*100 {
		fmt.Fprintf(&b, "\n  WARNING: connection pool is above %.0f%% capacity — investigate idle-in-transaction sessions before adding capacity.", connPressureThreshold*100)
	}
	b.WriteString("\n")
	return b.String()
}
