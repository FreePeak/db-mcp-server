# Cycle 16 — Mermaid ERD via schema Tool

**Status:** ✅ Shipped · **Artifacts:** main (this cycle)

## Research Findings
- After cycle 15 resolved FK targets, every ingredient for a relationship map existed: table list + per-table FK edges. Agents (and humans) read Mermaid natively, and no competitor in the surveyed set emits ERDs from live catalogs.
- The legacy schema tool also carried a "random_string" dummy parameter — replaced with a real `format` parameter.

## Shipped
- `DatabaseUseCase.RelationshipGraph(ctx, dbID)`: iterates tables, describes each, collects FK edges, renders a Mermaid `erDiagram` (`authors ||--o{ books : "author_id"`); graceful note when no FKs exist; skips non-identifier table names defensively.
- `schema_<db_id>` (and unified `schema`) gains `format: "mermaid" | "list" (default)`.

## Verification
- E2E on real in-memory SQLite two-table FK schema asserts the exact edge line. A refactor initially passed the wrong value to the row extractor (whole payload vs `payload["constraints"]`) — caught by the e2e going red, root-caused with targeted debug output, fixed.
- Full suite + golangci-lint green.

## Fed Forward
Cardinality is currently one-to-many by convention; deriving true cardinality from unique constraints on the referenced side is possible later. Composite FKs render one edge per column pair today.
