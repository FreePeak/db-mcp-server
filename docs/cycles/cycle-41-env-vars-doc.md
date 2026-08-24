# Cycle 41 — Environment Variables Documentation (backlog #10 closeout)

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- New `### Environment Variables` section in the README (after Command-Line Options): a table of every env var the server actually reads, sourced from `internal/config/config.go` rather than memory — `CONFIG_PATH`/`DB_CONFIG_FILE`, inline `DB_CONFIG`, `TRANSPORT_MODE`, `SERVER_PORT`, `LOG_LEVEL`, `DISABLE_LOGGING`, the single-database fallback set (`DB_TYPE`…`DB_NAME`), and `QUERY_TIMEOUT_SECONDS`.
- Precedence chain stated once, up front: `.env` → real env vars → JSON config file wins per database.
- Backlog #10 marked fully done.

## Design Notes
- The table documents only variables verified present in config.go this cycle — no aspirational entries.

## Verification
- Docs-only cycle: full suite green, smoke passes, vet/gofmt clean.
