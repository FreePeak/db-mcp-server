package usecase

import (
	"context"
	"strings"
	"testing"
)

// costLiveDB adds the max_query_cost option to a live engine connection.
type costLiveDB struct {
	*genericSQLDB
	limit float64
}

func (c costLiveDB) MaxQueryCost() float64 { return c.limit }

// TestCostGate_Live proves the estimated-cost gate against real planners.
// Skips when the compose PostgreSQL or MySQL containers are unreachable.
func TestCostGate_Live(t *testing.T) {
	cases := []struct {
		name, driver, dsn, dbType, bind string
		seed                            []string
	}{
		{
			name: "postgres", driver: "postgres", dbType: "postgres", bind: "$1",
			dsn: "host=localhost port=15432 user=user1 password=password1 dbname=db1 sslmode=disable",
			seed: []string{
				"DROP TABLE IF EXISTS cost_gate_t",
				"CREATE TABLE cost_gate_t (id INT PRIMARY KEY, v INT)",
				"INSERT INTO cost_gate_t SELECT g, g FROM generate_series(1, 1000) g",
				"ANALYZE cost_gate_t",
			},
		},
		{
			name: "mysql", driver: "mysql", dbType: "mysql", bind: "?",
			dsn: "user1:password1@tcp(localhost:13306)/db1?parseTime=true",
			seed: []string{
				"DROP TABLE IF EXISTS cost_gate_t",
				"CREATE TABLE cost_gate_t (id INT PRIMARY KEY, v INT)",
				"INSERT INTO cost_gate_t (id, v) WITH RECURSIVE seq AS (SELECT 1 AS n UNION ALL SELECT n + 1 FROM seq WHERE n < 1000) SELECT n, n FROM seq",
				"ANALYZE TABLE cost_gate_t",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := openLive(t, c.driver, c.dsn)
			for _, stmt := range c.seed {
				if _, err := g.db.Exec(stmt); err != nil {
					t.Fatalf("seed %q failed: %v", stmt, err)
				}
			}
			t.Cleanup(func() { _, _ = g.db.Exec("DROP TABLE IF EXISTS cost_gate_t") })

			uc := NewDatabaseUseCase(&fakeRepo{db: costLiveDB{genericSQLDB: g, limit: 1000}, dbType: c.dbType})
			ctx := context.Background()

			_, err := uc.ExecuteQueryMasked(ctx, "live", "SELECT a.id FROM cost_gate_t a CROSS JOIN cost_gate_t b CROSS JOIN cost_gate_t c", nil, false)
			if err == nil || !strings.Contains(err.Error(), "exceeds max_query_cost") {
				t.Fatalf("three-way cross join must be denied at limit 1000, got: %v", err)
			}

			if _, err := uc.ExecuteQueryMasked(ctx, "live", "SELECT 1", nil, false); err != nil {
				t.Fatalf("SELECT 1 must be allowed, got: %v", err)
			}

			out, err := uc.ExecuteQueryMasked(ctx, "live", "SELECT * FROM cost_gate_t WHERE id = "+c.bind, []interface{}{5}, false)
			if err != nil {
				t.Fatalf("parametrized query must run, got: %v", err)
			}
			if !strings.Contains(out, "Total rows: 1") {
				t.Fatalf("parametrized query must return one row, got:\n%s", out)
			}
		})
	}
}
