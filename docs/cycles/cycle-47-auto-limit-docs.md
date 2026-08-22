# Cycle 47 — Auto-LIMIT Documentation

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- Cycle 46 shipped injection behavior; the max_rows guardrail row and an explainer were needed for operators to understand the server now touches their SQL (transparently, conservatively).

## Shipped
- README: max_rows row updated with injection note; dedicated "Auto-LIMIT" explainer covering injection examples, existing-LIMIT passthrough, subquery semantics, non-SELECT safety, Oracle exclusion.

## Verification
- Docs-only cycle; build + full suite green. Zero Docker.

## Fed Forward
- None pending from this thread.
