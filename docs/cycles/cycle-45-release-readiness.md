# Cycle 45 — Release Readiness: CHANGELOG Backfill + Stale Backlog Sweep

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- CHANGELOG entries reconstructed from git history (not memory):
  - `[v1.10.0]` (2026-08-22, PR #85 merge): guardrails pack (read-only bypass fix, max_rows, real transactions), PG/MySQL engine-level read-only, explain/describe tools, real performance tool, streamable transport (#34), API-key middleware (#57), prompts fix (#35).
  - `[v1.11.0]`: documented honestly as a same-commit re-tag for distribution-pipeline verification — no functional delta.
  - `[Unreleased]`: everything since (cycles 13–44) grouped into Added/Fixed.
- Stale backlog items closed with evidence:
  - #3: PR #85 was merged 2026-08-22 with green CI (verified via `gh pr view 85`); tags exist; changelog now covers them.
  - #5: column masking fully shipped across cycles 35–39.

## Design Notes
- Both v1.10.0 and v1.11.0 point at commit 326424d; the changelog says so explicitly rather than inventing a delta.

## Verification
- Docs-only cycle; suite green, smoke passes at time of adjacent cycles' pushes.

## Release Note
- HEAD is 41+ commits past v1.11.0. Tagging v1.12.0 is ready whenever maintainers choose — it triggers publish pipelines, so it is left as an explicit decision rather than done autonomously.
