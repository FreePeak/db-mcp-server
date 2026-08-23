# Cycle 105 — Markdown Data Dictionary (schema format=dictionary)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- "Document this database" meant repeated describes hand-assembled
  into a doc. One call now emits the whole thing.

## Shipped

- `internal/usecase/dictionary.go`: `DataDictionary(ctx, dbID)` —
  sorted per-table Markdown sections with a column/type/notes table;
  notes carry PK and resolved FK targets (`FK -> users.id`) from the
  constraint catalog; internal `sqlite_*` tables excluded; describe
  failures degrade to an inline note, never fail the document.
- Schema tool: `format: "dictionary"` routed via capability interface
  `dataDictionaryUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestDictionary`: both table sections present with column rows,
    INTEGER types, PK marker, user_id column; no sqlite_* leakage.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=dictionary.
- Post-merge: verify npm v1.12.0 + docker tags published.
