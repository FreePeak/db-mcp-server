package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Aborted-connections audit: Aborted_connects climbing means failed
// handshakes — auth failures, TLS mismatches, or probes from hosts that
// will soon hit max_connect_errors and get blocked outright.
// Aborted_clients climbing means applications tear connections down
// without QUIT/COM_QUIT — usually pools closed hard or timeouts too
// tight. Cumulative counters; ratios against Connections are the
// signal.

// abortedConnsQuery returns the probe reading both abort counters plus
// the connection denominator, or "" when unsupported.
func abortedConnsQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT
       (SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME = 'Connections') AS total_conns,
       (SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME = 'Aborted_clients') AS aborted_clients,
       (SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME = 'Aborted_connects') AS aborted_connects`
	default:
		return ""
	}
}

// abortedConnVerdict classifies abort ratios; a healthy result renders
// "" so reports stay actionable. Thresholds are deliberately loose:
// these counters only matter at scale.
func abortedConnVerdict(totalConns, abortedClients, abortedConnects int64) string {
	if totalConns <= 0 {
		return "No connection history yet (Connections=0) — nothing to judge."
	}
	var notes []string
	if abortedConnects*10 >= totalConns && abortedConnects > 0 {
		notes = append(notes, fmt.Sprintf("WARNING: %d of %d connection attempts failed before authentication — failing handshakes (wrong password/TLS mismatch); hosts exceeding max_connect_errors get BLOCKED until FLUSH HOSTS.", abortedConnects, totalConns))
	}
	if abortedClients*10 >= totalConns && abortedClients > 0 {
		notes = append(notes, fmt.Sprintf("NOTE: %d of %d clients made unclean exits (no QUIT) — pools torn down hard or wait timeouts too tight; check application close paths.", abortedClients, totalConns))
	}
	if len(notes) == 0 {
		return ""
	}
	return strings.Join(notes, "\n")
}

// AuditAbortedConnections renders connection-abort health; a clean
// result states so explicitly.
func (uc *DatabaseUseCase) AuditAbortedConnections(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := abortedConnsQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("connection-abort introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("connection-abort query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing connection-abort rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read connection-abort counters: %w", rerr)
		}
		return "", fmt.Errorf("connection-abort query returned no rows")
	}

	var totalRaw, clientsRaw, connectsRaw string
	if scanErr := rows.Scan(&totalRaw, &clientsRaw, &connectsRaw); scanErr != nil {
		return "", fmt.Errorf("failed to scan connection-abort counters: %w", scanErr)
	}
	parse := func(s string) int64 {
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			logger.Error("unparseable connection counter %q: %v", s, err)
			return 0
		}
		return n
	}
	total, clients, connects := parse(totalRaw), parse(clientsRaw), parse(connectsRaw)
	if verdict := abortedConnVerdict(total, clients, connects); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("Connection health clean: %d connections, %d aborted pre-auth, %d unclean client exits.", total, connects, clients), nil
}
