package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Query result export: renders results as machine-readable CSV or JSON so
// agents can hand data to other tools without re-parsing tabular text.
// Honors max_rows (engine-side via auto-limit, plus a client-side cap) and
// server-enforced PII masking, same as the default text renderer.

// ExecuteQueryFormat executes a query and renders the result in the
// requested format: "csv" (default; RFC4180) or "json" (array of objects).
func (uc *DatabaseUseCase) ExecuteQueryFormat(ctx context.Context, dbID, query string, params []interface{}, format string) (string, error) {
	switch format {
	case "", "csv", "json", "inserts":
	default:
		return "", fmt.Errorf("unsupported format %q (want \"csv\", \"json\", or \"inserts\")", format)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	if db.IsReadOnly() && IsWriteStatement(query) {
		return "", fmt.Errorf("database %q is configured as read-only; write statements are not allowed via queries", dbID)
	}

	start := time.Now()
	rows, err := db.Query(ctx, uc.autoLimitedQuery(dbID, query, db), params...)
	uc.recordQueryHistory(dbID, query, start, err)
	if err != nil {
		return "", fmt.Errorf("query execution failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing rows: %v", closeErr)
		}
	}()

	maxRows := db.MaxRows()
	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("failed to get column names: %w", err)
	}
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	cellText := func(i int) string {
		v := values[i]
		if v == nil {
			return "NULL"
		}
		if b, ok := v.([]byte); ok {
			return string(b)
		}
		s := fmt.Sprintf("%v", v)
		if db.MaskPII() {
			s = maskPIIInText(s, columns[i])
		}
		return s
	}

	var collected [][]string
	count := 0
	for rows.Next() {
		if maxRows > 0 && count >= maxRows {
			break
		}
		if scanErr := rows.Scan(valuePtrs...); scanErr != nil {
			return "", fmt.Errorf("failed to scan row: %w", scanErr)
		}
		row := make([]string, len(columns))
		for i := range columns {
			row[i] = cellText(i)
		}
		collected = append(collected, row)
		count++
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate rows: %w", err)
	}

	switch format {
	case "json":
		return renderExportJSON(columns, collected)
	case "inserts":
		table, err := tableFromQuery(query)
		if err != nil {
			return "", err
		}
		return renderExportInserts(table, columns, collected), nil
	default:
		return renderExportCSV(columns, collected), nil
	}
}

// renderExportCSV emits RFC4180 CSV: header line then one line per row,
// quoting fields containing commas, quotes, or newlines.
func renderExportCSV(columns []string, rowValues [][]string) string {
	var b strings.Builder
	writeCSVLine(&b, columns)
	for _, row := range rowValues {
		writeCSVLine(&b, row)
	}
	return b.String()
}

func writeCSVLine(b *strings.Builder, fields []string) {
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		if strings.ContainsAny(f, ",\"\n\r") {
			f = `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
		}
		b.WriteString(f)
	}
	b.WriteByte('\n')
}

// renderExportJSON emits a JSON array of column-name → value objects.
// Numeric-looking cells stay numbers so downstream tooling can compute on
// them; everything else is a string.
func renderExportJSON(columns []string, rowValues [][]string) (string, error) {
	out := make([]map[string]interface{}, 0, len(rowValues))
	for _, row := range rowValues {
		obj := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			obj[col] = jsonCellValue(row[i])
		}
		out = append(out, obj)
	}
	data, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("failed to encode JSON: %w", err)
	}
	return string(data), nil
}

func jsonCellValue(s string) interface{} {
	if s == "NULL" {
		return nil
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return n
	}
	return s
}

// insertTableRe extracts the single table a plain SELECT reads from;
// INSERT generation only supports that simple shape.
var insertTableRe = regexp.MustCompile(`(?i)\bfrom\s+([A-Za-z_][\w$]*)`)

func tableFromQuery(query string) (string, error) {
	m := insertTableRe.FindStringSubmatch(query)
	if len(m) < 2 {
		return "", fmt.Errorf("format=inserts needs a simple SELECT ... FROM <table> statement")
	}
	return m[1], nil
}

// renderExportInserts emits one INSERT per row. Numeric cells stay
// unquoted so the generated DML round-trips types; strings are
// single-quote escaped and NULL stays a literal.
func renderExportInserts(table string, columns []string, rowValues [][]string) string {
	var b strings.Builder
	for _, row := range rowValues {
		fmt.Fprintf(&b, "INSERT INTO %s (%s) VALUES (", table, strings.Join(columns, ", "))
		for i, v := range row {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(insertValueLiteral(v))
		}
		b.WriteString(");\n")
	}
	return b.String()
}

func insertValueLiteral(s string) string {
	if s == "NULL" {
		return "NULL"
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
