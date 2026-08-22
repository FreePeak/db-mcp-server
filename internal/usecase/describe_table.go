package usecase

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// plainIdentifierRe accepts schema-qualified identifiers only, so catalog
// queries interpolating the table name cannot be turned into injection.
var plainIdentifierRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*(\.[A-Za-z_][A-Za-z0-9_$]*)?$`)

func isPlainIdentifier(s string) bool { return plainIdentifierRe.MatchString(s) }

// DescribeTable returns column metadata, index definitions, and a row
// estimate for one table, using engine-appropriate catalog queries.
// Agents previously could only list table names; per-column inspection is
// required for accurate SQL generation against large schemas.
func (uc *DatabaseUseCase) DescribeTable(ctx context.Context, dbID, table string) (map[string]interface{}, error) {
	table = strings.TrimSpace(table)
	if table == "" {
		return nil, fmt.Errorf("table parameter must not be empty")
	}
	if !isPlainIdentifier(table) {
		return nil, fmt.Errorf("table parameter must be a plain (schema-qualified) table name")
	}

	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database type: %w", err)
	}

	columns, err := uc.queryTableMetadata(ctx, dbID, columnQueries(strings.ToLower(dbType), table))
	if err != nil {
		return nil, fmt.Errorf("failed to describe columns: %w", err)
	}

	indexes, err := uc.queryTableMetadata(ctx, dbID, indexQueries(strings.ToLower(dbType), table))
	if err != nil {
		return nil, fmt.Errorf("failed to describe indexes: %w", err)
	}

	// Row estimate is best-effort; absence does not fail the describe.
	rowEstimate := ""
	if v, estErr := uc.queryScalar(ctx, dbID, rowEstimateQuery(strings.ToLower(dbType), table)); estErr == nil {
		rowEstimate = v
	}

	return map[string]interface{}{
		"database": dbID,
		"dbType":   dbType,
		"table":    table,
		"columns":  columns,
		"indexes":  indexes,
		"rowCount": rowEstimate,
	}, nil
}

// queryTableMetadata runs the first successful catalog query from candidates
// and normalizes rows into maps.
func (uc *DatabaseUseCase) queryTableMetadata(ctx context.Context, dbID string, candidates []string) ([]map[string]interface{}, error) {
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for _, q := range candidates {
		rows, err := db.Query(ctx, q)
		if err != nil {
			lastErr = err
			continue
		}

		out, convErr := rowsToMaps(rows)
		closeErr := rows.Close()
		if convErr != nil {
			return nil, convErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return out, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no catalog query configured")
	}
	return nil, lastErr
}

func (uc *DatabaseUseCase) queryScalar(ctx context.Context, dbID, query string) (string, error) {
	out, err := uc.ExecuteQuery(ctx, dbID, query, nil)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(out)
	for _, f := range fields {
		if isNumericField(f) {
			return f, nil
		}
	}
	return "", nil
}

func isNumericField(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return true
}

func rowsToMaps(rows interface {
	Columns() ([]string, error)
	Next() bool
	Scan(dest ...interface{}) error
}) ([]map[string]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]interface{}{}
	values := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range cols {
		ptrs[i] = &values[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]interface{}, len(cols))
		for i, c := range cols {
			v := values[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			m[c] = v
		}
		out = append(out, m)
	}
	return out, nil
}

// columnQueries returns engine-appropriate catalog queries, best first.
func columnQueries(dbType, table string) []string {
	switch dbType {
	case "postgres", "timescale", "timescaledb":
		return []string{fmt.Sprintf(`SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_name = '%s' ORDER BY ordinal_position`, table)}
	case "mysql":
		return []string{fmt.Sprintf("SHOW COLUMNS FROM `%s`", table)}
	case "sqlite", "sqlite3":
		return []string{fmt.Sprintf("SELECT name AS column_name, type AS data_type, \"notnull\" AS is_nullable, dflt_value AS column_default FROM pragma_table_info('%s')", table)}
	case "oracle":
		return []string{fmt.Sprintf(`SELECT column_name, data_type, nullable AS is_nullable, data_default AS column_default FROM user_tab_columns WHERE table_name = UPPER('%s') ORDER BY column_id`, table)}
	default:
		return []string{fmt.Sprintf(`SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_name = '%s' ORDER BY ordinal_position`, table)}
	}
}

func indexQueries(dbType, table string) []string {
	switch dbType {
	case "postgres", "timescale", "timescaledb":
		return []string{fmt.Sprintf(`SELECT indexname AS index_name, indexdef AS definition FROM pg_indexes WHERE tablename = '%s'`, table)}
	case "mysql":
		return []string{fmt.Sprintf("SHOW INDEX FROM `%s`", table)}
	case "sqlite", "sqlite3":
		return []string{
			fmt.Sprintf("SELECT name AS index_name, sql AS definition FROM sqlite_master WHERE type='index' AND tbl_name='%s'", table),
		}
	case "oracle":
		return []string{fmt.Sprintf(`SELECT index_name, '' AS definition FROM user_indexes WHERE table_name = UPPER('%s')`, table)}
	default:
		return []string{fmt.Sprintf(`SELECT indexname AS index_name, indexdef AS definition FROM pg_indexes WHERE tablename = '%s'`, table)}
	}
}

func rowEstimateQuery(dbType, table string) string {
	switch dbType {
	case "postgres", "timescale", "timescaledb":
		return fmt.Sprintf(`SELECT reltuples::bigint AS n FROM pg_class WHERE relname = '%s'`, table)
	case "mysql":
		return fmt.Sprintf("SELECT table_rows AS n FROM information_schema.tables WHERE table_name = '%s'", table)
	case "sqlite", "sqlite3":
		return fmt.Sprintf("SELECT COUNT(*) AS n FROM `%s`", table)
	case "oracle":
		return fmt.Sprintf(`SELECT num_rows AS n FROM user_tables WHERE table_name = UPPER('%s')`, table)
	default:
		return fmt.Sprintf("SELECT COUNT(*) AS n FROM `%s`", table)
	}
}
