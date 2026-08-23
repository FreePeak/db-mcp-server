package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Long-query observability: list engine sessions whose current query
// has been running longer than a threshold — the "database feels slow"
// triage view, without raw catalog SQL that read-only configs block.

// longQueriesCatalog returns the engine's activity SELECT (parameterized
// by the age threshold), or "" when unsupported.
func longQueriesCatalog(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT pid, usename, state,
       EXTRACT(EPOCH FROM (now() - query_start))::bigint AS seconds,
       left(query, 120) AS query
FROM pg_stat_activity
WHERE state = 'active' AND pid <> pg_backend_pid()
  AND now() - query_start > make_interval(secs => $1)
ORDER BY query_start`
	case "mysql", "mariadb":
		return `SELECT Id, User, state, Time, left(info, 120)
FROM information_schema.processlist
WHERE command = 'Query' AND TIME > ?
ORDER BY TIME`
	default:
		return ""
	}
}

// ListLongQueries renders active queries running longer than minSeconds.
func (uc *DatabaseUseCase) ListLongQueries(ctx context.Context, dbID string, minSeconds int) (string, error) {
	if minSeconds <= 0 {
		minSeconds = 30 // default triage threshold
	}
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := longQueriesCatalog(dbType)
	if q == "" {
		return "", fmt.Errorf("query activity is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	rows, err := db.Query(ctx, q, minSeconds)
	if err != nil {
		return "", fmt.Errorf("activity catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing activity rows: %v", closeErr)
		}
	}()

	var b strings.Builder
	count := 0
	for rows.Next() {
		var pid, user, state interface{}
		var secs interface{}
		var queryText interface{}
		if scanErr := rows.Scan(&pid, &user, &state, &secs, &queryText); scanErr != nil {
			return "", fmt.Errorf("failed to scan activity row: %w", scanErr)
		}
		fmt.Fprintf(&b, "- %v: user=%v state=%v running %vs: %v\n",
			pid, user, state, secs, queryText)
		count++
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate activity rows: %w", err)
	}
	if count == 0 {
		return fmt.Sprintf("No queries running longer than %d second(s).", minSeconds), nil
	}
	out := fmt.Sprintf("%d long-running query(ies):\n%s", count, b.String())
	return strings.TrimRight(out, "\n"), nil
}
