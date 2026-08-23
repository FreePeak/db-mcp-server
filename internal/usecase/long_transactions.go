package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Idle-in-transaction detection: a transaction left open (an app bug,
// a forgotten psql window) holds locks, blocks vacuum from reclaiming
// dead tuples, and starves replication. list_sessions shows running
// queries; running-long queries are its job — this audits transactions
// that sit open with no statement executing.

// longTransactionsQuery returns the engine's open-transaction SELECT
// with age rendering, or "" when unsupported.
func longTransactionsQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT pid, usename,
       COALESCE(state, '') AS state,
       now() - xact_start AS xact_age,
       COALESCE(left(query, 120), '') AS last_query
FROM pg_stat_activity
WHERE state = 'idle in transaction' AND xact_start IS NOT NULL
  AND now() - xact_start > interval '1 second' * %d
ORDER BY xact_start`
	case "mysql", "mariadb":
		return `SELECT trx_mysql_thread_id AS pid,
       trx_started AS started,
       TIMESTAMPDIFF(SECOND, trx_started, NOW()) AS age_secs,
       trx_state AS state,
       COALESCE(trx_query, '') AS last_query
FROM information_schema.innodb_trx
WHERE TIMESTAMPDIFF(SECOND, trx_started, NOW()) > %d
ORDER BY trx_started`
	default:
		return ""
	}
}

// ListLongTransactions renders every idle-in-transaction session older
// than minAgeSecs with its age and last statement.
func (uc *DatabaseUseCase) ListLongTransactions(ctx context.Context, dbID string, minAgeSecs int) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	tmpl := longTransactionsQuery(dbType)
	if tmpl == "" {
		return "", fmt.Errorf("transaction introspection is not available for engine %q", dbType)
	}
	if minAgeSecs <= 0 {
		minAgeSecs = 60
	}
	q := fmt.Sprintf(tmpl, minAgeSecs)
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("long-transaction catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing long-transaction rows: %v", closeErr)
		}
	}()

	var lines []string
	for rows.Next() {
		var id, lastQuery string
		var age string
		switch strings.ToLower(dbType) {
		case "mysql", "mariadb":
			var ageSecs int64
			var started, state string
			if scanErr := rows.Scan(&id, &started, &ageSecs, &state, &lastQuery); scanErr == nil {
				lines = append(lines, fmt.Sprintf("- pid %s: open %ds (%s) — last: %s",
					id, ageSecs, state, oneLine(lastQuery)))
			}
			continue
		default:
			var user string
			var state interface{}
			if scanErr := rows.Scan(&id, &user, &state, &age, &lastQuery); scanErr == nil {
				lines = append(lines, fmt.Sprintf("- pid %s (%s): %s old, %s — last: %s",
					id, user, strings.TrimSpace(fmt.Sprintf("%v", age)), strings.TrimSpace(fmt.Sprintf("%v", state)), oneLine(lastQuery)))
			}
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate long-transaction rows: %w", err)
	}
	if len(lines) == 0 {
		return fmt.Sprintf("No transactions idle-in-transaction older than %ds.", minAgeSecs), nil
	}
	out := fmt.Sprintf("%d idle-in-transaction session(s) older than %ds:\n%s", len(lines), minAgeSecs, strings.Join(lines, "\n"))
	return out, nil
}

// oneLine collapses whitespace so multi-line statements render inline.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
