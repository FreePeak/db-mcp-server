package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// max_allowed_packet audit: the largest single statement the server
// accepts. Too small and large blob writes or big multi-row INSERTs
// fail with "MySQL server has gone away" or "Packet too large" —
// errors that look like network problems but are pure configuration.
// Legacy defaults (1–4 MB) predate modern JSON/blob payloads.

// maxAllowedPacketQuery returns the probe for the global setting, or
// "" when unsupported.
func maxAllowedPacketQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT COALESCE(@@GLOBAL.max_allowed_packet, 0) AS max_packet`
	default:
		return ""
	}
}

// humanMB renders a byte count in MB with one decimal when fractional.
func humanMB(n int64) string {
	return fmt.Sprintf("%.1fMB", float64(n)/(1024*1024))
}

// maxAllowedPacketVerdict classifies the packet ceiling; a comfortable
// size renders "" so reports stay actionable. Below 4MB is the legacy
// default territory where "gone away" reports cluster.
func maxAllowedPacketVerdict(maxBytes int64) string {
	switch {
	case maxBytes <= 0:
		return "max_allowed_packet is 0 or unreadable — check server configuration."
	case maxBytes < 16*1024*1024:
		return fmt.Sprintf("WARNING: max_allowed_packet=%s is tight — large blob writes or big multi-row INSERTs fail with 'server has gone away'/'Packet too large'. SET GLOBAL max_allowed_packet=67108864 (64MB) and mirror it on the client DSN.", humanMB(maxBytes))
	default:
		return ""
	}
}

// AuditMaxAllowedPacket renders whether large statements will fit; a
// healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditMaxAllowedPacket(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := maxAllowedPacketQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("max_allowed_packet introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("max_allowed_packet query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing max-packet rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read max_allowed_packet: %w", rerr)
		}
		return "", fmt.Errorf("max_allowed_packet query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan max_allowed_packet: %w", scanErr)
	}
	n, perr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if perr != nil {
		logger.Error("unparseable max_allowed_packet %q: %v", raw, perr)
		n = 0
	}
	if verdict := maxAllowedPacketVerdict(n); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("max_allowed_packet healthy: %s — large statements fit comfortably.", humanMB(n)), nil
}
