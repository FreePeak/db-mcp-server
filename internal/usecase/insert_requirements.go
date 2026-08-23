package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Required-INSERT-columns audit: NOT NULL columns without a DEFAULT are
// exactly the columns an INSERT must supply — the difference between a
// generated INSERT that runs first try and one that fails on constraint
// violations. Built on the metadata DescribeTable already returns, so
// every engine works wherever describe works.

// isRequiredColumn classifies one column's nullability encoding across
// engines (PG "NO", Oracle "N", SQLite pragma notnull=1) against the
// presence of a DEFAULT. Unknown encodings are conservative: not flagged.
func isRequiredColumn(isNullable interface{}, hasDefault bool) bool {
	switch v := isNullable.(type) {
	case string:
		switch strings.ToUpper(strings.TrimSpace(v)) {
		case "NO", "N":
			return !hasDefault
		}
		return false
	case int:
		return v == 1 && !hasDefault
	case int64:
		return v == 1 && !hasDefault
	case bool:
		return v && !hasDefault
	default:
		return false
	}
}

// InsertRequirements renders, per table, the columns an INSERT must
// supply. Tables where everything is defaultable are omitted; a fully
// clean database states so explicitly.
func (uc *DatabaseUseCase) InsertRequirements(ctx context.Context, dbID string) (string, error) {
	info, err := uc.GetDatabaseInfo(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to list tables: %w", err)
	}
	tablesRaw, ok := info["tables"].([]map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no table listing available for %q", dbID)
	}

	type req struct {
		table string
		cols  []string
	}
	var required []req
	scanned := 0
	for _, tr := range tablesRaw {
		tableName := metaString(tr, "table_name")
		if tableName == "" {
			tableName = metaString(tr, "name")
		}
		if tableName == "" || !isPlainIdentifier(tableName) || strings.HasPrefix(tableName, "sqlite_") {
			continue
		}
		desc, derr := uc.DescribeTable(ctx, dbID, tableName)
		if derr != nil {
			continue // unreadable table: skip rather than fail the audit
		}
		colsRaw, _ := desc["columns"].([]map[string]interface{}) //nolint:errcheck // absent columns means nothing to require
		scanned++
		var cols []string
		for _, cr := range colsRaw {
			colName := metaString(cr, "name")
			if colName == "" {
				colName = metaString(cr, "column_name")
			}
			if colName == "" {
				continue
			}
			hasDefault := cr["column_default"] != nil
			if isRequiredColumn(cr["is_nullable"], hasDefault) {
				cols = append(cols, colName)
			}
		}
		sort.Strings(cols)
		if len(cols) > 0 {
			required = append(required, req{tableName, cols})
		}
	}
	sort.Slice(required, func(i, j int) bool { return required[i].table < required[j].table })

	if len(required) == 0 {
		return fmt.Sprintf("No required columns across %d table(s): every table accepts inserts with no explicit values.", scanned), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d of %d table(s) require explicit values on INSERT:\n", len(required), scanned)
	for _, r := range required {
		fmt.Fprintf(&b, "- %s: %s\n", r.table, strings.Join(r.cols, ", "))
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
