package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// MD5 password audit: roles whose stored hash is still md5… are
// protected by a broken scheme — rainbow-tableable and without
// channel binding. Postgres has shipped SCRAM-SHA-256 as the default
// since v14; a straggler is either an old role nobody re-set or a
// server still defaulting to md5. Two signals: the server-level
// password_encryption default (what NEW passwords get) and per-role
// stored hashes in pg_authid (what exists today).

// passwordEncryptionQuery returns the probe for the server's password
// hashing default, or "" when unsupported.
func passwordEncryptionQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('password_encryption') AS encryption`
	default:
		return ""
	}
}

// md5RolesQuery returns the SELECT for login roles whose stored hash
// is still md5, or "" when unsupported.
func md5RolesQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT rolname
FROM pg_authid
WHERE rolcanlogin
  AND rolpassword LIKE 'md5%'
ORDER BY rolname`
	default:
		return ""
	}
}

// encryptionVerdict classifies the server-level default.
func encryptionVerdict(encryption string) string {
	if !strings.EqualFold(strings.TrimSpace(encryption), "scram-sha-256") {
		return fmt.Sprintf("WARNING: password_encryption=%s — new passwords may still be stored as md5. Set it to scram-sha-256.", encryption)
	}
	return "Server default healthy: new passwords are hashed with SCRAM-SHA-256."
}

// AuditPasswordAuth renders which login roles still carry md5 hashes
// plus the server default; a clean result is stated explicitly.
func (uc *DatabaseUseCase) AuditPasswordAuth(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	settingsQ := passwordEncryptionQuery(dbType)
	rolesQ := md5RolesQuery(dbType)
	if settingsQ == "" || rolesQ == "" {
		return "", fmt.Errorf("password-auth introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	var lines []string

	rows, err := db.Query(ctx, settingsQ)
	if err != nil {
		return "", fmt.Errorf("password_encryption query failed (requires superuser or pg_read_all_settings): %w", err)
	}
	if rows.Next() {
		var encryption string
		if scanErr := rows.Scan(&encryption); scanErr == nil && !strings.EqualFold(strings.TrimSpace(encryption), "scram-sha-256") {
			lines = append(lines, "- "+encryptionVerdict(encryption))
		}
	}
	if cerr := rows.Close(); cerr != nil {
		logger.Error("error closing password-encryption rows: %v", cerr)
	}

	roleRows, err := db.Query(ctx, rolesQ)
	if err != nil {
		return "", fmt.Errorf("pg_authid query failed (requires superuser or pg_read_all_stats): %w", err)
	}
	defer func() {
		if closeErr := roleRows.Close(); closeErr != nil {
			logger.Error("error closing md5-role rows: %v", closeErr)
		}
	}()

	var md5Roles []string
	for roleRows.Next() {
		var name string
		if scanErr := roleRows.Scan(&name); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		md5Roles = append(md5Roles, name)
	}
	if rerr := roleRows.Err(); rerr != nil {
		return "", fmt.Errorf("failed to iterate md5 roles: %w", rerr)
	}
	for _, name := range md5Roles {
		lines = append(lines, fmt.Sprintf(
			"- role %s: stored hash is md5 — re-run ALTER ROLE %s WITH PASSWORD to upgrade to SCRAM-SHA-256",
			name, quoteIdent(name)))
	}

	if len(lines) == 0 {
		return "No md5 password risks found: SCRAM default and no md5-hashed login roles.", nil
	}
	out := fmt.Sprintf("%d password-auth risk(s):\n%s", len(lines), strings.Join(lines, "\n"))
	if len(md5Roles) > 0 {
		out += "\nNote: clients must support SCRAM before upgrading their passwords."
	}
	return out, nil
}
