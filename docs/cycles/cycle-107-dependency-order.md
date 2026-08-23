# Cycle 107 — FK Dependency Order (schema format=dependency_order)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Seeding/truncating a normalized schema in the wrong order violates
  foreign keys. Agents had to eyeball the mermaid graph and sort by
  hand. Kahn's topological order over the FK edges answers it in one
  call, with cycles flagged instead of silently mis-ordered.

## Shipped

- `internal/usecase/dependency_order.go`: `DependencyOrder(ctx, dbID)`
  — walks every user table's constraints, builds parent→child edges,
  renders deterministic (alphabetical tie-break) topological order
  with a "truncate in reverse" note; leftover tables render as a
  circular-reference group with the break-one-FK hint.
- Schema tool: `format: "dependency_order"` via capability interface
  `dependencyOrderUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestDependencyOrder`: users < orders < items positions.
  - `TestDependencyOrder_Cycle`: mutual FKs flagged "Circular".
  - `TestDependencyOrder_Empty`: no-FK database still lists all tables.
- Unused-import build error caught before wiring.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=dependency_order.
- Post-merge: verify npm v1.12.0 + docker tags published.
