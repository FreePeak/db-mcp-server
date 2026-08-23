# Cycle 111 — Redundant-Index Detection (schema format=redundant_indexes)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `ListUnusedIndexes` needs runtime stats; prefix-redundancy is static
  and works everywhere: a non-unique index whose column list is a
  prefix of a wider sibling does only work the wider index already
  covers — write amplification and disk waste with no read benefit.

## Shipped

- `internal/usecase/redundant_indexes.go`:
  `FindRedundantIndexes(ctx, dbID)` — walks every user table's index
  definitions, parses trailing column lists (`indexColumns`) and the
  UNIQUE marker, flags non-unique indexes covered by a wider
  non-unique sibling on the same table. Unique indexes never flagged
  (they enforce constraints). Deterministic sort by table/index;
  bounded clean state naming scanned-index count; DROP INDEX rendered
  as a candidate for review, with a verify-before-dropping note.
- Helpers: `indexColumns` (definition parser), `coversPrefix`.
- Schema tool: `format: "redundant_indexes"` via capability interface
  `redundantIndexUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestFindRedundantIndexes`: idx_email flagged via idx_email_name;
    uniq_email never flagged even though covered; unrelated and
    single-index tables stay silent.
- `go vet` Printf-directive warning fixed in the test before wiring.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=redundant_indexes.
- Post-merge: verify npm v1.12.0 + docker tags published.
