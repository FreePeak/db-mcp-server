# Cycle 19 — PII Masking at the Tool Boundary

**Status:** ✅ Shipped · **Branch:** hackathon

## Research Findings
- Bytebase's governance moat (identity, masking, review, audit) is absent from every lightweight DB MCP server surveyed (DBHub, Postgres MCP Pro, Toolbox).
- Agents querying through MCP pull raw rows into LLM context — PII leaves the database boundary unredacted. Masking at the result renderer is the cheapest interception point.

## Shipped
- `internal/usecase/pii_mask.go`: two-layer masking — column-name heuristics (`email`, `phone`, `ssn`, `card_number`, `iban`, … → whole-cell redaction) plus content patterns (emails, SSNs, credit cards, phones, IPv4, 19+ digit identifiers). Ordered passes: email → ssn → card → ipv4 → long-number → phone so specific shapes win over loose ones.
- `ExecuteQueryMasked` + shared `renderQueryResults` — the legacy `formatQueryResults` now delegates to one code path (duplicated renderer deleted).
- Query tool (per-db + unified) gains opt-in `mask_pii` boolean; delivery routes via capability detection so existing mocks/providers stay source-compatible.

## Verification
- TDD RED-first: pattern unit tests, sensitive/benign column matrix, end-to-end SQLite masked vs raw, and MCP-layer routing tests proving mask_pii dispatch + legacy fallback.
- Bugs caught by tests: phone pass originally nested inside card regex callback (never ran for non-card values); long-number precedence over phone; RE2 lacks lookahead (cycle-17 lesson re-applied).
- Full suite (9 packages) + vet + golangci-lint green. Zero Docker.

## Fed Forward
- Config-level per-database default (`mask_pii: true` in connection config) would make masking mandatory for prod-like DBs.
- Luhn validation for card candidates reduces false positives on digit-heavy text.
