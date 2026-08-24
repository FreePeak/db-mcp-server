package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"

	_ "github.com/lib/pq"
)

// liveTSUseCase is a minimal UseCaseProvider backed by a real *sql.DB so
// read-only TimescaleDB operations can be exercised end to end. Only the
// methods the handlers touch are implemented; the embedded interface keeps
// the rest compile-time satisfied.
type liveTSUseCase struct {
	UseCaseProvider
	db     *sql.DB
	dbType string
}

func (u *liveTSUseCase) GetDatabaseType(dbID string) (string, error) { return u.dbType, nil }
func (u *liveTSUseCase) ListDatabases() []string                     { return nil }
func (u *liveTSUseCase) IsLazyLoading() bool                         { return false }

// renderRows mirrors internal/usecase's query-result rendering convention:
// header, column names, dash rule, tab-joined data rows, "Total rows" footer.
// Handlers rely on substrings of this shape (e.g. extension presence checks).
func renderRows(rows *sql.Rows) string {
	cols, err := rows.Columns()
	if err != nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n", strings.Join(cols, "\t"), strings.Repeat("-", 8*len(cols)))
	vals := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	n := 0
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return "scan error: " + err.Error()
		}
		cells := make([]string, len(vals))
		for i, v := range vals {
			switch x := v.(type) {
			case nil:
				cells[i] = "NULL"
			case []byte:
				cells[i] = string(x)
			default:
				cells[i] = fmt.Sprintf("%v", x)
			}
		}
		b.WriteString(strings.Join(cells, "\t") + "\n")
		n++
	}
	fmt.Fprintf(&b, "\nTotal rows: %d\n", n)
	return b.String()
}

func (u *liveTSUseCase) ExecuteQuery(ctx context.Context, dbID, query string, _ []interface{}) (string, error) {
	rows, err := u.db.QueryContext(ctx, query)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	return renderRows(rows), nil
}

func (u *liveTSUseCase) ExecuteStatement(ctx context.Context, dbID, statement string, _ []interface{}) (string, error) {
	res, err := u.db.ExecContext(ctx, statement)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	return fmt.Sprintf("rows affected: %d\nTotal rows: 1\n", n), nil
}

// openTimescaleLive connects to a real TimescaleDB, skipping gracefully when
// unreachable (same contract as the usecase-package live tests).
func openTimescaleLive(t *testing.T) (*sql.DB, *liveTSUseCase) {
	t.Helper()
	raw, err := sql.Open("postgres", "host=localhost port=15435 user=timescale_user password=timescale_password dbname=timescale_test sslmode=disable")
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	if err := raw.Ping(); err != nil {
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "i/o timeout") {
			t.Skipf("TimescaleDB container not reachable, skipping: %v", err)
		}
		t.Fatalf("ping failed: %v", err)
	}
	seedTimescaleScenario(t, raw)
	return raw, &liveTSUseCase{db: raw, dbType: "postgres"}
}

// seedTimescaleScenario creates (idempotently) the hypertable, rows, and
// continuous aggregate the read-only assertions need. Best-effort DDL:
// anything already present is left alone.
func seedTimescaleScenario(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, stmt := range []string{
		`CREATE EXTENSION IF NOT EXISTS timescaledb`,
		`CREATE SCHEMA IF NOT EXISTS test_data`,
		`CREATE TABLE IF NOT EXISTS test_data.sensor_readings (
			time TIMESTAMPTZ NOT NULL, sensor_id INTEGER NOT NULL,
			temperature DOUBLE PRECISION, humidity DOUBLE PRECISION,
			pressure DOUBLE PRECISION, battery_level DOUBLE PRECISION, location VARCHAR(50))`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed %q failed: %v", stmt, err)
		}
	}
	// Convert to hypertable if not already; ignore when it already is one.
	_, _ = db.Exec(`SELECT create_hypertable('test_data.sensor_readings', 'time', if_not_exists=>TRUE, migrate_data=>TRUE)`)

	var n int
	if err := db.QueryRow(`SELECT count(*) FROM test_data.sensor_readings`).Scan(&n); err == nil && n == 0 {
		_, _ = db.Exec(`INSERT INTO test_data.sensor_readings (time, sensor_id, temperature, humidity)
			SELECT now() - (i || ' hours')::interval, i%3, 20+i%5, 50.0 FROM generate_series(1,48) i`)
	}
	_, _ = db.Exec(`CREATE MATERIALIZED VIEW IF NOT EXISTS test_data.hourly_sensor_stats
		WITH (timescaledb.continuous) AS
		SELECT time_bucket('1 hour', time) AS bucket, sensor_id,
			AVG(temperature) AS avg_temp, MIN(temperature) AS min_temp, MAX(temperature) AS max_temp
		FROM test_data.sensor_readings GROUP BY bucket, sensor_id WITH DATA`)
}

