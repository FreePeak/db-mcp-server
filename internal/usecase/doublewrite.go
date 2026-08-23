package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// innodb_doublewrite audit: the doublewrite buffer is InnoDB's defense
// against torn pages — a crash mid-write can leave a page half-written
// inside the tablespace, which is silent data corruption that survives
// until a backup restore exercises it. It is sometimes disabled for
// benchmarks ("doublewrite costs ~5% writes") and forgotten. ON (the
// default) is healthy.

// doublewriteQuery returns the probe for the setting, or "" when
// unsupported.
func doublewriteQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT @@GLOBAL.innodb_doublewrite AS dblb`
	default:
		return ""
	}
}

// doublewriteVerdict classifies the setting; ON renders "" so reports
// stay actionable.
func doublewriteVerdict(v string) string {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "ON", "1":
		return ""
	case "":
		return "innodb_doublewrite is empty or unreadable — verify with SHOW GLOBAL VARIABLES LIKE 'innodb_doublewrite'."
	default:
		return "WARNING: innodb_doublewrite=OFF — torn-page protection is disabled: a crash mid-write can leave half-written pages inside the tablespace, silent corruption that only surfaces when a backup is restored. Fix: set innodb_doublewrite=ON in my.cnf and restart (MySQL 8.0.30+ can SET GLOBAL it live). The benchmark write gain is not worth unrepairable data."
	}
}

// AuditDoublewrite renders whether torn-page protection is enabled; a
// healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditDoublewrite(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := doublewriteQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("doublewrite introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("doublewrite query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing doublewrite rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read doublewrite setting: %w", rerr)
		}
		return "", fmt.Errorf("doublewrite query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan doublewrite setting: %w", scanErr)
	}
	if verdict := doublewriteVerdict(raw); verdict != "" {
		return verdict, nil
	}
	return "Torn-page protection healthy: innodb_doublewrite is enabled.", nil
}
