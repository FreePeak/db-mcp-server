package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/pkg/dbtools"
)

// WorkloadIndexSuggestions analyzes the database's most expensive recent
// statements and proposes indexes serving the largest share of them. It is
// our counterpart of Postgres MCP Pro's analyze_workload_indexes: engine
// catalogs (pg_stat_statements, MySQL digest tables) are preferred, with a
// fallback to this server's own execution history so every supported engine
// can answer. Heuristic output — verify with EXPLAIN before creating.
func (uc *DatabaseUseCase) WorkloadIndexSuggestions(ctx context.Context, dbID string, limit int) (string, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 25 {
		limit = 25
	}
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}

	stmts := uc.workloadStatements(ctx, dbID, strings.ToLower(dbType), limit)
	if len(stmts) == 0 {
		return "No workload statements available yet. Run some queries through this server first, or enable engine statistics (pg_stat_statements on PostgreSQL, performance_schema on MySQL).", nil
	}

	var adviceList []map[string]*tableAdvice
	for _, s := range stmts {
		if a := extractIndexAdvice(s); len(a) > 0 {
			adviceList = append(adviceList, a)
		}
	}
	if len(adviceList) == 0 {
		return fmt.Sprintf("No index-worthy access patterns found across %d analyzed statement(s).", len(stmts)), nil
	}

	header := fmt.Sprintf(
		"Workload index suggestions from %d statement(s), ranked by coverage (heuristic — verify with EXPLAIN before creating):",
		len(adviceList))
	return uc.emitIndexSuggestions(ctx, dbID, dbType, adviceList, header, true), nil
}

// workloadStatements returns up to limit expensive statements: engine
// catalogs first (full statement text, unlike the display-truncated slow
// query views), then this server's own tracked history.
func (uc *DatabaseUseCase) workloadStatements(ctx context.Context, dbID, dbType string, limit int) []string {
	if queries := workloadQueries(dbType, limit); len(queries) > 0 {
		rows, err := uc.queryTableMetadata(ctx, dbID, queries)
		if err == nil {
			out := make([]string, 0, len(rows))
			for _, r := range rows {
				for _, key := range []string{"query", "QUERY", "Query"} {
					if v, ok := r[key]; ok {
						if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
							out = append(out, s)
						}
						break
					}
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}

	metrics := dbtools.GetPerformanceAnalyzer().GetAllMetrics()
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].AvgDuration > metrics[j].AvgDuration
	})
	out := make([]string, 0, limit)
	for _, m := range metrics {
		if out == nil {
			out = make([]string, 0, limit)
		}
		out = append(out, m.Query)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// workloadQueries returns catalog queries for the engine's statement
// statistics; nil when the engine has no statement-stats catalog.
func workloadQueries(dbType string, limit int) []string {
	switch dbType {
	case "postgres", "timescale", "timescaledb":
		return []string{fmt.Sprintf(
			`SELECT query FROM pg_stat_statements ORDER BY total_exec_time DESC LIMIT %d`, limit)}
	case "mysql":
		return []string{fmt.Sprintf(
			"SELECT DIGEST_TEXT AS query FROM performance_schema.events_statements_summary_by_digest WHERE DIGEST_TEXT LIKE 'SELECT%%' ORDER BY SUM_TIMER_WAIT DESC LIMIT %d", limit)}
	default:
		return nil
	}
}
