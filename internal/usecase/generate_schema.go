package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Schema-to-code generation: renders the live schema as Go structs or
// TypeScript interfaces so an LLM can bind application types to real
// columns instead of guessing. Driven entirely by introspection
// (GetDatabaseInfo + DescribeTable), read-only by construction.

type schemaColumn struct {
	name     string
	dbType   string
	goType   string
	tsType   string
	exported string // Go exported field name
}

var goTypeByFrag = []struct {
	frag  string
	goTyp string
}{
	{"int", "int64"},
	{"serial", "int64"},
	{"bool", "bool"},
	{"float", "float64"},
	{"double", "float64"},
	{"decimal", "float64"},
	{"numeric", "float64"},
	{"real", "float64"},
	{"blob", "[]byte"},
	{"bytea", "[]byte"},
	{"date", "string"},
	{"time", "string"},
}

func mapSchemaColumn(name, dbType string) schemaColumn {
	t := strings.ToLower(dbType)
	goType := "string" // default: text-ish or unknown
	for _, m := range goTypeByFrag {
		if strings.Contains(t, m.frag) {
			goType = m.goTyp
			break
		}
	}
	tsType := "string"
	switch goType {
	case "int64":
		tsType = "number"
	case "float64":
		tsType = "number"
	case "bool":
		tsType = "boolean"
	case "[]byte":
		tsType = "Uint8Array | Buffer"
	}
	c := schemaColumn{name: name, dbType: dbType, goType: goType, tsType: tsType}
	c.exported = goFieldName(name)
	return c
}

// goInitialisms lists lowercase fragments rendered in full caps per Go
// naming convention (effective golint).
var goInitialisms = []string{"id", "url", "api", "sql", "db", "http", "json"}

// goFieldName capitalizes the first rune and applies initialism casing so
// the field is exported and idiomatic: id -> ID, sku -> Sku.
func goFieldName(name string) string {
	if name == "" {
		return name
	}
	for _, init := range goInitialisms {
		if name == init {
			return strings.ToUpper(name)
		}
	}
	up := strings.ToUpper(name[:1])
	return up + name[1:]
}

// goTypeName converts a table name to an exported type name:
// order_items -> OrderItems.
func goTypeName(table string) string {
	parts := strings.Split(table, "_")
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(goFieldName(p))
	}
	return b.String()
}

// GenerateSchemaCode renders every table's columns in the requested target
// language ("go" structs with db tags, "typescript" interfaces).
func (uc *DatabaseUseCase) GenerateSchemaCode(ctx context.Context, dbID, target string) (string, error) {
	info, err := uc.GetDatabaseInfo(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to list tables: %w", err)
	}
	tablesRaw, ok := info["tables"].([]map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no table listing available for %q", dbID)
	}
	switch target {
	case "go", "typescript":
	default:
		return "", fmt.Errorf("unsupported target %q (want \"go\" or \"typescript\")", target)
	}

	var tables []string
	colsByTable := map[string][]schemaColumn{}
	for _, tr := range tablesRaw {
		tableName := ""
		for _, k := range []string{"name", "table_name", "tableName", "TABLE_NAME"} {
			if v, ok := tr[k].(string); ok && v != "" {
				tableName = v
				break
			}
		}
		if strings.TrimSpace(tableName) == "" || strings.HasPrefix(tableName, "sqlite_") {
			continue
		}
		desc, err := uc.DescribeTable(ctx, dbID, tableName)
		if err != nil {
			continue // unreadable table: skip rather than fail the batch
		}
		colsRaw, _ := desc["columns"].([]map[string]interface{}) //nolint:errcheck // absent columns means empty struct
		for _, cr := range colsRaw {
			colName := ""
			colType := ""
			for _, k := range []string{"name", "column_name", "COLUMN_NAME"} {
				if v, ok := cr[k].(string); ok && v != "" {
					colName = v
					break
				}
			}
			for _, k := range []string{"type", "data_type", "Type", "DATA_TYPE"} {
				if v, ok := cr[k].(string); ok && v != "" {
					colType = v
					break
				}
			}
			if colName == "" {
				continue
			}
			colsByTable[tableName] = append(colsByTable[tableName], mapSchemaColumn(colName, colType))
			tables = appendUniqueStr(tables, tableName)
		}
	}
	sort.Strings(tables)

	var b strings.Builder
	for _, tbl := range tables {
		typeName := goTypeName(tbl)
		cols := colsByTable[tbl]
		switch target {
		case "go":
			fmt.Fprintf(&b, "type %s struct {\n", typeName)
			width := 0
			for _, c := range cols {
				if len(c.exported) > width {
					width = len(c.exported)
				}
			}
			for _, c := range cols {
				fmt.Fprintf(&b, "\t%-*s %s `db:\"%s\"`\n", width, c.exported, c.goType, c.name)
			}
			b.WriteString("}\n\n")
		case "typescript":
			fmt.Fprintf(&b, "export interface %s {\n", typeName)
			for _, c := range cols {
				fmt.Fprintf(&b, "\t%s: %s;\n", c.name, c.tsType)
			}
			b.WriteString("}\n\n")
		}
	}
	out := b.String()
	if out == "" {
		return "", fmt.Errorf("no tables found for %q", dbID)
	}
	return out, nil
}

func appendUniqueStr(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}
