package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Extension listing: performance.go probes for pg_stat_statements
// specifically, but nothing shows the full picture — which extensions
// are installed (timescaledb? postgis?) and what is available but not
// yet installed. One catalog read answers "why does engine_slow_queries
// say unsupported?" before anyone guesses.

// extensionQuery returns the installed+available extension SELECT, or ""
// when unsupported.
func extensionQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT name, version, installed FROM (
  SELECT e.extname AS name,
         e.extversion AS version,
         true AS installed
  FROM pg_extension e
  UNION ALL
  SELECT a.name,
         COALESCE(a.default_version, '') AS version,
         false AS installed
  FROM pg_available_extensions a
  WHERE NOT EXISTS (SELECT 1 FROM pg_extension x WHERE x.extname = a.name)
) t
ORDER BY installed DESC, name`
	default:
		return ""
	}
}

// ListExtensions renders every installed Postgres extension with its
// version, then available-but-not-installed ones; engine-gated features
// become checkable instead of guessable.
func (uc *DatabaseUseCase) ListExtensions(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := extensionQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("extension introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("extension catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing extension rows: %v", closeErr)
		}
	}()

	var installed, available []string
	for rows.Next() {
		var name, version string
		var isInstalled bool
		if scanErr := rows.Scan(&name, &version, &isInstalled); scanErr != nil {
			continue // unscannable row: skip rather than fail the listing
		}
		entry := "- " + name
		if version != "" {
			entry += " (" + version + ")"
		}
		if isInstalled {
			installed = append(installed, entry)
		} else {
			available = append(available, entry+" — not installed")
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate extension rows: %w", err)
	}

	if len(installed) == 0 && len(available) == 0 {
		return "No extensions installed and none available.", nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d extension(s) installed:\n%s", len(installed), strings.Join(installed, "\n"))
	if len(available) > 0 {
		fmt.Fprintf(&b, "\n\n%d available (not installed):\n%s", len(available), strings.Join(available, "\n"))
	}
	return b.String(), nil
}
