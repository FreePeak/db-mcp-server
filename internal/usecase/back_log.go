package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// back_log audit: the TCP listen backlog for new connections. Bursts
// beyond it are refused by the kernel before authentication ever
// runs — a retry avalanche after a load spike looks like "connect
// failed" storms while max_connections still shows headroom. The
// setting is read-only at runtime: raising it requires my.cnf plus
// restart, which makes discovering it during an incident painful.
// -1 (MySQL 8 default) means autosized from max_connections and is
// healthy by definition.

// backLogProbe returns the probe pairing the setting with
// max_connections, or "" when unsupported.
func backLogProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT COALESCE(@@GLOBAL.back_log, '') AS backlog,
       @@GLOBAL.max_connections AS max_conn`
	default:
		return ""
	}
}

// backLogFloorSecs is not applicable here; backLogFloor is the
// smallest explicit value that absorbs typical burst arrivals.
const backLogFloor = 64

// backLogVerdict classifies the setting; healthy values render ""
// so reports stay actionable.
func backLogVerdict(backlog int64) string {
	switch {
	case backlog == 0:
		return "back_log is 0 or unreadable — verify with SHOW GLOBAL VARIABLES LIKE 'back_log'."
	case backlog < 0:
		return fmt.Sprintf("back_log=%d: autosized from max_connections (MySQL 8 default) — no action needed.", backlog)
	case backlog < backLogFloor:
		return fmt.Sprintf("WARNING: back_log=%d — connection bursts beyond the listen backlog are refused by the kernel before authentication, so load spikes surface as connect-failed storms even with max_connections headroom. Fix: set back_log=128 in my.cnf and restart (read-only at runtime).",
			backlog)
	default:
		return "" // adequate burst absorption
	}
}

// AuditBackLog renders whether connection bursts survive a spike; a
// healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditBackLog(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := backLogProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("back_log introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("back_log query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing back_log rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read back_log counters: %w", rerr)
		}
		return "", fmt.Errorf("back_log query returned no rows")
	}

	var backlogRaw interface{}
	var maxConnRaw interface{}
	if scanErr := rows.Scan(&backlogRaw, &maxConnRaw); scanErr != nil {
		return "", fmt.Errorf("failed to scan back_log counters: %w", scanErr)
	}
	parse := func(v interface{}) int64 {
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		n, perr := strconv.ParseInt(s, 10, 64)
		if perr != nil {
			logger.Error("unparseable back_log counter %q: %v", s, perr)
			return 0 // renders as unreadable, never guessed at
		}
		return n
	}
	backlog := parse(backlogRaw)
	maxConn := parse(maxConnRaw)
	if verdict := backLogVerdict(backlog); verdict != "" {
		return verdict, nil
	}
	if strings.TrimSpace(fmt.Sprintf("%v", backlogRaw)) == "-1" {
		return backLogVerdict(-1), nil
	}
	return fmt.Sprintf("back_log healthy: back_log=%d against max_connections=%d.",
		backlog, maxConn), nil
}
