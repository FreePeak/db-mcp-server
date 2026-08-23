# Cycle 59 — generate_schema Tool (Schema-to-Code)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Feature-surface survey (subagent) found the README advertises
  `generate_schema_<db_id>` ("Generate SQL or code from database schema")
  but no Go implementation exists — a documented-but-missing feature, worse
  than a missing one because agents trust the docs.
- Competitor context: schema-to-code generation is standard in DB tooling;
  Postgres MCP Pro differentiates on plan/index tuning instead. Closing the
  doc gap was the highest-leverage pick.
- Other survey gaps queued: data export (CSV), cross-DB schema compare,
  session/lock observability with kill switch.

## Shipped

- `internal/usecase/generate_schema.go`: `GenerateSchemaCode(ctx, dbID,
  target)` — renders every table as `go` structs (`db:"..."` tags,
  initialism casing ID/URL/API..., snake_case type names OrderItems) or
  `typescript` interfaces. Type mapping int/serial→int64, floats→float64,
  bool, blob→[]byte; unknown/text default string. Driven purely by
  introspection; unreadable tables skipped, empty result errors.
- `internal/delivery/mcp/generate_schema_tool.go`: `generate_schema_<db_id>`
  tool (+ unified variant), required `format` param ("go"|"typescript"),
  registered in the factory; `UseCaseProvider` interface extended.

## Verification

- TDD RED first (undefined symbol → build fail), then GREEN through three
  real bugs caught by tests: non-CamelCase type names (order_items →
  Order_items), off-by-one struct padding vs gofmt alignment, missing
  initialism casing (Id vs ID).
- Delivery mocks updated for the new interface method (6 files).
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- Data export action (CSV/JSON dump of query results).
- Session/lock observability + cancel (pg_stat_activity equivalent).
- Cross-DB schema compare (drift check is same-DB only today).
- README row now truthful: tool exists.
