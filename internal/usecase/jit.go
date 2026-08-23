package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// JIT audit: PostgreSQL's default jit=on hands queries crossing
// jit_above_cost (100000) to the LLVM compiler — and for short OLTP
// shapes the compilation overhead routinely exceeds the query itself,
// showing up as mysterious latency spikes on otherwise-cheap plans.
// Analytical warehouses with long-running queries are the case it was
// built for; most OLTP services should disable it.

// jitQuery returns the probe for the toggle, or "" when unsupported.
func jitQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('jit') AS jit`
	default:
		return ""
	}
}

// jitVerdict classifies the toggle; disabled renders "" so reports
// stay actionable.
func jitVerdict(v string) string {
	s := strings.TrimSpace(strings.ToLower(v))
	switch {
	case s == "":
		return "jit is empty or unreadable — verify with SHOW jit."
	case s == "on":
		return "WARNING: jit=on — queries crossing jit_above_cost pay LLVM compilation that can exceed the query itself on OLTP shapes. Fix if the workload is mostly OLTP: ALTER SYSTEM SET jit='off' then SELECT pg_reload_conf(); keep it on only for long analytical queries."
	default:
		return ""
	}
}

// AuditJIT renders whether the JIT compiler is enabled; a disabled
// result is stated explicitly.
func (uc *DatabaseUseCase) AuditJIT(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := jitQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("JIT introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("JIT query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing JIT rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read jit setting: %w", rerr)
		}
		return "", fmt.Errorf("JIT query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan jit setting: %w", scanErr)
	}
	if verdict := jitVerdict(raw); verdict != "" {
		return verdict, nil
	}
	return "JIT compiler disabled (or unrecognized value) — no OLTP compilation overhead.", nil
}
