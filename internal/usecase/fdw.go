package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Foreign-table (FDW) discovery: postgres_fdw links make remote tables
// look local — a query against one silently crosses to another database,
// which changes both the performance story (network latency, remote load)
// and the security surface (data leaves this instance). Only the SQL/MED
// catalogs reveal which tables are not what they seem.

// foreignTableQuery returns the SELECT joining user-schema foreign
// tables to their servers, or "" when unsupported.
func foreignTableQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT s.srvname AS server,
       COALESCE(s.srvoptions::text, '') AS server_options,
       n.nspname AS schema_name,
       t.relname AS table_name
FROM pg_foreign_table ft
JOIN pg_class t ON t.oid = ft.ftrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
JOIN pg_foreign_server s ON s.oid = ft.ftserver
ORDER BY s.srvname, n.nspname, t.relname`
	default:
		return ""
	}
}

// ListForeignTables renders every FDW server with the local names that
// proxy to it; a clean result is stated explicitly.
func (uc *DatabaseUseCase) ListForeignTables(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := foreignTableQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("foreign-table introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("foreign-table catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing foreign-table rows: %v", closeErr)
		}
	}()

	type server struct {
		options string
		tables  []string
	}
	servers := map[string]*server{}
	var order []string
	for rows.Next() {
		var srvName, options, schema, table string
		if scanErr := rows.Scan(&srvName, &options, &schema, &table); scanErr != nil {
			continue // unscannable row: skip rather than fail the listing
		}
		if servers[srvName] == nil {
			servers[srvName] = &server{options: options}
			order = append(order, srvName)
		}
		servers[srvName].tables = append(servers[srvName].tables,
			fmt.Sprintf("%s.%s", schema, table))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate foreign-table rows: %w", err)
	}

	if len(order) == 0 {
		return "No foreign tables (FDW) configured — every table is stored locally.", nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d FDW server(s) proxy remote data into local table names:\n", len(order))
	for _, name := range order {
		s := servers[name]
		fmt.Fprintf(&b, "\nserver %s (%d table(s))\n", name, len(s.tables))
		if s.options != "" && s.options != "{}" {
			fmt.Fprintf(&b, "  options: %s\n", s.options)
		}
		for _, t := range s.tables {
			fmt.Fprintf(&b, "  - %s\n", t)
		}
	}
	b.WriteString("Queries against these tables read from the REMOTE system transparently.")
	return strings.TrimRight(b.String(), "\n"), nil
}
