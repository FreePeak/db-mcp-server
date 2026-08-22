# Cycle 21 — Operator-Enforced PII Masking (`mask_pii` Config)

**Status:** ✅ Shipped · **Branch:** hackathon

## Research Findings
- Cycle 19's masking was opt-in per request — an agent could simply not pass `mask_pii`. Bytebase-style governance requires operators, not agents, to control policy.
- Existing guardrail pattern (`read_only`, `max_rows`) already flowed config → engine config → domain interface; masking followed the identical proven path.

## Shipped
- `"mask_pii": true` per-database connection setting: parsed from JSON config, mapped through `buildDatabaseConfig`, surfaced as `Database.MaskPII()` on the pkg/db interface and `domain.Database`.
- **Enforcement semantics:** server config is additive-only. `ExecuteQueryMasked` masks when `requested || db.MaskPII()`; the legacy `ExecuteQuery` path honors server config too, so omitting the parameter cannot bypass governance.
- Delivery layer now always routes through the masked-capable path when available (capability detection keeps legacy mocks working); README documents the flag in the guardrails table.

## Verification
- TDD RED-first at three layers: manager config mapping (all 4 engines), end-to-end SQLite through `LoadConfig`, and use-case enforcement proving agent opt-out loses against server config on both query paths.
- All test doubles updated for the extended interface (fakeDB, sqliteDB, genericSQLDB, timescale MockDB).
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- Luhn validation for card candidates (cycle 19 carry-over) still open.
- Masking audit log: record which queries triggered redactions would complete the governance story.
