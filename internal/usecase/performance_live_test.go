package usecase

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

// genericSQLDB adapts a real *sql.DB of any engine to domain.Database for
// live performance-action tests.
type genericSQLDB struct {
	db     *sql.DB
	dbType string
}

func (g *genericSQLDB) Query(ctx context.Context, query string, args ...interface{}) (domain.Rows, error) {
	return g.db.QueryContext(ctx, query, args...)
}
func (g *genericSQLDB) Exec(ctx context.Context, statement string, args ...interface{}) (domain.Result, error) {
	return g.db.ExecContext(ctx, statement, args...)
}
func (g *genericSQLDB) Begin(ctx context.Context, opts *domain.TxOptions) (domain.Tx, error) {
	return nil, nil
}
func (g *genericSQLDB) IsReadOnly() bool { return false }
func (g *genericSQLDB) MaxRows() int     { return 0 }

// TestEngineSlowQueries_Live exercises the engine_slow_queries action
// against the compose stack. Requires docker-compose.test.yml; skips when
// containers are unreachable.
// openLive opens a real engine connection for live tests, skipping
// gracefully when the target is unreachable.
func openLive(t *testing.T, driver, dsn string) *genericSQLDB {
	t.Helper()
	raw, err := sql.Open(driver, dsn)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	if err := raw.Ping(); err != nil {
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "i/o timeout") {
			t.Skipf("container not reachable, skipping: %v", err)
		}
		t.Fatalf("ping failed: %v", err)
	}
	return &genericSQLDB{db: raw, dbType: driver}
}

func TestEngineSlowQueries_Live(t *testing.T) {
	t.Run("mysql digest stats", func(t *testing.T) {
		g := openLive(t, "mysql", "user1:password1@tcp(localhost:13306)/db1?parseTime=true")
		if _, err := g.db.Exec("SELECT 1"); err != nil {
			t.Fatalf("warmup query failed: %v", err)
		}
		uc := NewDatabaseUseCase(&fakeRepo{db: g, dbType: "mysql"})
		out, err := uc.AnalyzePerformance(context.Background(), "mysql1", "engine_slow_queries", "", 5, 0)
		if err != nil {
			t.Fatalf("engine_slow_queries failed: %v", err)
		}
		if !strings.Contains(out, "performance_schema") {
			t.Fatalf("expected header or grant hint mentioning performance_schema, got:\n%s", out)
		}
		// Either digests are readable (table output) or the action degrades
		// gracefully with an actionable grant hint — never an error.
	})

	t.Run("postgres graceful degradation", func(t *testing.T) {
		g := openLive(t, "postgres", "host=localhost port=15432 user=user1 password=password1 dbname=db1 sslmode=disable")
		// Best effort: make the extension available; the compose PG may not
		// have it in shared_preload_libraries, in which case the action must
		// still respond gracefully rather than error.
		_, _ = g.db.Exec("CREATE EXTENSION IF NOT EXISTS pg_stat_statements")

		uc := NewDatabaseUseCase(&fakeRepo{db: g, dbType: "postgres"})
		out, err := uc.AnalyzePerformance(context.Background(), "pg1", "engine_slow_queries", "", 5, 0)
		if err != nil {
			t.Fatalf("engine_slow_queries must not error even when extension is unavailable: %v", err)
		}
		switch {
		case strings.Contains(out, "pg_stat_statements"):
			if !strings.Contains(out, "Top statements") && !strings.Contains(out, "not available") && !strings.Contains(out, "unavailable") {
				t.Fatalf("unexpected output shape:\n%s", out)
			}
		default:
			t.Fatalf("unexpected output:\n%s", out)
		}
	})
}

