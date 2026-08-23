package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// random_page_cost audit: the PostgreSQL default 4.0 prices random
// page reads at four times a sequential read — calibrated for
// spinning disks where seeks dominate. On SSD/NVMe random reads are
// nearly as cheap as sequential ones, and the common tuning (~1.1)
// stops the planner from over-discounting index scans in favor of
// seq scans. Workload-dependent advice, so the warning is explicit
// that it only applies to flash-backed storage.

const rpcSpinningDiskDefault = 4.0 // PG default; SSD guidance is ~1.1

// randomPageCostQuery returns the probe for the setting, or "" when
// unsupported.
func randomPageCostQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('random_page_cost') AS rpc`
	default:
		return ""
	}
}

// randomPageCostVerdict classifies the setting; already-tuned values
// render "" so reports stay actionable.
func randomPageCostVerdict(v float64) string {
	switch {
	case v <= 0:
		return "random_page_cost is unreadable — verify with SHOW random_page_cost."
	case v < rpcSpinningDiskDefault:
		return "" // already tuned below the spinning-disk default
	case v == rpcSpinningDiskDefault:
		return fmt.Sprintf("WARNING: random_page_cost=%v — still at the spinning-disk default, which makes the planner under-trust index scans vs seq scans. If storage is SSD/NVMe (nearly all modern deployments), ~1.1 reflects real costs: ALTER SYSTEM SET random_page_cost='1.1' then SELECT pg_reload_conf(). Leave it on genuine spinning-disk data drives.", v)
	default:
		return fmt.Sprintf("random_page_cost=%v — above the spinning-disk default; intentional if the storage is slower than disk, otherwise reconsider.", v)
	}
}

// AuditRandomPageCost renders whether the planner's random-I/O cost
// model matches modern flash storage; a tuned result is stated
// explicitly.
func (uc *DatabaseUseCase) AuditRandomPageCost(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := randomPageCostQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("random_page_cost introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("random_page_cost query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing random_page_cost rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read random_page_cost: %w", rerr)
		}
		return "", fmt.Errorf("random_page_cost query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan random_page_cost: %w", scanErr)
	}
	v, perr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if perr != nil {
		v = 0 // unparseable → unreadable verdict
	}
	if verdict := randomPageCostVerdict(v); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("random_page_cost=%.1f — planner cost model matches flash-era I/O.", v), nil
}
