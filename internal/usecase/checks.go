package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// CHECK-constraint listing: check clauses encode business rules —
// "status IN ('active','closed')", "amount >= 0" — that an agent needs
// to write valid INSERTs or understand why one failed. The describe
// constraint catalog only surfaces PK/FK; this reads the engine's
// check_constraints catalogs directly.

// checksCatalog returns the engine's CHECK-constraint SELECT, or ""
// when unsupported.
func checksCatalog(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT tc.table_name, cc.check_clause
FROM information_schema.table_constraints tc
JOIN information_schema.check_constraints cc
  ON cc.constraint_name = tc.constraint_name
 AND cc.constraint_schema = tc.constraint_schema
WHERE tc.constraint_type = 'CHECK'
ORDER BY tc.table_name`
	case "mysql", "mariadb":
		return `SELECT TABLE_NAME, CHECK_CLAUSE
FROM information_schema.CHECK_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA = DATABASE()
ORDER BY TABLE_NAME`
	default:
		return ""
	}
}

// ListCheckConstraints renders every table's CHECK clauses grouped by
// table so business rules are visible without parsing DDL.
func (uc *DatabaseUseCase) ListCheckConstraints(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := checksCatalog(dbType)
	if q == "" {
		return "", fmt.Errorf("CHECK-constraint introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("check-constraint catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing checks rows: %v", closeErr)
		}
	}()

	byTable := map[string][]string{}
	var tables []string
	count := 0
	for rows.Next() {
		var table, clause string
		if scanErr := rows.Scan(&table, &clause); scanErr != nil {
			continue // unscannable row: skip rather than fail the listing
		}
		if byTable[table] == nil {
			tables = append(tables, table)
		}
		byTable[table] = append(byTable[table], strings.TrimSpace(clause))
		count++
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate check rows: %w", err)
	}
	if count == 0 {
		return "No CHECK constraints defined (business rules live in application code).", nil
	}
	sort.Strings(tables)
	var b strings.Builder
	fmt.Fprintf(&b, "%d CHECK constraint(s) across %d table(s):\n", count, len(tables))
	for _, t := range tables {
		fmt.Fprintf(&b, "\n%s:\n", t)
		for _, clause := range byTable[t] {
			fmt.Fprintf(&b, "  - %s\n", clause)
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
