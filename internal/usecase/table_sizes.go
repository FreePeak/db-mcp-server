package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Table size report: live row count per table plus engine-reported disk
// size where the catalog exposes it — capacity overview in one call.

// tableSizeQuery returns (query, hasSize): a catalog SELECT over user
// tables rendering name, estimated/live row count, and byte size when
// the engine tracks it.
func tableSizeQuery(dbType string) (string, bool) {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT relname, GREATEST(n_live_tup, 0), pg_total_relation_size(c.oid)
FROM pg_stat_user_tables st JOIN pg_class c ON c.oid = st.relid ORDER BY 3 DESC`, true
	case "mysql", "mariadb":
		return `SELECT TABLE_NAME, TABLE_ROWS,
COALESCE(DATA_LENGTH + INDEX_LENGTH, 0)
FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE()
AND TABLE_TYPE = 'BASE TABLE' ORDER BY 3 DESC`, true
	case "sqlite":
		return "", false // counted live below; sqlite_master carries no sizes
	default:
		return "", false
	}
}

type sizeRow struct {
	table string
	rows  int64
	bytes int64
}

// TableSizes renders every table with its row count (catalog estimate on
// server engines, exact COUNT(*) on SQLite) and byte size when known,
// heaviest first.
func (uc *DatabaseUseCase) TableSizes(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	hasSize := false

	var rows []sizeRow
	if q, sized := tableSizeQuery(dbType); q != "" {
		hasSize = sized
		rws, qerr := db.Query(ctx, q)
		if qerr != nil {
			return "", fmt.Errorf("size catalog query failed: %w", qerr)
		}
		for rws.Next() {
			var r sizeRow
			if scanErr := rws.Scan(&r.table, &r.rows, &r.bytes); scanErr == nil {
				rows = append(rows, r)
			}
		}
		if cerr := rws.Close(); cerr != nil {
			logger.Error("error closing size rows: %v", cerr)
		}
	} else {
		// Fallback: exact counts via the table listing.
		info, ierr := uc.GetDatabaseInfo(dbID)
		if ierr != nil {
			return "", fmt.Errorf("failed to list tables: %w", ierr)
		}
		tablesRaw, _ := info["tables"].([]map[string]interface{}) //nolint:errcheck // absent listing means empty report
		for _, tr := range tablesRaw {
			name := metaString(tr, "table_name")
			if name == "" {
				name = metaString(tr, "name")
			}
			if name == "" || !isPlainIdentifier(name) || strings.HasPrefix(name, "sqlite_") {
				continue
			}
			crows, qerr := db.Query(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdent(name)))
			if qerr != nil {
				continue
			}
			var n int64
			if crows.Next() {
				_ = crows.Scan(&n) //nolint:errcheck // COUNT(*) always scans
			}
			if cerr := crows.Close(); cerr != nil {
				logger.Error("error closing count rows: %v", cerr)
			}
			rows = append(rows, sizeRow{table: name, rows: n})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].bytes != rows[j].bytes {
			return rows[i].bytes > rows[j].bytes
		}
		return rows[i].rows > rows[j].rows
	})

	var b strings.Builder
	fmt.Fprintf(&b, "%d table(s):\n", len(rows))
	for _, r := range rows {
		if hasSize {
			fmt.Fprintf(&b, "- %s: %d row(s) (~%s)\n", r.table, r.rows, humanBytes(r.bytes))
		} else {
			fmt.Fprintf(&b, "- %s: %d row(s)\n", r.table, r.rows)
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// humanBytes renders a byte count for humans (B/KB/MB/GB/TB).
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
