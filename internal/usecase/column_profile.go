package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Column profiling: null density, cardinality, range, and top values for
// one column — the facts an agent needs to spot enum-like columns, join
// keys, or mostly-empty fields before writing queries. Portable SQL only.

// validateIdentifier rejects empty or quote-bearing identifiers up front;
// quoting handles the rest.
func validateIdentifier(name string) error {
	if strings.TrimSpace(name) == "" || strings.ContainsAny(name, `";`) {
		return fmt.Errorf("invalid identifier %q", name)
	}
	return nil
}

// ProfileColumn renders a one-column statistical profile.
func (uc *DatabaseUseCase) ProfileColumn(ctx context.Context, dbID, table, column string) (string, error) {
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

	q := quoteIdent(table)
	col := quoteIdent(column)

	single := func(sel string) (string, error) {
		rows, err := db.Query(ctx, "SELECT "+sel+" FROM "+q)
		if err != nil {
			return "", err
		}
		defer func() {
			if closeErr := rows.Close(); closeErr != nil {
				logger.Error("error closing profile rows: %v", closeErr)
			}
		}()
		vals := make([]interface{}, 1)
		ptrs := []interface{}{&vals[0]}
		if !rows.Next() {
			return "NULL", rows.Err()
		}
		if err := rows.Scan(ptrs...); err != nil {
			return "", err
		}
		if vals[0] == nil {
			return "NULL", nil
		}
		if b, ok := vals[0].([]byte); ok {
			return string(b), nil
		}
		return fmt.Sprintf("%v", vals[0]), nil
	}

	total, err := single("COUNT(*)")
	if err != nil {
		return "", fmt.Errorf("profile failed: %w", err)
	}
	nulls, err := single(fmt.Sprintf("COUNT(*) - COUNT(%s)", col))
	if err != nil {
		return "", fmt.Errorf("profile failed: %w", err)
	}
	distinct, err := single(fmt.Sprintf("COUNT(DISTINCT %s)", col))
	if err != nil {
		return "", fmt.Errorf("profile failed: %w", err)
	}
	minV, minErr := single(fmt.Sprintf("MIN(%s)::text", col))
	maxV, maxErr := single(fmt.Sprintf("MAX(%s)::text", col))
	if minErr != nil || maxErr != nil {
		// ::text cast is Postgres-only; retry portable form.
		minV, minErr = single(fmt.Sprintf("MIN(%s)", col))
		if minErr != nil {
			return "", fmt.Errorf("profile failed: %w", minErr)
		}
		maxV, maxErr = single(fmt.Sprintf("MAX(%s)", col))
		if maxErr != nil {
			return "", fmt.Errorf("profile failed: %w", maxErr)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Profile of %s.%s\n", table, column)
	fmt.Fprintf(&b, "rows: %s\nnull_count: %s\ndistinct: %s\nmin: %s\nmax: %s\n",
		total, nulls, distinct, minV, maxV)

	rows, err := db.Query(ctx,
		fmt.Sprintf("SELECT %s, COUNT(*) AS n FROM %s GROUP BY %s ORDER BY n DESC LIMIT 3", col, q, col))
	if err == nil {
		b.WriteString("top_values:\n")
		vals := make([]interface{}, 2)
		ptrs := []interface{}{&vals[0], &vals[1]}
		for rows.Next() {
			if scanErr := rows.Scan(ptrs...); scanErr != nil {
				break
			}
			v := "NULL"
			if vals[0] != nil {
				if bs, ok := vals[0].([]byte); ok {
					v = string(bs)
				} else {
					v = fmt.Sprintf("%v", vals[0])
				}
			}
			fmt.Fprintf(&b, "  %v: %v\n", v, vals[1])
		}
		if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing top-value rows: %v", cerr)
		}
	} else {
		b.WriteString("top_values: unavailable\n")
	}
	return b.String(), nil
}
