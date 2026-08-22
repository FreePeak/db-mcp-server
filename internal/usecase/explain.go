package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// BuildExplainSQL renders the engine-appropriate EXPLAIN statement.
// analyze requests actual execution with plan statistics where supported
// (PostgreSQL, MySQL); SQLite only supports static plans and Oracle uses
// a two-step EXPLAIN PLAN FOR flow handled by ExecuteExplain.
func BuildExplainSQL(dbType, statement string, analyze bool) string {
	switch dbType {
	case "postgres", "timescale", "timescaledb":
		if analyze {
			return fmt.Sprintf("EXPLAIN (ANALYZE, BUFFERS) %s", statement)
		}
		return fmt.Sprintf("EXPLAIN %s", statement)
	case "mysql":
		if analyze {
			return fmt.Sprintf("EXPLAIN ANALYZE %s", statement)
		}
		return fmt.Sprintf("EXPLAIN %s", statement)
	case "sqlite", "sqlite3":
		// SQLite has no executing variant; QUERY PLAN gives the tree.
		return fmt.Sprintf("EXPLAIN QUERY PLAN %s", statement)
	default:
		if analyze {
			return fmt.Sprintf("EXPLAIN ANALYZE %s", statement)
		}
		return fmt.Sprintf("EXPLAIN %s", statement)
	}
}

// ExecuteExplain returns the execution plan for a statement without
// (normally) executing it. With analyze=true on engines that support it,
// the statement runs and timing/buffer statistics are included — writes
// remain blocked on read_only databases by the shared SQL classifier,
// which scans through the EXPLAIN prefix.
func (uc *DatabaseUseCase) ExecuteExplain(ctx context.Context, dbID, statement string, analyze bool) (string, error) {
	statement = strings.TrimSpace(statement)
	if statement == "" {
		return "", fmt.Errorf("statement parameter must not be empty")
	}

	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	// The classifier sees through the EXPLAIN prefix, so data-modifying
	// statements are refused here exactly as in ExecuteQuery.
	if db.IsReadOnly() && IsWriteStatement(statement) {
		return "", fmt.Errorf("database %q is configured as read-only; write statements are not allowed via explain", dbID)
	}

	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}

	// Oracle requires two steps: populate PLAN_TABLE, then display it.
	// Note EXPLAIN PLAN FOR writes to PLAN_TABLE, so it fails on
	// engine-level read-only sessions — an intentional safety property.
	if dbType == "oracle" {
		if _, err := uc.ExecuteStatement(ctx, dbID, fmt.Sprintf("EXPLAIN PLAN FOR %s", statement), nil); err != nil {
			return "", fmt.Errorf("explain plan failed: %w", err)
		}
		return uc.ExecuteQuery(ctx, dbID, "SELECT * FROM TABLE(DBMS_XPLAN.DISPLAY)", nil)
	}

	explainSQL := BuildExplainSQL(strings.ToLower(dbType), statement, analyze)
	result, err := uc.ExecuteQuery(ctx, dbID, explainSQL, nil)
	if err != nil {
		return "", fmt.Errorf("explain failed: %w", err)
	}
	logger.Info("EXPLAIN executed on %s (%s): analyzed=%v", dbID, dbType, analyze)
	return result, nil
}
