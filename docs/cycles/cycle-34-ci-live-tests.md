# Cycle 34 — Live-Engine Tests in CI

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- The integration-test job in `.github/workflows/go.yml` now runs the live-gated regression tests against its service containers:
  - Extra host port mappings (`15432:5432`, `13306:3306`) so the tests' hardcoded DSNs — which match docker-compose.test.yml — work unchanged; no env-var plumbing needed.
  - A seeding step that creates the orders scenario on both engines via `docker exec`, including the forced-invalid index (`UPDATE pg_index SET indisvalid = false`, possible because POSTGRES_USER is a superuser in the official image) and recursive-CTE row seeding with raised depth.
  - A `Run Live-Engine Regression Tests` step: `go test -count=1 -run '_Live' ./internal/usecase/`.
- MySQL seed made re-run tolerant (`|| echo skipped`) since `CREATE INDEX` has no IF NOT EXISTS on MySQL 8.

## Verification
- Seed SQL validated statement-for-statement against real PostgreSQL 18.6 and MySQL 9.7.1 locally before pushing; the duplicate-index failure mode was observed live, motivating the tolerance fix.
- All three `_Live` tests pass against both engines with zero skips; YAML parses; full suite green uncached; smoke passes.

## Fed Forward
- First CI run will confirm the runner-side behavior (service container discovery by ancestor filter mirrors the existing Oracle steps).
- If CI time becomes a concern, gate the live step behind a path filter or make it non-blocking initially.
