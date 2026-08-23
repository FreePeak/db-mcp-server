package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Connection-saturation check: "FATAL: too many connections already"
// is an incident the tool surface could not see coming — health reports
// the server-side pool, not how close the engine is to its client
// ceiling. One catalog read per engine returns current usage vs the
// configured maximum.

// saturationQuery returns the engine's connections-vs-ceiling SELECT
// (two columns: current, max), or "" when unsupported.
func saturationQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT (SELECT count(*) FROM pg_stat_activity) AS current_connections,
       current_setting('max_connections')::int AS max_connections`
	case "mysql", "mariadb":
		return `SELECT
  (SELECT VARIABLE_VALUE FROM performance_schema.global_status
    WHERE VARIABLE_NAME = 'Threads_connected') AS current_connections,
  (SELECT VARIABLE_VALUE FROM performance_schema.global_variables
    WHERE VARIABLE_NAME = 'max_connections') AS max_connections`
	default:
		return ""
	}
}

// CheckConnectionSaturation renders current engine connections against
// max_connections with a headroom verdict: >=80% is a warning, >=95%
// critical.
func (uc *DatabaseUseCase) CheckConnectionSaturation(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := saturationQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("connection-saturation introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("saturation catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing saturation rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		return "", fmt.Errorf("saturation catalog returned no rows")
	}
	var rawCur, rawMax any
	if err := rows.Scan(&rawCur, &rawMax); err != nil {
		return "", fmt.Errorf("failed to scan saturation row: %w", err)
	}

	cur, cerr := toInt(rawCur)
	maxC, merr := toInt(rawMax)
	if cerr != nil || merr != nil || maxC <= 0 {
		return "", fmt.Errorf("unparseable saturation values (cur=%v max=%v)", rawCur, rawMax)
	}

	pct := cur * 100 / maxC
	switch {
	case pct >= 95:
		return fmt.Sprintf("Connection saturation CRITICAL: %d/%d (%d%%). New clients will be refused.", cur, maxC, pct), nil
	case pct >= 80:
		return fmt.Sprintf("Connection saturation WARNING: %d/%d (%d%%). Investigate idle or leaked connections before exhaustion.", cur, maxC, pct), nil
	default:
		return fmt.Sprintf("Connection saturation healthy: %d/%d (%d%%).", cur, maxC, pct), nil
	}
}

// toInt coerces driver values (int64, string from MySQL status tables).
func toInt(v any) (int, error) {
	switch x := v.(type) {
	case int64:
		return int(x), nil
	case int:
		return x, nil
	case []byte:
		return strconv.Atoi(strings.TrimSpace(string(x)))
	case string:
		return strconv.Atoi(strings.TrimSpace(x))
	default:
		return strconv.Atoi(fmt.Sprintf("%v", v))
	}
}
