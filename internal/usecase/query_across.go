package usecase

import (
	"context"
	"fmt"
	"strings"
)

// Cross-database fan-out: run one SELECT against several databases and
// render per-database sections — staging-vs-prod spot checks in a single
// call. Failures degrade per database; one bad target never fails the
// batch.

// ExecuteQueryAcross runs the statement on every listed database.
func (uc *DatabaseUseCase) ExecuteQueryAcross(ctx context.Context, query string, dbIDs []string) (string, error) {
	clean := strings.TrimSpace(query)
	if !IsSelectStatement(strings.TrimSpace(stripSQLLiterals(clean))) {
		return "", fmt.Errorf("fan-out requires a SELECT statement")
	}
	if len(dbIDs) == 0 {
		return "", fmt.Errorf("no databases specified")
	}

	var b strings.Builder
	for _, dbID := range dbIDs {
		fmt.Fprintf(&b, "=== [%s] ===\n", dbID)
		out, err := uc.ExecuteQuery(ctx, dbID, clean, nil)
		if err != nil {
			fmt.Fprintf(&b, "error: %v\n\n", err)
			continue
		}
		b.WriteString(strings.TrimRight(out, "\n") + "\n\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
