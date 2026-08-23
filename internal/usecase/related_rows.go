package usecase

import (
	"context"

	"fmt"
	"github.com/FreePeak/db-mcp-server/internal/domain"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// FK traversal: follow one row's foreign keys to parent rows and list the
// child rows that reference it — "show me what this row relates to" in a
// single call instead of hand-written joins per relationship.

func (uc *DatabaseUseCase) fetchRow(ctx context.Context, dbID, table, col, val string) string {
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return ""
	}
	rows, err := db.Query(ctx,
		fmt.Sprintf("SELECT * FROM %s WHERE %s = ? LIMIT 5", quoteIdent(table), quoteIdent(col)), val)
	if err != nil {
		return ""
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing related-row fetch: %v", closeErr)
		}
	}()
	out, _, err := renderQueryResults(rows, 5, false, VerbosityFull)
	if err != nil {
		return ""
	}
	return out
}

// RelatedRows resolves one row by the table's primary key, follows its
// foreign keys to the parent rows, and lists child rows in other tables
// whose foreign keys point back at it.
func (uc *DatabaseUseCase) RelatedRows(ctx context.Context, dbID, table, keyValue string) (string, error) {
	if err := validateIdentifier(table); err != nil {
		return "", err
	}
	desc, err := uc.DescribeTable(ctx, dbID, table)
	if err != nil {
		return "", fmt.Errorf("failed to describe %q: %w", table, err)
	}

	conRaw, _ := desc["constraints"].([]map[string]interface{}) //nolint:errcheck // absent constraints means none
	pkCol := ""
	type fk struct{ col, refTable, refCol string }
	var outFKs []fk
	for _, cr := range conRaw {
		typ, _ := cr["constraint_type"].(string) //nolint:errcheck // absent means skip
		col, _ := cr["column_name"].(string)     //nolint:errcheck // absent means skip
		switch {
		case strings.EqualFold(typ, "PRIMARY KEY") && col != "":
			pkCol = col
		case strings.EqualFold(typ, "FOREIGN KEY"):
			refTable, _ := cr["referenced_table"].(string) //nolint:errcheck // absent means skip
			refCol, _ := cr["referenced_column"].(string)  //nolint:errcheck // absent means skip
			if col != "" && refTable != "" && refCol != "" {
				outFKs = append(outFKs, fk{col, refTable, refCol})
			}
		}
	}
	if pkCol == "" {
		return "", fmt.Errorf("table %q has no single-column primary key to resolve rows by", table)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Related rows for %s where %s = %q:\n", table, pkCol, keyValue)

	rowOut := uc.fetchRow(ctx, dbID, table, pkCol, keyValue)
	if rowOut == "" {
		return "", fmt.Errorf("no row in %q with %s = %q", table, pkCol, keyValue)
	}

	// Outgoing: look up each referenced row using THIS row's FK value,
	// read back from the rendered row output is unreliable — instead run
	// a targeted scalar fetch per FK column.
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	parents := 0
	for _, k := range outFKs {
		fkVal, err := uc.scalarValue(ctx, db, table, k.col, pkCol, keyValue)
		if err != nil || fkVal == "" {
			continue
		}
		out := uc.fetchRow(ctx, dbID, k.refTable, k.refCol, fkVal)
		if out == "" {
			continue
		}
		parents++
		fmt.Fprintf(&b, "\nparent via %s -> %s.%s (value %s):\n%s", k.col, k.refTable, k.refCol, fkVal, out)
	}
	if parents == 0 {
		b.WriteString("\nno resolvable outgoing foreign keys\n")
	}

	// Incoming: other tables' FK columns referencing this table's PK.
	info, infoErr := uc.GetDatabaseInfo(dbID)
	if infoErr != nil {
		b.WriteString("incoming scan skipped: failed to list tables\n")
		return b.String(), nil
	}
	tablesRaw, ok := info["tables"].([]map[string]interface{})
	if !ok {
		b.WriteString("incoming scan skipped: no table listing\n")
		return b.String(), nil
	}
	children := 0
	for _, tr := range tablesRaw {
		childName := ""
		for _, key := range []string{"name", "table_name", "tableName", "TABLE_NAME"} {
			if v, ok := tr[key].(string); ok && v != "" {
				childName = v
				break
			}
		}
		if strings.TrimSpace(childName) == "" || strings.EqualFold(childName, table) ||
			strings.HasPrefix(childName, "sqlite_") {
			continue
		}
		cdesc, derr := uc.DescribeTable(ctx, dbID, childName)
		if derr != nil {
			continue
		}
		ccon, _ := cdesc["constraints"].([]map[string]interface{}) //nolint:errcheck // absent constraints means none
		for _, cr := range ccon {
			typ, _ := cr["constraint_type"].(string)       //nolint:errcheck // absent means skip
			refTable, _ := cr["referenced_table"].(string) //nolint:errcheck // absent means skip
			if !strings.EqualFold(typ, "FOREIGN KEY") || !strings.EqualFold(refTable, table) {
				continue
			}
			col, _ := cr["column_name"].(string)          //nolint:errcheck // absent means skip
			refCol, _ := cr["referenced_column"].(string) //nolint:errcheck // absent means skip
			if col == "" || refCol == "" {
				continue
			}
			out := uc.fetchRow(ctx, dbID, childName, col, keyValue)
			if out != "" {
				children++
				fmt.Fprintf(&b, "\nchild %s.%s -> %s.%s:\n%s", childName, col, table, refCol, out)
			}
			break
		}
	}
	if children == 0 {
		b.WriteString("no incoming references\n")
	}
	return b.String(), nil
}

// scalarValue reads one column of the row matched on another column.
func (uc *DatabaseUseCase) scalarValue(ctx context.Context, db domain.Database, table, wantCol, keyCol, keyVal string) (string, error) {
	rows, err := db.Query(ctx,
		fmt.Sprintf("SELECT %s FROM %s WHERE %s = ? LIMIT 1",
			quoteIdent(wantCol), quoteIdent(table), quoteIdent(keyCol)), keyVal)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing scalar fetch: %v", closeErr)
		}
	}()
	if !rows.Next() {
		return "", rows.Err()
	}
	var v interface{}
	if err := rows.Scan(&v); err != nil {
		return "", err
	}
	if v == nil {
		return "", nil
	}
	if bs, ok := v.([]byte); ok {
		return string(bs), nil
	}
	return fmt.Sprintf("%v", v), nil
}
