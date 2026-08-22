# Cycle 26 — Docs Surface + Schema-Contract Guard (drift bug fixed)

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- Cycles 17–25 shipped three agent-facing capabilities and one operator flag that the README never mentioned — adoption-blocking for a tool whose users discover features through tool listings and docs.
- Nothing locked the advertised schema to the documented surface; cycle 21's per-db patch had silently failed (assertion aborted the write), leaving `query_<db_id>` without `mask_pii` in its schema while unified mode had it.

## Shipped
- README: full tool-table refresh (suggest_indexes actions, describe constraints/FK resolution, schema mermaid ERD, health masking counts); "Testing Without Docker" section with registerdb + env-var auto-detection walkthrough; new flags documented (`-unified-tools`, `-lazy-loading`, `-masking-audit-log`).
- `TestToolSchemas_DocumentedActionsLocked`: JSON-marshals each tool schema and asserts documented parameters/actions exist — docs and code must change together.

## Verification
- Contract test went RED on first run and exposed the real drift bug: per-db query tool missing `mask_pii`. Fixed; all four schema variants now assert clean.
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.
- PR #87 CI: Build & Test ✅, Lint ✅ at last check.

## Fed Forward
- Version bump to 1.12.0 + tag once PR #87 merges.
- Same contract-guard pattern could cover unified-vs-perdb description parity.
