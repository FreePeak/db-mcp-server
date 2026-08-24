# Column Masking — Scoping Document

**Status:** Scoped (cycle 35) · **Origin:** backlog #5, "Bytebase differentiator" · **Deferred since:** cycle 21

## Problem

An MCP client that can `query` a database receives raw values for every
column it selects — including PII (emails, phones, national IDs, card
numbers). Read-only mode protects the database from the client; nothing
protects sensitive data from the client. Bytebase's governance story
includes exactly this gap, which is why it was named a differentiator in
the competitive review.

## The Insight That Un-blocks Scoping

Cycle 21 deferred this because "SELECT-target-to-table resolution
requires real SQL parsing." That is true only for **table-qualified**
policies ("mask orders.customer_id"). It is false for **name-based**
policies ("mask any output column matching /email|phone|ssn/"):

- Column names are already available at the enforcement point —
  `domain.Rows.Columns()` inside `ExecuteQuery`
  (internal/usecase/database_usecase.go:260) before
  `formatQueryResults` (:305) renders text.
- Name-based rules apply regardless of query shape: explicit lists,
  aliases, joins, subqueries, even `SELECT *`.
- No parser dependency, no dialect matrix, no alias-resolution bugs.

So v1 ships name/pattern-based result masking with zero SQL parsing.
Table-qualified rules become an optional later phase that may adopt a
parser (vitess sqlparser or pingcap/parser) once name-based value is
proven.

## Design Sketch

**Config** (mirrors existing per-database flags like `read_only`,
`max_rows`, `query_timeout`):

```json
{
  "id": "db1",
  "masking_rules": [
    { "pattern": "(?i)(email)",        "strategy": "fixed_string", "value": "***MASKED***" },
    { "pattern": "(?i)(phone|mobile)", "strategy": "partial",      "keep": "last4" },
    { "pattern": "(?i)(ssn|tax_id)",   "strategy": "null" }
  ]
}
```

Precedence: first matching rule wins; per-database rules only (a global
section can be added later without breaking this shape).

**Strategies (v1):** `fixed_string`, `null`. (`partial`/`hash` are cheap
follow-ups but each needs its own type-coercion tests.)

**Enforcement point:** usecase layer, after rows are scanned and before
formatting — one pass over columns + cells inside the existing result
path. Applies to `ExecuteQuery` only in v1; transaction reads
(`ExecuteTransaction` action=query) get it when v1 proves out, since they
share formatQueryResults downstream.

**Known limitation, documented not hidden:** renaming a column via
alias (`SELECT email AS e`) defeats name matching by construction. This
is inherent to name-based policies; the report should say so where rules
are active (e.g., GetDatabaseInfo exposes `masking_rules_count`).

## Multi-Cycle Breakdown

1. **Cycle A (core):** config parsing + validation for `masking_rules`;
   name/pattern matcher; fixed_string + null strategies; wire into
   ExecuteQuery; unit tests + SQLite e2e test proving masked output.
2. **Cycle B (hardening):** `partial` strategy with type coercion;
   observability (masked-cell count in metadata); README section;
   live-engine gated test using scripts/live-db-setup.sh.
3. **Cycle C (optional):** table-qualified rules; evaluate parser deps
   then decide.

## Non-Goals

Audit logging, role-based access (MCP has no user identity), write-path
masking, engine-native DDM integration (documented pointers instead:
Postgres RLS/views, SQL Server dynamic data masking).

## Risks

- False positives from loose patterns mask legitimate analytics columns — mitigated by requiring explicit config opt-in per database.
- Performance: regex per column per result set is negligible vs network+engine cost, but precompile patterns at config load.
