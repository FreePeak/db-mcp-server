package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FreePeak/db-mcp-server/pkg/dbtools"
)

// weightedStatement pairs an analyzed statement with its real-world cost,
// so suggestions rank by impact rather than statement variety. Cost is the
// engine-reported total execution time when available (duration-ranked);
// otherwise call counts stand in (traffic-ranked).
type weightedStatement struct {
	sql         string
	executions  int
	totalMillis float64 // engine/tracker estimate of total time spent in this statement
}

// WorkloadIndexSuggestions analyzes the database's most expensive recent
// statements and proposes indexes serving the largest share of executions.
// It is our counterpart of Postgres MCP Pro's analyze_workload_indexes:
// engine catalogs (pg_stat_statements, MySQL digest tables) are preferred,
// with a fallback to this server's own execution history so every supported
// engine can answer. Heuristic output — verify with EXPLAIN before creating.
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

	var entries []statementAdvice
	totalWeight := 0
	durationRanked := false
	for _, s := range stmts {
		if a := extractIndexAdvice(s.sql); len(a) > 0 {
			var w int
			if s.totalMillis > 0 {
				// Cost units are milliseconds of accumulated engine time:
				// one slow query can outweigh thousands of cheap ones.
				w = int(s.totalMillis)
				if w < 1 {
					w = 1
				}
				durationRanked = true
			} else {
				w = s.executions
				if w <= 0 {
					w = 1 // catalogs that omit call counts still count as one
				}
			}
			entries = append(entries, statementAdvice{advice: a, weight: w})
			totalWeight += w
		}
	}
	if len(entries) == 0 {
		return fmt.Sprintf("No index-worthy access patterns found across %d analyzed statement(s).", len(stmts)), nil
	}

	ranking := "ranked by traffic"
	if durationRanked {
		ranking = "ranked by estimated total time"
	}
	header := fmt.Sprintf(
		"Workload index suggestions from %d statement(s), %s (heuristic — verify with EXPLAIN before creating):",
		len(entries), ranking)
	unit := "execution(s)"
	if durationRanked {
		unit = "ms of engine time"
	}
	return uc.emitIndexSuggestions(ctx, dbID, dbType, entries, header, true, unit), nil
}

// workloadStatements returns up to limit expensive statements with their
// execution counts: engine catalogs first (full statement text, unlike the
// display-truncated slow query views), then this server's own tracked
// history.
func (uc *DatabaseUseCase) workloadStatements(ctx context.Context, dbID, dbType string, limit int) []weightedStatement {
	if queries := workloadQueries(dbType, limit); len(queries) > 0 {
		rows, err := uc.queryTableMetadata(ctx, dbID, queries)
		if err == nil {
			out := make([]weightedStatement, 0, len(rows))
			for _, r := range rows {
				var ws *weightedStatement
				for _, key := range []string{"query", "QUERY", "Query"} {
					if v, ok := r[key]; ok {
						if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
							ws = &weightedStatement{sql: s, executions: 1}
						}
						break
					}
				}
				if ws == nil {
					continue
				}
				for _, key := range []string{"calls", "executions", "CALLS", "EXECUTIONS", "Count", "count"} {
					if v, ok := r[key]; ok {
						switch n := v.(type) {
						case int:
							ws.executions = n
						case int64:
							ws.executions = int(n)
						case float64:
							ws.executions = int(n)
						}
						break
					}
				}
				for _, key := range []string{"total_ms", "TOTAL_MS"} {
					if v, ok := r[key]; ok {
						switch n := v.(type) {
						case int:
							ws.totalMillis = float64(n)
						case int64:
							ws.totalMillis = float64(n)
						case float64:
							ws.totalMillis = n
						}
						break
					}
				}
				out = append(out, *ws)
			}
			if len(out) > 0 {
				return out
			}
		}
	}

	metrics := dbtools.GetPerformanceAnalyzer().GetAllMetrics()
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].AvgDuration*time.Duration(metrics[i].Count) > metrics[j].AvgDuration*time.Duration(metrics[j].Count)
	})
	out := make([]weightedStatement, 0, limit)
	for _, m := range metrics {
		totalMillis := float64(m.AvgDuration.Milliseconds()) * float64(m.Count)
		if totalMillis <= 0 {
			totalMillis = float64(m.Count) // degenerate durations; rank by traffic
		}
		out = append(out, weightedStatement{sql: m.Query, executions: m.Count, totalMillis: totalMillis})
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
			`SELECT query, calls AS executions, total_exec_time AS total_ms FROM pg_stat_statements ORDER BY total_exec_time DESC LIMIT %d`, limit)}
	case "mysql":
		return []string{fmt.Sprintf(
			"SELECT DIGEST_TEXT AS query, COUNT_STAR AS executions, SUM_TIMER_WAIT/1000000 AS total_ms FROM performance_schema.events_statements_summary_by_digest WHERE DIGEST_TEXT LIKE 'SELECT%%' ORDER BY SUM_TIMER_WAIT DESC LIMIT %d", limit)}
	default:
		return nil
	}
}