// callReadOnly runs one operation through HandleRequest and returns its
// rendered message+details as text.
func callReadOnly(t *testing.T, uc UseCaseProvider, params map[string]interface{}) string {
	t.Helper()
	request := server.ToolCallRequest{Parameters: params}
	resp, err := NewTimescaleDBTool().HandleRequest(context.Background(), request, "tsdb", uc)
	if err != nil {
		t.Fatalf("operation %v failed: %v", params["operation"], err)
	}
	m, ok := resp.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected response type %T for %v", resp, params["operation"])
	}
	return fmt.Sprintf("%v\n%v", m["message"], m["details"])
}

// TestTimescaleReadOnly_Live exercises all seven registered read-only
// TimescaleDB operations against a real engine — cycle 59's counterpart of
// cycles 31/32/39's live validation, which caught real-SQL bugs mocks miss.
// Requires docker-compose.timescaledb-test.yml or any TimescaleDB on port
// 15435; skips when unreachable.
func TestTimescaleReadOnly_Live(t *testing.T) {
	uc := func() UseCaseProvider {
		_, u := openTimescaleLive(t)
		return u
	}()

	t.Run("list_hypertables", func(t *testing.T) {
		out := callReadOnly(t, uc, map[string]interface{}{"operation": "list_hypertables"})
		if !strings.Contains(out, "sensor_readings") {
			t.Errorf("expected seeded hypertable in listing, got:\n%s", out)
		}
	})

	t.Run("get_compression_settings", func(t *testing.T) {
		out := callReadOnly(t, uc, map[string]interface{}{
			"operation":    "get_compression_settings",
			"target_table": "sensor_readings",
		})
		if !strings.Contains(out, "empty result means compression is not enabled") {
			t.Errorf("expected disclosure header, got:\n%s", out)
		}
	})

	t.Run("get_retention_policy", func(t *testing.T) {
		out := callReadOnly(t, uc, map[string]interface{}{
			"operation":    "get_retention_policy",
			"target_table": "sensor_readings",
		})
		if !strings.Contains(out, "empty result means none is configured") {
			t.Errorf("expected disclosure header, got:\n%s", out)
		}
	})

	t.Run("list_continuous_aggregates", func(t *testing.T) {
		out := callReadOnly(t, uc, map[string]interface{}{"operation": "list_continuous_aggregates"})
		if !strings.Contains(out, "hourly_sensor_stats") {
			t.Errorf("expected seeded continuous aggregate in listing, got:\n%s", out)
		}
	})

	t.Run("get_continuous_aggregate_info", func(t *testing.T) {
		out := callReadOnly(t, uc, map[string]interface{}{
			"operation": "get_continuous_aggregate_info",
			"view_name": "hourly_sensor_stats",
		})
		if !strings.Contains(out, "hourly_sensor_stats") {
			t.Errorf("expected view info row, got:\n%s", out)
		}
	})

	t.Run("time_series_query", func(t *testing.T) {
		out := callReadOnly(t, uc, map[string]interface{}{
			"operation":       "time_series_query",
			"target_table":    "test_data.sensor_readings",
			"time_column":     "time",
			"bucket_interval": "1 hour",
			"aggregations":    "AVG(temperature) AS avg_temp",
			"limit":           float64(10),
		})
		if !strings.Contains(out, "avg_temp") {
			t.Errorf("expected bucketed aggregates in output, got:\n%s", out)
		}
	})

	t.Run("analyze_time_series", func(t *testing.T) {
		out := callReadOnly(t, uc, map[string]interface{}{
			"operation":    "analyze_time_series",
			"target_table": "test_data.sensor_readings",
			"time_column":  "time",
		})
		if strings.TrimSpace(out) == "" {
			t.Errorf("expected non-empty analysis output")
		}
	})
}
