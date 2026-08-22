# Cycle 13 — Health Check Tool (Pool + Engine Stats)

**Status:** ✅ Shipped · **Artifacts:** main (this cycle), `internal/usecase/health.go`

## Research Findings
- DBHub ships an opt-in `health_check` reporting connection-pool state and buffer-cache hit ratio; we surfaced nothing about pool pressure or engine health. Agents diagnosing "why is my query slow" had no way to see pool waits.
- domain.Database intentionally stays minimal, so capabilities were added via optional interfaces (`pingProvider`, `healthProvider`) — zero churn to existing mocks.

## Shipped
- `DatabaseUseCase.HealthCheck(ctx, dbID)`:
  - connectivity probe with ping latency in ms; unhealthy databases return a structured payload instead of an error
  - Go `database/sql` pool snapshot: open/in-use/idle connections, wait count and cumulative wait duration
  - best-effort engine indicators: PostgreSQL buffer-cache hit ratio (`pg_stat_database`), MySQL InnoDB buffer efficiency (`performance_schema.global_status`); failures degrade to notes, never fail the check
- `DatabaseAdapter` exposes `Ping`/`HealthStats` (its wrapped interface gained the two methods).
- New `health` tool type registered per-database and unified; compact text rendering.
- Live verification queries extracted to `TestLiveEngineStats` (auto-skips without containers).

## Verification
- SQLite e2e: healthy=true with timestamp present.
- Failing probe: healthy=false + error detail, no error propagation.
- **Live against docker-compose.test.yml**: both engine-stat queries return real ratio values (Postgres 15, MySQL 8). Learned: go-sql-driver returns ROUND() as float64 — handled in the test formatter.

## Fed Forward
Engine stats are point-in-time; a follow-up could add thresholds/alerting hints. Oracle health equivalents remain open.
