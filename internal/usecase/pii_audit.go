package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Combined PII audit: merge the name heuristic and the content scan
// into one deduplicated report so an agent asks once and gets both
// signals — columns named like PII carriers and innocently-named
// columns whose sampled values carry PII.

type piiAuditEntry struct {
	table   string
	column  string
	nameCat string // name-heuristic category, "" if none
	content []string
	samples int
}

// AuditPII runs both detectors and renders a merged report sorted by
// table/column. Columns flagged by both appear once with both signals.
func (uc *DatabaseUseCase) AuditPII(ctx context.Context, dbID string, sampleRows int) (string, error) {
	nameFindings, err := uc.FindSensitiveColumns(ctx, dbID)
	if err != nil {
		return "", fmt.Errorf("name scan failed: %w", err)
	}
	contentFindings, err := uc.ScanContentPII(ctx, dbID, sampleRows)
	if err != nil {
		return "", fmt.Errorf("content scan failed: %w", err)
	}

	merged := map[string]*piiAuditEntry{}
	key := func(t, c string) string { return t + "." + c }
	for _, f := range nameFindings {
		k := key(f.Table, f.Column)
		e := merged[k]
		if e == nil {
			e = &piiAuditEntry{table: f.Table, column: f.Column}
			merged[k] = e
		}
		if e.nameCat == "" {
			e.nameCat = f.Category
		}
	}
	for _, f := range contentFindings {
		k := key(f.Table, f.Column)
		e := merged[k]
		if e == nil {
			e = &piiAuditEntry{table: f.Table, column: f.Column}
			merged[k] = e
		}
		e.content = append(e.content, f.Categories...)
		e.samples = f.SamplesScanned
	}

	if len(merged) == 0 {
		return "No PII findings: no suspect column names and no PII-shaped content in sampled values.", nil
	}

	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	fmt.Fprintf(&b, "%d column(s) with potential PII in %s:\n", len(keys), dbID)
	for _, k := range keys {
		e := merged[k]
		var signals []string
		if e.nameCat != "" {
			signals = append(signals, "name suggests "+e.nameCat)
		}
		if len(e.content) > 0 {
			sort.Strings(e.content)
			signals = append(signals, fmt.Sprintf("content matches %s (%d sample(s) scanned)",
				strings.Join(e.content, ", "), e.samples))
		}
		fmt.Fprintf(&b, "- %s.%s: %s\n", e.table, e.column, strings.Join(signals, "; "))
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
