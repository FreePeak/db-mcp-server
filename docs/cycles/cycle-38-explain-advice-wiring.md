# Cycle 38 — Explain Output Points at Concrete Fixes (backlog #9)

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
Closes backlog #9's loop from plan analysis to fix:
- `ExecuteExplain` now runs the heuristic index advisor over the explained statement and appends its proposals under an `--- Index suggestions (heuristic) ---` separator when actual `CREATE INDEX` DDL is produced. Advisory only — advisor failures never mask or fail the plan output.
- Call-site map confirmed the other two surfaces were already wired (`suggest_indexes` action, `workload_suggestions`); explain was the missing one named in the backlog.

## Design Notes
- The "has proposals" gate is a substring check on `CREATE INDEX` in the advisor's report rather than plumbing structured results through — honest for heuristic advice and avoids reshaping SuggestIndexes' string contract used by three callers.

## Verification
- New integration test locks in both directions on real SQLite: an uncovered predicate column (`region`) surfaces CREATE INDEX advice under the plan; a covered column (`customer_id`, indexed) leaves the plan clean.
- Full suite green uncached; smoke passes; vet/gofmt clean.

## Fed Forward
- The slow-queries action could carry per-statement advice for its top entry; deferred as it multiplies catalog reads per report and explain now covers the interactive path where an agent actually iterates.
