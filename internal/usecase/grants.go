package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Grants audit: who can do what to which tables. An agent asked "why
// can't this app read orders?" or reviewing least-privilege had no
// visibility — privilege catalogs were never queried. Catalog reads
// only; revocations are rendered for review, never executed.

// grantsCatalog returns the engine's table-privilege SELECT, or ""
// when unsupported.
func grantsCatalog(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT grantee, table_name, privilege_type
FROM information_schema.role_table_grants
WHERE table_schema = current_schema()
ORDER BY grantee, table_name`
	case "mysql", "mariadb":
		return `SELECT GRANTEE, TABLE_NAME, PRIVILEGE_TYPE
FROM information_schema.TABLE_PRIVILEGES
WHERE TABLE_SCHEMA = DATABASE()
ORDER BY GRANTEE, TABLE_NAME`
	default:
		return ""
	}
}

// ListGrants renders the database's table privileges grouped by grantee.
func (uc *DatabaseUseCase) ListGrants(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := grantsCatalog(dbType)
	if q == "" {
		return "", fmt.Errorf("privilege introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("grants catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing grants rows: %v", closeErr)
		}
	}()

	type key struct{ grantee, table string }
	privs := map[key][]string{}
	grantees := map[string]bool{}
	count := 0
	for rows.Next() {
		var grantee, table, priv string
		if scanErr := rows.Scan(&grantee, &table, &priv); scanErr != nil {
			continue
		}
		k := key{grantee, table}
		privs[k] = append(privs[k], priv)
		grantees[grantee] = true
		count++
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate grants rows: %w", err)
	}
	if count == 0 {
		return fmt.Sprintf("No table-level grants visible in %s (owner-only access or none granted).", dbID), nil
	}

	names := make([]string, 0, len(grantees))
	for g := range grantees {
		names = append(names, g)
	}
	sort.Strings(names)
	var b strings.Builder
	fmt.Fprintf(&b, "%d table privilege(s) across %d grantee(s):\n", count, len(names))
	for _, g := range names {
		tables := make([]string, 0)
		for k := range privs {
			if k.grantee == g {
				tables = append(tables, k.table)
			}
		}
		sort.Strings(tables)
		fmt.Fprintf(&b, "\n%s:\n", g)
		for _, t := range tables {
			sort.Strings(privs[key{g, t}])
			fmt.Fprintf(&b, "  %s: %s\n", t, strings.Join(privs[key{g, t}], ", "))
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
