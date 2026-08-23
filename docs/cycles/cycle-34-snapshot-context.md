# Cycle 34 — Snapshot Audit Context + Docs Surface

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- Snapshots without provenance are weak audit artifacts: an operator reviewing `snap_7` needs to know which statement created it. Bytebase's audit entries always carry the SQL; ours now do too.
- README's tool table predated cycles 29–33 (dry_run, risk notices, snapshot actions) — agents and humans discover capabilities through that table.

## Shipped
- `MutationSnapshot.Statement`: originating statement text (first line, 200-char cap via the shared truncateQuery helper) recorded at capture time.
- README tool table: transaction tool documents auto-snapshot + `list_snapshots`/`rollback_snapshot`; execute tool documents `dry_run` and post-execution risk notices.

## Verification
- TDD RED-first: snapshot carries the exact DELETE statement text (failed on missing field, green after).
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- Version bump to 1.12.0 remains queued for PR #87 merge.
- Snapshot listing could paginate if rings grow beyond display comfort.
