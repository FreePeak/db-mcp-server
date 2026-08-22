package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Table row-count compare: for every table the two databases share,
// report both sides' counts and the delta. The cheap first check for
// "did the seed/migration land?" before any row-level diffing.

func (uc *DatabaseUseCase) countRows(ctx context.Context, dbID, table string) (int64, error) {
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return 0, fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdent(table)))
	if err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing count rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var n int64
	if err := rows.Scan(&n); err != nil {
		return 0, err
	}
	return n, rows.Err()
}

// CompareTableCounts renders per-shared-table row counts on both sides
// with signed deltas; one-sided tables are listed without counts.
func (uc *DatabaseUseCase) CompareTableCounts(ctx context.Context, dbIDA, dbIDB string) (string, error) {
	snapA, err := uc.collectSchemaSnapshot(ctx, dbIDA)
	if err != nil {
		return "", err
	}
	snapB, err := uc.collectSchemaSnapshot(ctx, dbIDB)
	if err != nil {
		return "", err
	}

	tables := map[string]bool{}
	for t := range snapA.columns {
		tables[t] = true
	}
	for t := range snapB.columns {
		tables[t] = true
	}
	names := make([]string, 0, len(tables))
	for t := range tables {
		names = append(names, t)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("Row counts: " + dbIDA + " vs " + dbIDB + "\n")
	for _, t := range names {
		_, inA := snapA.columns[t]
		_, inB := snapB.columns[t]
		switch {
		case !inB:
			fmt.Fprintf(&b, "%s: only in %s\n", t, dbIDA)
			continue
		case !inA:
			fmt.Fprintf(&b, "%s: only in %s\n", t, dbIDB)
			continue
		}
		ca, aerr := uc.countRows(ctx, dbIDA, t)
		cb, berr := uc.countRows(ctx, dbIDB, t)
		if aerr != nil || berr != nil {
			fmt.Fprintf(&b, "%s: unreadable (%v / %v)\n", t, aerr, berr)
			continue
		}
		delta := ca - cb
		sign := "+"
		if delta < 0 {
			sign = ""
		}
		fmt.Fprintf(&b, "%s: %d vs %d (%s%d)\n", t, ca, cb, sign, delta)
	}
	return b.String(), nil
}

// CompareTableSamples diffs the first `limit` rows (ordered by the first
// shared column) of one table across two databases and reports rows that
// exist on only one side. Sampled, not exhaustive: good for seed/config
// tables, not for multi-million-row fact tables.
func (uc *DatabaseUseCase) CompareTableSamples(ctx context.Context, dbIDA, dbIDB, table string, limit int) (string, error) {
	if err := validateIdentifier(table); err != nil {
		return "", err
	}
	if limit <= 0 {
		limit = 50
	}
	snapA, err := uc.collectSchemaSnapshot(ctx, dbIDA)
	if err != nil {
		return "", err
	}
	snapB, err := uc.collectSchemaSnapshot(ctx, dbIDB)
	if err != nil {
		return "", err
	}
	colsA, inA := snapA.columns[table]
	_, inB := snapB.columns[table]
	if !inA || !inB {
		return "", fmt.Errorf("table %q must exist in both databases", table)
	}

	shared := make([]string, 0, len(colsA))
	for c := range colsA {
		if _, ok := snapB.columns[table][c]; ok {
			shared = append(shared, c)
		}
	}
	sort.Strings(shared)
	if len(shared) == 0 {
		return "", fmt.Errorf("table %q has no shared columns", table)
	}

	fetch := func(dbID string) (map[string]bool, error) {
		db, err := uc.repo.GetDatabase(dbID)
		if err != nil {
			return nil, fmt.Errorf("failed to get database: %w", err)
		}
		rows, err := db.Query(ctx,
			fmt.Sprintf("SELECT %s FROM %s ORDER BY %s LIMIT %d",
				quoteIdentList(shared), quoteIdent(table), quoteIdent(shared[0]), limit))
		if err != nil {
			return nil, err
		}
		defer func() {
			if closeErr := rows.Close(); closeErr != nil {
				logger.Error("error closing sample rows: %v", closeErr)
			}
		}()
		set := map[string]bool{}
		vals := make([]interface{}, len(shared))
		ptrs := make([]interface{}, len(shared))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		for rows.Next() {
			if scanErr := rows.Scan(ptrs...); scanErr != nil {
				return nil, scanErr
			}
			parts := make([]string, len(shared))
			for i, v := range vals {
				if v == nil {
					parts[i] = "NULL"
				} else if bs, ok := v.([]byte); ok {
					parts[i] = string(bs)
				} else {
					parts[i] = fmt.Sprintf("%v", v)
				}
			}
			set["("+strings.Join(parts, ", ")+")"] = true
		}
		return set, rows.Err()
	}

	setA, err := fetch(dbIDA)
	if err != nil {
		return "", fmt.Errorf("sample read failed on %s: %w", dbIDA, err)
	}
	setB, err := fetch(dbIDB)
	if err != nil {
		return "", fmt.Errorf("sample read failed on %s: %w", dbIDB, err)
	}

	var onlyInA, onlyInB []string
	for k := range setA {
		if !setB[k] {
			onlyInA = append(onlyInA, k)
		}
	}
	for k := range setB {
		if !setA[k] {
			onlyInB = append(onlyInB, k)
		}
	}
	sort.Strings(onlyInA)
	sort.Strings(onlyInB)

	if len(onlyInA) == 0 && len(onlyInB) == 0 {
		return fmt.Sprintf("Sampled data matches: %d row(s) from %s.%s identical on both sides.", len(setA), table, strings.Join(shared, ", ")), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Sampled differences in %s (first %d rows by %s):\n", table, limit, shared[0])
	for _, k := range onlyInA {
		fmt.Fprintf(&b, "- only in %s: %s\n", dbIDA, k)
	}
	for _, k := range onlyInB {
		fmt.Fprintf(&b, "- only in %s: %s\n", dbIDB, k)
	}
	return b.String(), nil
}
