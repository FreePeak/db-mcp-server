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
