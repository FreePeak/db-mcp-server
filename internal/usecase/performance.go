package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FreePeak/db-mcp-server/pkg/dbtools"
)

// AnalyzePerformance implements the performance_* tools against the real
// query-tracking analyzer. Actions:
//   - "stats": aggregated metrics per normalized query text
//   - "slow_queries": recorded slow queries above the configured threshold
//   - "suggest": static SQL issue detection for a given query
//   - "reset": clears collected history
func (uc *DatabaseUseCase) AnalyzePerformance(ctx context.Context, dbID, action, query string, limit, thresholdMs int) (string, error) {
	analyzer := dbtools.GetPerformanceAnalyzer()

	if thresholdMs > 0 {
		analyzer.SetSlowThreshold(time.Duration(thresholdMs) * time.Millisecond)
	}

	switch action {
	case "stats":
		return formatQueryMetrics(analyzer), nil
	case "slow_queries":
		if limit <= 0 {
			limit = 10
		}
		return formatSlowQueries(analyzer.SlowQueries(), limit), nil
	case "engine_slow_queries":
		if limit <= 0 {
			limit = 10
		}
		return uc.engineSlowQueries(ctx, dbID, limit)
	case "suggest_indexes":
		return uc.SuggestIndexes(ctx, dbID, query)
	case "suggest":
		if strings.TrimSpace(query) == "" {
			return "", fmt.Errorf("query parameter is required for suggest action")
		}
		issues := dbtools.NewSQLIssueDetector().DetectIssues(query)
		return formatSuggestions(query, issues), nil
	case "reset":
		analyzer.Reset()
		return fmt.Sprintf("Performance history reset on database %q.", dbID), nil
	default:
		return "", fmt.Errorf("invalid performance action %q (use stats, slow_queries, engine_slow_queries, suggest, suggest_indexes, or reset)", action)
	}
}

// engineSlowQueries surfaces the database's own statement statistics —
// ground truth that outlives this process, unlike the in-memory tracker.
// PostgreSQL reads pg_stat_statements (graceful note when the extension is
// absent); MySQL reads performance_schema digest tables. Output reuses the
// standard query-result formatting.
func (uc *DatabaseUseCase) engineSlowQueries(ctx context.Context, dbID string, limit int) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}

	var header, sqlText string
	switch strings.ToLower(dbType) {
	case "postgres", "timescale", "timescaledb":
		header = "Top statements by mean execution time (pg_stat_statements):\n\n"
		check, err := uc.ExecuteQuery(ctx, dbID, `SELECT count(*) AS n FROM pg_extension WHERE extname = 'pg_stat_statements'`, nil)
		if err == nil && !strings.Contains(check, " 1") {
			return "pg_stat_statements extension is not available on this database. Enable it with:\n  CREATE EXTENSION pg_stat_statements;\n(shared_preload_libraries must include pg_stat_statements for tracking.)", nil
		}
		sqlText = fmt.Sprintf(`SELECT round(mean_exec_time::numeric, 2) AS mean_ms, calls, left(query, 80) AS query
FROM pg_stat_statements
ORDER BY mean_exec_time DESC LIMIT %d`, limit)
	case "mysql":
		header = "Top statement digests by total time (performance_schema):\n\n"
		sqlText = fmt.Sprintf(`SELECT LEFT(DIGEST_TEXT, 80) AS digest, COUNT_STAR AS calls,
ROUND(AVG_TIMER_WAIT/1e9, 2) AS mean_ms, ROUND(SUM_TIMER_WAIT/1e9, 2) AS total_ms
FROM performance_schema.events_statements_summary_by_digest
WHERE SCHEMA_NAME = DATABASE()
ORDER BY SUM_TIMER_WAIT DESC LIMIT %d`, limit)
	default:
		return fmt.Sprintf("engine-level statement statistics are not supported on %q; they are available on PostgreSQL (pg_stat_statements) and MySQL (performance_schema)", dbType), nil
	}

	out, err := uc.ExecuteQuery(ctx, dbID, sqlText, nil)
	if err != nil {
		note := fmt.Sprintf("engine-level statistics unavailable: %v", err)
		switch strings.ToLower(dbType) {
		case "mysql":
			note += "\nReading statement digests requires SELECT on performance_schema.events_statements_summary_by_digest."
		case "postgres", "timescale", "timescaledb":
			note += "\npg_stat_statements must also be loaded via shared_preload_libraries for tracking."
		}
		return note, nil
	}
	return header + out, nil
}

func formatQueryMetrics(analyzer *dbtools.PerformanceAnalyzer) string {
	metrics := analyzer.GetAllMetrics()
	var b strings.Builder
	b.WriteString("Query performance metrics:\n\n")
	if len(metrics) == 0 {
		b.WriteString("No queries tracked yet. Metrics accumulate as query_* tools execute.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "%-8s %-10s %-10s %-10s %s\n", "COUNT", "AVG(ms)", "MAX(ms)", "MIN(ms)", "QUERY")
	for _, m := range metrics {
		query := m.Query
		if len(query) > 60 {
			query = query[:57] + "..."
		}
		fmt.Fprintf(&b, "%-8d %-10.2f %-10.2f %-10.2f %s\n",
			m.Count,
			float64(m.AvgDuration.Microseconds())/1000.0,
			float64(m.MaxDuration.Microseconds())/1000.0,
			float64(m.MinDuration.Microseconds())/1000.0,
			strings.ReplaceAll(query, "\n", " "))
	}
	return b.String()
}

func formatSlowQueries(records []dbtools.QueryRecord, limit int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Recorded slow queries (newest first, up to %d):\n\n", limit))
	if len(records) == 0 {
		b.WriteString("No slow queries recorded above the current threshold.\n")
		return b.String()
	}

	shown := 0
	for i := len(records) - 1; i >= 0 && shown < limit; i-- {
		r := records[i]
		query := r.Query
		if len(query) > 80 {
			query = query[:77] + "..."
		}
		fmt.Fprintf(&b, "[%s] %d.%03dms — %s", r.StartTime.Format(time.RFC3339),
			r.Duration.Milliseconds(), int(r.Duration.Microseconds())%1000, strings.ReplaceAll(query, "\n", " "))
		if r.Error != "" {
			fmt.Fprintf(&b, " (error: %s)", r.Error)
		}
		b.WriteString("\n")
		shown++
	}
	return b.String()
}

func formatSuggestions(query string, issues map[string]string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Static analysis for: %s\n\n", strings.TrimSpace(strings.ReplaceAll(query, "\n", " "))))
	if len(issues) == 0 {
		b.WriteString("No common issues detected. For execution-level insight use the explain tool.\n")
		return b.String()
	}

	names := make([]string, 0, len(issues))
	for name := range issues {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&b, "- %s: %s\n", name, issues[name])
	}
	return b.String()
}
