package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Strict-mode audit: without STRICT_TRANS_TABLES in sql_mode, MySQL
// silently truncates overlong strings, coerces invalid dates to the
// zero-date, and substitutes zeros for bad numerics — corruption that
// surfaces weeks later as "how did this value get here?". Legacy
// servers and old docker images frequently ship without it.

// sqlModeQuery returns the probe for the global sql_mode, or "" when
// unsupported.
func sqlModeQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT @@GLOBAL.sql_mode AS mode`
	default:
		return ""
	}
}

// strictModeVerdict classifies sql_mode against data-corruption risk;
// a healthy result renders "" so reports stay actionable.
func strictModeVerdict(mode string) string {
	for _, m := range strings.FieldsFunc(mode, func(r rune) bool { return r == ',' || r == ' ' }) {
		if strings.EqualFold(m, "STRICT_TRANS_TABLES") || strings.EqualFold(m, "STRICT_ALL_TABLES") {
			return fmt.Sprintf("Strict mode enabled: invalid values are rejected instead of silently coerced (%s).", firstToken(mode))
		}
	}
	return fmt.Sprintf("WARNING: STRICT_TRANS_TABLES is absent from sql_mode — overlong strings are silently truncated and invalid dates/numerics coerced instead of erroring. SET GLOBAL sql_mode=CONCAT(@@sql_mode, ',STRICT_TRANS_TABLES') (current: %s).", mode)
}

func firstToken(mode string) string {
	f := strings.Fields(strings.ReplaceAll(mode, ",", " "))
	if len(f) == 0 {
		return "(empty)"
	}
	return f[0]
}

// AuditStrictMode renders whether invalid values are rejected at write
// time; a durable result is stated explicitly.
func (uc *DatabaseUseCase) AuditStrictMode(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := sqlModeQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("sql_mode introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("sql_mode query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing sql_mode rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read sql_mode: %w", rerr)
		}
		return "", fmt.Errorf("sql_mode query returned no rows")
	}

	var mode string
	if scanErr := rows.Scan(&mode); scanErr != nil {
		return "", fmt.Errorf("failed to scan sql_mode: %w", scanErr)
	}
	return strictModeVerdict(mode), nil
}
