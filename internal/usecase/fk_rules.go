package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Foreign-key rule audit: an agent deleting parent rows cannot see
// whether ON DELETE CASCADE will silently destroy children or NO ACTION
// will block the delete — DescribeTable carries edge endpoints but not
// the referential actions. Reads delete/update rules straight from the
// engine catalogs and names the dangerous edges.

// fkRule is one foreign-key edge with its referential actions.
type fkRule struct {
	childTable  string
	childColumn string
	parentTable string
	parentCol   string
	deleteRule  string
	updateRule  string
}

// fkRulesQuery returns the engine's FK-actions SELECT (child table/
// column, parent table/column, delete_rule, update_rule), or "" when
// unsupported.
func fkRulesQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT kcu.table_name, kcu.column_name,
       ccu.table_name, ccu.column_name,
       rc.delete_rule, rc.update_rule
FROM information_schema.referential_constraints rc
JOIN information_schema.key_column_usage kcu
  ON kcu.constraint_name = rc.constraint_name
 AND kcu.constraint_schema = rc.constraint_schema
JOIN information_schema.constraint_column_usage ccu
  ON ccu.constraint_name = rc.constraint_name
 AND ccu.constraint_schema = rc.constraint_schema
WHERE rc.constraint_schema = current_schema()`
	case "mysql", "mariadb":
		return `SELECT kcu.TABLE_NAME, kcu.COLUMN_NAME,
       kcu.REFERENCED_TABLE_NAME, kcu.REFERENCED_COLUMN_NAME,
       rc.DELETE_RULE, rc.UPDATE_RULE
FROM information_schema.KEY_COLUMN_USAGE kcu
JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
  ON rc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
 AND rc.CONSTRAINT_SCHEMA = kcu.CONSTRAINT_SCHEMA
WHERE kcu.REFERENCED_TABLE_NAME IS NOT NULL
  AND kcu.CONSTRAINT_SCHEMA = DATABASE()`
	default:
		return ""
	}
}

// ListFKRules renders every FK edge grouped by its delete behavior,
// flagging CASCADE (silent child destruction) and SET NULL edges.
func (uc *DatabaseUseCase) ListFKRules(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := fkRulesQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("foreign-key rule introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("fk-rules catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing fk-rule rows: %v", closeErr)
		}
	}()

	var rules []fkRule
	for rows.Next() {
		var r fkRule
		if scanErr := rows.Scan(&r.childTable, &r.childColumn,
			&r.parentTable, &r.parentCol, &r.deleteRule, &r.updateRule); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate fk-rule rows: %w", err)
	}
	if len(rules) == 0 {
		return "No foreign keys with referential actions found.", nil
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].childTable != rules[j].childTable {
			return rules[i].childTable < rules[j].childTable
		}
		return rules[i].childColumn < rules[j].childColumn
	})

	var b strings.Builder
	fmt.Fprintf(&b, "%d foreign-key edge(s):\n", len(rules))
	for _, r := range rules {
		switch strings.ToUpper(r.deleteRule) {
		case "CASCADE":
			fmt.Fprintf(&b, "- %s.%s -> %s.%s: ON DELETE CASCADE — deleting a parent row silently deletes all matching children\n",
				r.childTable, r.childColumn, r.parentTable, r.parentCol)
		case "SET NULL":
			fmt.Fprintf(&b, "- %s.%s -> %s.%s: ON DELETE SET NULL — deleting a parent row nulls the child reference\n",
				r.childTable, r.childColumn, r.parentTable, r.parentCol)
		default:
			fmt.Fprintf(&b, "- %s.%s -> %s.%s: ON DELETE %s (delete blocks while children exist)\n",
				r.childTable, r.childColumn, r.parentTable, r.parentCol, r.deleteRule)
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
