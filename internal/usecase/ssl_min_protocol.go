package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// ssl_min_protocol_version audit: the floor of TLS versions Postgres
// accepts. TLSv1 and TLSv1.1 are deprecated (PCI-DSS prohibits them
// outright) and downgrade-vulnerable; modern servers should floor at
// TLSv1.2 or TLSv1.3. And when `ssl` itself is off every connection
// is plaintext while everyone assumes TLS — that is escalated before
// protocol version even matters.

// sslMinProtocolProbe returns the probe reading both settings, or ""
// when unsupported.
func sslMinProtocolProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('ssl_min_protocol_version') AS min_proto,
       current_setting('ssl') AS ssl_enabled`
	default:
		return ""
	}
}

// sslMinProtocolVerdict classifies (sslEnabled, minProto); healthy
// values render "" so reports stay actionable.
func sslMinProtocolVerdict(sslEnabled bool, minProto string) string {
	if !sslEnabled {
		return "WARNING: ssl=off — client connections are unencrypted in transit; anyone on the network path reads credentials and data. Fix: enable ssl in postgresql.conf with a server certificate and require hostssl rules in pg_hba.conf (restart required)."
	}
	p := strings.ToUpper(strings.TrimSpace(minProto))
	switch p {
	case "TLSV1.3", "TLSV1.2":
		return "" // modern floor
	case "TLSV1", "TLSV1.1":
		return fmt.Sprintf("WARNING: ssl_min_protocol_version=%s — deprecated protocols vulnerable to downgrade attacks and prohibited by PCI-DSS. Fix: ALTER SYSTEM SET ssl_min_protocol_version = 'TLSv1.2' then SELECT pg_reload_conf(); clients older than ~2010 will be refused.",
			minProto)
	default:
		return fmt.Sprintf("ssl_min_protocol_version=%q is unreadable — verify with SHOW ssl_min_protocol_version.", minProto)
	}
}

// AuditSSLMinProtocol renders whether connections meet modern TLS
// floors; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditSSLMinProtocol(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := sslMinProtocolProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("ssl introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("ssl query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing ssl rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read ssl settings: %w", rerr)
		}
		return "", fmt.Errorf("ssl query returned no rows")
	}

	var minProtoRaw, sslRaw interface{}
	if scanErr := rows.Scan(&minProtoRaw, &sslRaw); scanErr != nil {
		return "", fmt.Errorf("failed to scan ssl settings: %w", scanErr)
	}
	minProto := strings.TrimSpace(fmt.Sprintf("%v", minProtoRaw))
	sslOn := strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", sslRaw)), "on")
	if verdict := sslMinProtocolVerdict(sslOn, minProto); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("TLS healthy: ssl=on with floor %s.", strings.TrimSpace(minProto)), nil
}
