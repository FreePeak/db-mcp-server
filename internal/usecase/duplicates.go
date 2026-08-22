package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Duplicate detection: find values of one column that appear more than
// once, with counts and example row ids — the first step of any data-
// cleaning pass. Bounded output (top 20 groups).

// FindDuplicates renders duplicated values in `column` with occurrence
// counts and one example primary-key value per group.
func (uc *DatabaseUseCase) FindDuplicates(ctx context.Context, dbID, table, column string) (string, error) {
	if err := validateIdentifier(table); err != nil {
		return "", err
	}
	if err := validateIdentifier(column); err != nil {
		return "", err
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	pk := "rowid"
	desc, err := uc.DescribeTable(ctx, dbID, table)
	if err == nil {
		conRaw, _ := desc["constraints"].([]map[string]interface{}) //nolint:errcheck // absent constraints means no PK
		for _, cr := range conRaw {
			typ, _ := cr["constraint_type"].(string) //nolint:errcheck // absent means skip
			col, _ := cr["column_name"].(string)     //nolint:errcheck // absent means skip
			if strings.EqualFold(typ, "PRIMARY KEY") && col != "" {
				pk = col
				break
			}
		}
	}

	rows, err := db.Query(ctx, fmt.Sprintf(
		"SELECT %s, COUNT(*) AS n, MIN(%s) AS example FROM %s GROUP BY %s HAVING COUNT(*) > 1 ORDER BY n DESC LIMIT 20",
		quoteIdent(column), quoteIdent(pk), quoteIdent(table), quoteIdent(column)))
	if err != nil {
		return "", fmt.Errorf("duplicate scan failed: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing duplicate rows: %v", cerr)
		}
	}()

	var groups []string
	for rows.Next() {
		val, n, example := interface{}(nil), int64(0), interface{}(nil)
		if scanErr := rows.Scan(&val, &n, &example); scanErr != nil {
			return "", fmt.Errorf("scan failed: %w", scanErr)
		}
		groups = append(groups, fmt.Sprintf("%v: %d occurrence(s), example %s=%v", renderScalar(val), n, pk, renderScalar(example)))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate failed: %w", err)
	}
	if len(groups) == 0 {
		return fmt.Sprintf("No duplicates in %s.%s.", table, column), nil
	}
	return fmt.Sprintf("Duplicate values in %s.%s:\n- %s", table, column, strings.Join(groups, "\n- ")), nil
}

// renderScalar stringifies a scanned cell for report output.
func renderScalar(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	if bs, ok := v.([]byte); ok {
		return string(bs)
	}
	return fmt.Sprintf("%v", v)
}
