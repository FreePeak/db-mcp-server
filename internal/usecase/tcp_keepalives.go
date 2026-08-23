package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// tcp_keepalives_idle audit: how long a connection must sit idle
// before the OS sends TCP keepalive probes. The default (0 = OS
// default, often 2 hours on Linux) lets dead clients — closed
// laptops, dropped NAT sessions — hold connection slots for hours.
// idle_sessions shows the symptom; this names the config-level fix
// so the slots free themselves in minutes instead.

const (
	tcpKeepaliveQuietSecs  = 600       // ≤10 minutes frees dead slots fast enough
	tcpKeepaliveUnreadable = int64(-1) // sentinel: unparseable values render as unreadable
)

// tcpKeepalivesProbe returns the probe reading the setting in
// seconds, or "" when unsupported.
func tcpKeepalivesProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('tcp_keepalives_idle') AS keepalive_secs`
	default:
		return ""
	}
}

// tcpKeepaliveVerdict classifies the setting in seconds; a tight
// value renders "" so reports stay actionable. Zero means the OS
// default (often 2h) and is escalated as the risky case it is.
func tcpKeepaliveVerdict(secs int64) string {
	if secs == tcpKeepaliveUnreadable {
		return "tcp_keepalives_idle is unreadable on this platform — verify with SHOW tcp_keepalives_idle."
	}
	if secs <= tcpKeepaliveQuietSecs && secs > 0 {
		return "" // dead slots reclaimed within minutes
	}
	d := time.Duration(secs) * time.Second
	fix := "ALTER SYSTEM SET tcp_keepalives_idle = '300' then SELECT pg_reload_conf()"
	if secs == 0 {
		return fmt.Sprintf("WARNING: tcp_keepalives_idle=0 — the OS default applies (often 2 hours), so a dead client holds its connection slot that long before the kernel notices. Fix: %s; idle connections then die in ~5 minutes.", fix)
	}
	return fmt.Sprintf("WARNING: tcp_keepalives_idle=%s — a dead client (closed laptop, dropped NAT session) holds its connection slot this long before keepalive fires. Fix: %s.",
		d, fix)
}

// AuditTCPKeepalives renders whether dead clients release their
// slots promptly; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditTCPKeepalives(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := tcpKeepalivesProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("tcp_keepalives_idle introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("tcp_keepalives_idle query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing tcp-keepalive rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read tcp_keepalives_idle: %w", rerr)
		}
		return "", fmt.Errorf("tcp_keepalives_idle query returned no rows")
	}

	var raw interface{}
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan tcp_keepalives_idle: %w", scanErr)
	}
	s := strings.TrimSpace(fmt.Sprintf("%v", raw))
	secs, perr := strconv.ParseInt(s, 10, 64)
	if perr != nil {
		// Some platforms render the resolved value with units or blank;
		// treat anything non-numeric as unreadable rather than guessing.
		secs = tcpKeepaliveUnreadable
	}
	if verdict := tcpKeepaliveVerdict(secs); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("tcp_keepalives_idle healthy: dead clients released after %ds.", secs), nil
}
