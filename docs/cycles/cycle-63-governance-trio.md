# Cycle 63 — Governance Trio Ported (masking, dry_run, snapshots)

**Status:** ✅ Shipped · **Artifacts:** PRs #101, #102, #103 (all merged 2026-10-12/13)

## Objective

The PRD's §4.3 called the hackathon branch's *masking / dry-run / snapshot*
trio "the differentiator cluster and the natural next feature to build", with
the explicit instruction not to merge PR #87 wholesale (28k insertions, a
branch that rewrites files main has since touched) but to port the high-value
items as small, reviewable PRs.

Three PRs, one item each, each rebased onto current main rather than merged
from the branch:

| PR | Ported from | What landed |
|---|---|---|
| #101 | cycles 19 + 21 | Content-based PII masking: `mask_pii` per query, `"mask_pii": true` per connection |
| #102 | cycles 29–31 | `dry_run` pre-flight + post-execution risk advisories, configurable warn threshold |
| #103 | cycles 32–34 | Pre-mutation snapshots + `list_snapshots`/`rollback_snapshot` |

## What the port changed vs the branch

Rebasing onto main was not mechanical — the branch predates the config
canonicalization (#88) and the tool-surface changes, so each port was
re-derived against the current seams:

- **Masking** reached the delivery layer by *capability assertion*
  (`maskedQueryProvider`) instead of widening `UseCaseProvider`, so every
  existing provider and test double kept compiling.
- **dry_run** and **snapshots** use the same pattern (`dryRunCapable`,
  `snapshotCapable`), which is what makes three independent PRs land without
  touching each other's interfaces.
- The branch's `renderQueryResults`/verbosity refactor was **not** ported: the
  masking PR only needed the mask switch, and verbosity modes remain a
  separate backlog item.

## Findings the ports produced

- **A latent bug in the ported masking code**: the loose phone pattern ate
  dates (`2026-08-25`), UUID tails and order numbers. Fixed with a shape check
  (7–15 digits, not a date, not adjacent to hex) rather than shipping a masker
  that mangles ordinary data — a masker that gets switched off protects
  nothing.
- **A pre-existing test-ordering flake**, found when #103's CI run failed on
  `TestWorkloadIndexSuggestions_EndToEnd` while main passed: the
  process-global performance analyzer was never reset, so the composite
  suggestion depended on which tests ran first (~1 run in 8). Fixed in #103
  and verified over 25 uncached runs.

## Verification

Each PR was driven against the built binary over stdio, not just the test
suite:

- masking: two connections to one SQLite file — plain returns the raw email;
  `mask_pii=true`, database-level `mask_pii`, and database-level `mask_pii`
  with an explicit `mask_pii=false` all return `[EMAIL]`/`[PHONE]` (the
  governance property: the agent asked for raw data and did not get it);
- dry_run: `DROP TABLE` reports Critical and leaves the table in place;
  the same statement without `dry_run` executes and carries the risk notice;
- snapshots: `DELETE` reports `snap_1`, `list_snapshots` shows it, rollback
  restores the row; an unbounded `UPDATE` captures and reverses all three
  rows; an unknown id errors clearly.

`go test -short ./...` green across all packages and `golangci-lint` v1.64.8
clean on each PR; all three merged with green CI.

## Backlog Impact

§4.3's high-value cluster is fully ported. The remaining branch items are the
Medium/Low rows (auto-LIMIT injection, verbosity modes, schema-to-code,
query history, live engine ops, schema drift, TARDIS telemetry) — pick them up
on demand, each as its own small PR.

## Fed Forward

- Wire `-risk-warn-at` / a config field to `SetRiskWarnAt` (the threshold is
  configurable in code but has no operator-facing surface yet).
- Durable snapshot spill for large row sets: the ring is in memory.
- npm publishing is still blocked on issue #86 (no `NPM_TOKEN`); Docker Hub
  publishes normally.
