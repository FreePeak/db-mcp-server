package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// SQLite foreign-key enforcement audit: SQLite parses FOREIGN KEY
// constraints but does not enforce them unless PRAGMA foreign_keys=ON
// is set on the connection — the default is OFF. With it off, inserts
// of orphaned child rows succeed silently and referential integrity is
// only a promise in the schema. The setting is per-connection (and
// cannot change inside a transaction), so this audits the connections
// this server itself uses.

// fkEnforcementQuery returns the enforcement pragma for SQLite, or ""
// when unsupported.
func fkEnforcementQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "sqlite", "sqlite3":
		return `PRAGMA foreign_keys`
	default:
		return ""
	}
}

// fkEnforcementVerdict classifies the flag; enforced renders "" so
// reports stay actionable.
func fkEnforcementVerdict(enforced bool) string {
	if enforced {
		return ""
	}
	return "WARNING: foreign_keys=OFF — FK constraints are parsed but NOT enforced; orphaned child rows are written silently. Fix: PRAGMA foreign_keys=ON on every connection (per-connection setting; set it right after open, it cannot flip inside a transaction)."
}

// AuditFKEnforcement renders whether writes through this server
// enforce foreign keys; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditFKEnforcement(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := fkEnforcementQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("foreign_keys introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("foreign_keys pragma failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing foreign_keys rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read foreign_keys: %w", rerr)
		}
		return "", fmt.Errorf("foreign_keys pragma returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan foreign_keys: %w", scanErr)
	}
	on, perr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if perr != nil {
		logger.Error("unparseable foreign_keys %q: %v", raw, perr)
		on = 0
	}
	if verdict := fkEnforcementVerdict(on == 1); verdict != "" {
		return verdict, nil
	}
	return "foreign_keys=ON: FK constraints are enforced on this server's connections.", nil
}
