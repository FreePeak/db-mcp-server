# Cycle 41 — Merged Sensitive Report (Name + Content)

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- Two discovery mechanisms (cycle 39 name heuristics, cycle 40 content sampling) reported through different APIs — agents shouldn't need to know both exist. One report, two evidence classes.

## Shipped
- `format=sensitive` now merges content findings: after the name-based report it appends a "Content-detected columns" section (table.column [categories] (rows sampled)). Content scan is optional-capability detected — providers without sampling keep the original report shape.

## Verification
- TDD RED-first at delivery layer: stub implements both capabilities; assertion requires name section, content section header, and a specific content finding in one payload.
- Test-contract fix: case-insensitive section matching (report capitalizes the heading).
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- Threshold tuning for content matches if noisy on real schemas.
- README schema-tool row should mention merged report semantics.
