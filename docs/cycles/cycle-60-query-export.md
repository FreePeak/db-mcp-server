# Cycle 60 — Query Export (CSV / JSON)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Survey gap #1 (cycle 59): agents could only get human-tabular text back
  from queries — no machine-readable export, so handing data to other tools
  meant re-parsing decorated output.
- Design choice: inline CSV/JSON rather than server-side file dumps — no
  new filesystem attack surface, agent saves the bytes itself.

## Shipped

- `internal/usecase/query_export.go`: `ExecuteQueryFormat(ctx, dbID,
  query, params, format)` — "csv" (RFC4180 quoting; default for empty
  format) or "json" (array of row objects, numeric cells stay numbers,
  NULL preserved). Honors auto-limit + max_rows cap, read-only enforcement,
  query history, and server-enforced PII masking, same as the text path.
- `internal/delivery/mcp/tool_types.go`: `format` param on the query tool
  (per-db + unified); csv/json requests route through a capability
  interface (`queryExportUseCase`) so existing mocks/providers keep working.

## Verification

- TDD RED first (undefined symbol → build fail), then GREEN:
  - `TestExecuteQueryFormat_CSV`: header row, comma+quote escaping.
  - `TestExecuteQueryFormat_JSON`: per-row objects with numeric types.
  - `TestExecuteQueryFormat_Errors`: unknown format errors, empty = csv.
- `TestQueryTool_FormatRouting` locks the delivery routing (csv → export
  path, empty → text path).
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- Optional `--export-dir` for server-side file dumps with path sandboxing.
- Session/lock observability + query cancel.
- Cross-DB schema compare.
- Post-merge: verify npm v1.12.0 + docker tags published.
