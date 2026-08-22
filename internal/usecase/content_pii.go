package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Content-based PII detection: sample rows from every table and run the
// masking content patterns against text values. Catches PII hiding in
// innocently-named columns (notes, payload, description) that name
// heuristics cannot see. Read-only by construction — SELECT with LIMIT only.

// ContentPIIFinding is one column whose sampled values look like they carry PII.
type ContentPIIFinding struct {
	Table          string   `json:"table"`
	Column         string   `json:"column"`
	Categories     []string `json:"categories"`
	SamplesScanned int      `json:"samples_scanned"`
}

// contentCategoryOrder lists pattern checks most-specific-first so one value
// contributes its strongest category.
type contentCheck struct {
	category string
	test     func(string) bool
}

var contentChecks = []contentCheck{
	{"email", func(s string) bool { return emailRe.MatchString(s) }},
	{"ssn", func(s string) bool { return ssnRe.MatchString(s) }},
	{"credit_card", func(s string) bool {
		for _, m := range cardRe.FindAllString(s, -1) {
			digits := len(digitCountRe.FindAllString(m, -1))
			if digits >= 13 && digits <= 19 && luhnValid(m) {
				return true
			}
		}
		return false
	}},
	{"ip_address", func(s string) bool { return ipv4Re.MatchString(s) }},
	{"phone", func(s string) bool {
		// Exclude values already claimed by more specific checks to keep the
		// category list meaningful; loose pattern is fine for detection.
		return !emailRe.MatchString(s) && phoneRe.MatchString(s)
	}},
}

// ScanContentPII samples up to sampleRows rows per table and flags TEXT-ish
// columns whose values match PII content patterns. Columns already flagged by
// the name heuristic are skipped to avoid double-reporting.
func (uc *DatabaseUseCase) ScanContentPII(ctx context.Context, dbID string, sampleRows int) ([]ContentPIIFinding, error) {
	if sampleRows <= 0 {
		sampleRows = 50
	}
	if sampleRows > 500 {
		sampleRows = 500 // bounded: never full-table scans on operator behalf
	}

	info, err := uc.GetDatabaseInfo(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	tablesRaw, ok := info["tables"].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no table listing available for %q", dbID)
	}

	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	var findings []ContentPIIFinding
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
			return nil, fmt.Errorf("describe %q failed: %w", tableName, err)
		}
		colsRaw, _ := desc["columns"].([]map[string]interface{}) //nolint:errcheck // absent columns means nothing scannable

		textCols := map[string]bool{}
		nameFlagged := map[string]bool{}
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
			if isTextualType(colType) {
				textCols[colName] = true
			}
			norm := normalizeColumnName(colName)
			for _, p := range sensitivePatterns {
				if strings.Contains(norm, p.fragment) {
					nameFlagged[colName] = true
					break
				}
			}
		}

		// Sample once per table, evaluate every candidate column per row.
		candidates := make([]string, 0, len(textCols))
		for c := range textCols {
			if !nameFlagged[c] {
				candidates = append(candidates, c)
			}
		}
		sort.Strings(candidates)
		if len(candidates) == 0 {
			continue
		}

		query := fmt.Sprintf("SELECT %s FROM %s LIMIT %d",
			quoteIdentList(candidates), quoteIdent(tableName), sampleRows)
		rows, err := db.Query(ctx, query)
		if err != nil {
			continue // unreadable table: skip, never fail the scan
		}

		scanned := 0
		hits := map[string]map[string]int{} // col -> category -> count
		for rows.Next() {
			values := make([]any, len(candidates))
			ptrs := make([]any, len(candidates))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				break
			}
			scanned++
			for i, c := range candidates {
				v := values[i]
				s, ok := v.(string)
				if !ok {
					if b, isB := v.([]byte); isB {
						s = string(b)
					} else if v != nil {
						s = fmt.Sprintf("%v", v)
					} else {
						continue
					}
				}
				if len(s) < 6 {
					continue // too short to be meaningful PII-bearing text
				}
				for _, cc := range contentChecks {
					if cc.test(s) {
						if hits[c] == nil {
							hits[c] = map[string]int{}
						}
						hits[c][cc.category]++
					}
				}
			}
		}
		if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing content scan rows: %v", cerr)
		}
		if err := rows.Err(); err != nil {
			continue
		}

		for col, cats := range hits {
			categories := make([]string, 0, len(cats))
			for cat, n := range cats {
				if contentThresholdMet(n, scanned) {
					categories = append(categories, cat)
				}
			}
			if len(categories) == 0 {
				continue // every category in this column is below the noise floor
			}
			sort.Strings(categories)
			findings = append(findings, ContentPIIFinding{
				Table: tableName, Column: col,
				Categories: categories, SamplesScanned: scanned,
			})
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Table != findings[j].Table {
			return findings[i].Table < findings[j].Table
		}
		return findings[i].Column < findings[j].Column
	})
	return findings, nil
}

func quoteIdentList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = quoteIdent(n)
	}
	return strings.Join(quoted, ", ")
}

// contentThresholdMet decides whether a category's hit count is signal or
// noise: at least 5% of scanned samples must match (minimum one hit), so a
// single phone-shaped order id in a 100-row sample does not flag a column
// while dense PII columns still do.
func contentThresholdMet(hits, scanned int) bool {
	if hits < 1 {
		return false
	}
	return hits*20 >= scanned
}

// isTextualType conservatively limits scanning to string-like columns.
func isTextualType(t string) bool {
	t = strings.ToLower(t)
	for _, frag := range []string{"text", "char", "varchar", "clob", "string", "json"} {
		if strings.Contains(t, frag) {
			return true
		}
	}
	return t == ""
}