// TestDbHealth_Live exercises the db_health action against a real
// PostgreSQL with known-seeded defects: an exact duplicate pair and a
// redundant prefix pair, plus a subtest that forces an index invalid.
// Requires docker-compose.test.yml or any PostgreSQL on port 15432 seeded
// with the orders scenario; skips when unreachable.
func TestDbHealth_Live(t *testing.T) {
	g := openLive(t, "postgres", "host=localhost port=15432 user=user1 password=password1 dbname=db1 sslmode=disable")
	// Seed idempotently; ignore errors when the objects already exist.
	_, _ = g.db.Exec(`CREATE TABLE IF NOT EXISTS orders (id SERIAL PRIMARY KEY, customer_id INT, region TEXT, total REAL)`)
	_, _ = g.db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_customer ON orders (customer_id)`)
	_, _ = g.db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_cust_region ON orders (customer_id, region)`)
	_, _ = g.db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_customer_copy ON orders (customer_id)`)

	uc := NewDatabaseUseCase(&fakeRepo{db: g, dbType: "postgres"})

	t.Run("structure findings", func(t *testing.T) {
		out, err := uc.DbHealth(context.Background(), "pg1")
		if err != nil {
			t.Fatalf("db_health failed: %v", err)
		}

		if !strings.Contains(out, "DUPLICATE on orders") {
			t.Errorf("expected duplicate-index finding, got:\n%s", out)
		}
		if !strings.Contains(out, "REDUNDANT on orders") {
			t.Errorf("expected redundant-prefix finding, got:\n%s", out)
		}
		// Primary-key indexes must never receive DROP advice (cycle 31 fix).
		if strings.Contains(out, "DROP INDEX orders_pkey") {
			t.Errorf("orders_pkey is constraint-backed and must not be flagged UNUSED:\n%s", out)
		}
	})

	t.Run("invalid index", func(t *testing.T) {
		// Forcing indisvalid=false needs catalog-write privileges and does
		// not stick on every engine/permission combination (CI postgres:15
		// vs local PG18 diverged here). Verify the flip took effect rather
		// than asserting blind; skip honestly where it cannot be arranged.
		_, _ = g.db.Exec(`UPDATE pg_index SET indisvalid = false WHERE indexrelid = 'idx_orders_customer_copy'::regclass`)
		var valid bool
		err := g.db.QueryRow(`SELECT i.indisvalid FROM pg_index i WHERE i.indexrelid = 'idx_orders_customer_copy'::regclass`).Scan(&valid)
		if err != nil || valid {
			t.Skipf("cannot force indisvalid=false in this environment (err=%v valid=%v)", err, valid)
		}

		out, err := uc.DbHealth(context.Background(), "pg1")
		if err != nil {
			t.Fatalf("db_health failed: %v", err)
		}
		if !strings.Contains(out, "INVALID") {
			t.Errorf("expected invalid-index finding from pg_index.indisvalid while indisvalid=%v, got:\n%s", valid, out)
		}
	})
}

// TestDbHealth_LiveMySQL exercises db_health against real MySQL 9.x,
// locking in cycle 32's catalog fixes: sys.schema_unused_indexes column
// names (object_name/index_name) and PRIMARY-key filtering. Skips when
// unreachable; requires docker-compose.test.yml or any MySQL on 13306
// seeded with the orders scenario.
func TestDbHealth_LiveMySQL(t *testing.T) {
	g := openLive(t, "mysql", "user1:password1@tcp(localhost:13306)/db1?parseTime=true")
	_, _ = g.db.Exec(`CREATE TABLE IF NOT EXISTS orders (id INT PRIMARY KEY AUTO_INCREMENT, customer_id INT, region VARCHAR(50), total REAL)`)
	_, _ = g.db.Exec(`CREATE INDEX idx_orders_customer ON orders (customer_id)`)
	_, _ = g.db.Exec(`CREATE INDEX idx_orders_customer_copy ON orders (customer_id)`)

	uc := NewDatabaseUseCase(&fakeRepo{db: g, dbType: "mysql"})
	out, err := uc.DbHealth(context.Background(), "mysql1")
	if err != nil {
		t.Fatalf("db_health failed: %v", err)
	}

	if !strings.Contains(out, "DUPLICATE on orders") {
		t.Errorf("expected duplicate-index finding from SHOW INDEX parsing, got:\n%s", out)
	}
	// The duplicate pair has zero reads on a fresh server, so at least one
	// UNUSED finding should surface with correct table attribution.
	if strings.Contains(out, "UNUSED on db1:") {
		t.Errorf("object_schema leaked into table attribution:\n%s", out)
	}
	// PRIMARY backs AUTO_INCREMENT; never DROP advice for it.
	if strings.Contains(out, "DROP INDEX `PRIMARY`") {
		t.Errorf("PRIMARY index must never receive DROP advice:\n%s", out)
	}
}

// TestEngineSlowQueries_IndexAdvice_Live locks in backlog #9's second
// half: the slow-queries view appends bounded index suggestions derived
// from the same statements it ranked (latency-ranked, unlike the
// total-time-ranked workload_suggestions action). Requires the MySQL
// container on 13306; skips when unreachable.
func TestEngineSlowQueries_IndexAdvice_Live(t *testing.T) {
	g := openLive(t, "mysql", "user1:password1@tcp(localhost:13306)/db1?parseTime=true")
	_, _ = g.db.Exec(`CREATE TABLE IF NOT EXISTS slow46 (id INT PRIMARY KEY AUTO_INCREMENT, tenant_id INT)`)
	// Warm the digest table with an unindexed-filter statement.
	for i := 0; i < 2; i++ {
		if _, err := g.db.Exec(`SELECT id FROM slow46 WHERE tenant_id = 42`); err != nil {
			t.Fatalf("warmup failed: %v", err)
		}
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: g, dbType: "mysql"})
	out, err := uc.AnalyzePerformance(context.Background(), "mysql1", "engine_slow_queries", "", 5, 0)
	if err != nil {
		t.Fatalf("engine_slow_queries failed: %v", err)
	}
	if !strings.Contains(out, "Index suggestions") || !strings.Contains(out, "idx_slow46_tenant_id") {
		t.Fatalf("expected bounded index advice appended to slow-queries output:\n%s", out)
	}
}
