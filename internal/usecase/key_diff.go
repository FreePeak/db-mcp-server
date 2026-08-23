package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/domain"
	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Key diff: compare the primary-key sets of one table across two
// databases — "which rows exist in prod but not staging?" — so an
// agent can verify a copy/sync actually landed. Counts always render;
// up to 20 example keys per side keep the output bounded.

const keyDiffSample = 20

// loadKeys reads every value of one column into a set.
func loadKeys(ctx context.Context, db domain.Database, table, col string) (map[string]bool, error) {
	rows, err := db.Query(ctx, fmt.Sprintf("SELECT %s FROM %s", quoteIdent(col), quoteIdent(table)))
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing key-diff rows: %v", closeErr)
		}
	}()
	set := map[string]bool{}
	for rows.Next() {
		var v interface{}
		if scanErr := rows.Scan(&v); scanErr != nil {
			continue // unreadable cell: skip rather than fail the diff
		}
		set[fmt.Sprintf("%v", v)] = true
	}
	return set, rows.Err()
}

// DiffKeys renders the primary-key difference of table between dbA and
// dbB: keys only on each side plus the shared count.
func (uc *DatabaseUseCase) DiffKeys(ctx context.Context, dbA, dbB, table string) (string, error) {
	if !isPlainIdentifier(table) || strings.TrimSpace(table) == "" {
		return "", fmt.Errorf("invalid or missing table name %q", table)
	}
	if dbA == dbB {
		return "", fmt.Errorf("source and destination must differ")
	}
	desc, err := uc.DescribeTable(ctx, dbA, table)
	if err != nil {
		return "", fmt.Errorf("failed to describe %s.%s: %w", dbA, table, err)
	}
	conRaw, _ := describeConstraintRows(desc["constraints"])
	pkCol := ""
	for _, c := range conRaw {
		if metaString(c, "constraint_type") == "PRIMARY KEY" && metaString(c, "column_name") != "" {
			pkCol = metaString(c, "column_name")
			break
		}
	}
	if pkCol == "" {
		return "", fmt.Errorf("table %q has no single-column primary key to diff on", table)
	}

	db1, err := uc.repo.GetDatabase(dbA)
	if err != nil {
		return "", fmt.Errorf("failed to get database %s: %w", dbA, err)
	}
	db2, err := uc.repo.GetDatabase(dbB)
	if err != nil {
		return "", fmt.Errorf("failed to get database %s: %w", dbB, err)
	}
	keysA, err := loadKeys(ctx, db1, table, pkCol)
	if err != nil {
		return "", fmt.Errorf("failed to read keys from %s.%s: %w", dbA, table, err)
	}
	keysB, err := loadKeys(ctx, db2, table, pkCol)
	if err != nil {
		return "", fmt.Errorf("failed to read keys from %s.%s: %w", dbB, table, err)
	}

	var onlyA, onlyB []string
	for k := range keysA {
		if !keysB[k] {
			onlyA = append(onlyA, k)
		}
	}
	for k := range keysB {
		if !keysA[k] {
			onlyB = append(onlyB, k)
		}
	}
	sort.Strings(onlyA)
	sort.Strings(onlyB)

	var b strings.Builder
	fmt.Fprintf(&b, "Key diff for %s.%s (%s vs %s), keyed on %s:\n", table, pkCol, dbA, dbB, pkCol)
	if len(onlyA) == 0 && len(onlyB) == 0 {
		fmt.Fprintf(&b, "In sync: %d shared key(s).\n", len(keysA))
		return b.String(), nil
	}
	shared := len(keysA) - len(onlyA)
	fmt.Fprintf(&b, "- %d shared key(s)\n", shared)
	fmt.Fprintf(&b, "- %d key(s) only in %s\n", len(onlyA), dbA)
	fmt.Fprintf(&b, "- %d key(s) only in %s\n", len(onlyB), dbB)
	render := func(label string, keys []string) {
		if len(keys) == 0 {
			return
		}
		shown := keys
		more := ""
		if len(shown) > keyDiffSample {
			shown = shown[:keyDiffSample]
			more = fmt.Sprintf(" …(+%d more)", len(keys)-len(shown))
		}
		fmt.Fprintf(&b, "  %s examples:%s %s\n", label, more, strings.Join(shown, ", "))
	}
	render(fmt.Sprintf("only-in-%s", dbA), onlyA)
	render(fmt.Sprintf("only-in-%s", dbB), onlyB)
	return strings.TrimRight(b.String(), "\n"), nil
}
