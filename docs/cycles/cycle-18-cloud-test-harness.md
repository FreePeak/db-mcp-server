# Cycle 18 — Cloud-DB Test Harness (Docker-Free Testing)

**Status:** ✅ Shipped · **Branch:** hackathon

## Research Findings
- Free-tier managed DBs in 2026: Neon (0.5GB, scale-to-zero, no card), Supabase (500MB, pauses after 1w idle), Aiven (dedicated VM Postgres+MySQL, always-free), TiDB Cloud Serverless (MySQL-compatible). All speak standard wire protocols → zero driver changes needed.
- Constraint honored: testing never starts Docker containers. Cloud free tiers replace local stacks; suite skips gracefully with zero credentials.

## Shipped
- `pkg/db/cloud_registry.go`: `ParseDSN` accepts `postgres://`/`postgresql://`, `mysql://`, and Go-form `user:pass@tcp(host:port)/db` strings; provider auto-detection (neon/supabase/aiven/tidbcloud/cockroachdb/xata/generic); JSON registry at `.test-cloud-db.json` (0600, gitignored); `ConfigsFromEnv` scans `NEON_DATABASE_URL` / `SUPABASE_DATABASE_URL` / `AIVEN_DATABASE_URL` / `TIDBCLOUD_DATABASE_URL` / `CLOUD_MYSQL_URL` / `DATABASE_URL`.
- `cmd/registerdb`: validates a DSN by live-pinging through our own `pkg/db` layer before saving — one command registers any free cloud DB (`go run ./cmd/registerdb my_neon "postgresql://..."`; `-list` to inspect).
- `TestCloudRegression`: runs the full regression battery (basic query, execute ops, transactions, data types) against every env/registry cloud DB; actionable skip message when none configured.

## Verification
- 13 new tests written RED-first: DSN parsing (Neon/Supabase/TiDB/Aiven shapes + error paths), registry round-trip persistence, provider classification, env detection incl. empty-env silence.
- Full suite green across all 9 packages; golangci-lint clean (fixed errcheck findings in CLI).
- E2E smoke: `registerdb -list` renders table + onboarding hint; `TestCloudRegression` skips with instructions when unconfigured.

## Fed Forward
- Cold-start flake: Neon scales to zero; first connect may need retry/backoff — add warm-up loop if flakes appear.
- Registry stores plaintext DSN by design (local file, gitignored); consider OS keychain later.
