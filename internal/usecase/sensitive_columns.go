package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Sensitive-column discovery: scan catalog metadata for columns whose names
// look like PII carriers, so operators know what mask_pii should protect.
// Pure name-heuristic analysis — no row data is ever read.

// SensitiveFinding is one flagged column.
type SensitiveFinding struct {
	Table    string `json:"table"`
	Column   string `json:"column"`
	Category string `json:"category"`
}

// sensitivePatterns maps column-name fragments to PII categories. Order
// matters only for specificity; matching is substring-based on the
// lowercased identifier with separators normalized to underscores.
var sensitivePatterns = []struct {
	fragment string
	category string
}{
	{"email", "email"},
	{"e_mail", "email"},
	{"phone", "phone"},
	{"mobile", "phone"},
	{"tel", "phone"},
	{"fax", "phone"},
	{"ssn", "national_id"},
	{"social_security", "national_id"},
	{"national_id", "national_id"},
	{"passport", "national_id"},
	{"credit_card", "card"},
	{"card_number", "card"},
	{"card_no", "card"},
	{"card_", "card"}, // card_token, card_holder, ...
	{"_card", "card"}, // gift_card, debit_card, ...
	{"pan", "card"},
	{"cvv", "card"},
	{"cvc", "card"},
	{"first_name", "personal_name"},
	{"last_name", "personal_name"},
	{"full_name", "personal_name"},
	{"given_name", "personal_name"},
	{"family_name", "personal_name"},
	{"surname", "personal_name"},
	{"display_name", "personal_name"},
	{"nickname", "personal_name"},
	{"birth_date", "date_of_birth"},
	{"date_of_birth", "date_of_birth"},
	{"dob", "date_of_birth"},
	{"birthdate", "date_of_birth"},
	{"street", "address"},
	{"address", "address"}, // covers address_line, street_address, etc.
	{"city", "address"},
	{"zipcode", "address"},
	{"zip_code", "address"},
	{"postal_code", "address"},
	{"postcode", "address"},
	{"iban", "bank_account"},
	{"swift", "bank_account"},
	{"account_number", "bank_account"},
	{"routing_number", "bank_account"},
}

// normalizeColumnName folds common separators to underscores for matching.
func normalizeColumnName(name string) string {
	l := strings.ToLower(name)
	l = strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(l)
	return l
}

// FindSensitiveColumns scans every table's columns and returns the ones whose
// names look like PII carriers, grouped deterministically by table/column.
func (uc *DatabaseUseCase) FindSensitiveColumns(ctx context.Context, dbID string) ([]SensitiveFinding, error) {
	info, err := uc.GetDatabaseInfo(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	tablesRaw, ok := info["tables"].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no table listing available for %q", dbID)
	}

	var findings []SensitiveFinding
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
		colsRaw, _ := desc["columns"].([]map[string]interface{}) //nolint:errcheck // absent columns means nothing to flag
		for _, cr := range colsRaw {
			colName := ""
			for _, k := range []string{"name", "column_name", "COLUMN_NAME"} {
				if v, ok := cr[k].(string); ok && v != "" {
					colName = v
					break
				}
			}
			if colName == "" {
				continue
			}
			norm := normalizeColumnName(colName)
			for _, p := range sensitivePatterns {
				if strings.Contains(norm, p.fragment) {
					findings = append(findings, SensitiveFinding{
						Table:    tableName,
						Column:   colName,
						Category: p.category,
					})
					break // first match wins; most specific listed first
				}
			}
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

// FormatSensitiveColumnsReport renders findings as a compact operator report
// with masking guidance.
func FormatSensitiveColumnsReport(dbID string, findings []SensitiveFinding) string {
	var b strings.Builder
	if len(findings) == 0 {
		fmt.Fprintf(&b, "No PII-suspect columns detected in %s.\n", dbID)
		return b.String()
	}
	fmt.Fprintf(&b, "Sensitive columns in %s (%d found):\n\n", dbID, len(findings))
	currentTable := ""
	for _, f := range findings {
		if f.Table != currentTable {
			currentTable = f.Table
			fmt.Fprintf(&b, "%s:\n", currentTable)
		}
		fmt.Fprintf(&b, "  - %-24s [%s]\n", f.Column, f.Category)
	}
	b.WriteString("\nRecommendation: enable \"mask_pii\": true on this database, or pass mask_pii per query.\n")
	return b.String()
}
