# Governance and production controls

This guide describes the controls the server offers for AI agents that query
production databases. It covers transport and authentication, authorization,
read-only connections, PII masking, query limits and the audit log. It ends
with a worked example for a pilot.

Each control is part of the open-source server. Most controls run only when
you set them in the config file or the environment. Two checks run by default:
Streamable HTTP refuses to start without authentication, and legacy SSE stays
off unless you opt in. Tool scopes apply to every Streamable HTTP call, and the
default scope is `db:read`.

## Overview

The server applies these controls in order for every request:

1. The transport checks who is calling (API key, OIDC token, or a per-agent key).
2. The authorization check decides which scopes and databases the caller may use.
3. The database layer applies read-only, row, cost and timeout limits.
4. The result passes through PII and column masking.
5. The audit log records the statement, its outcome and the caller.

Stdio has no caller identity. Step 1 and the scope and database checks in step 2
do not run for stdio. The database limits, masking and audit log still run.

## Transport and authentication

### Secure transport defaults

- Streamable HTTP refuses to start unless one of these is set: an OIDC issuer,
  `DB_MCP_API_KEY`, `DB_MCP_API_KEY_FILE`, `-api-key` or `DB_MCP_AGENT_KEYS_FILE`.
- Legacy SSE refuses to start unless `-allow-insecure-sse` is passed. It has no
  built-in authentication, and the API key does not apply to it.
- The default bind address is `127.0.0.1` (`-h`).
- The Docker image defaults to Streamable HTTP. It refuses to start without
  authentication.

### Stdio

Stdio connections carry no token. Tool scopes and database allowlists do not
apply to stdio. Audit records from stdio have no principal unless
`DB_MCP_AUDIT_PRINCIPAL` is set (see [Audit log](#audit-log)).

### Shared API key

Set exactly one source:

| Source | Use |
|--------|-----|
| `-api-key` flag | Visible in the process list. The server logs a warning. |
| `DB_MCP_API_KEY` | Environment variable. |
| `DB_MCP_API_KEY_FILE` | Path to a file, for example a Docker or Kubernetes secret. |

Setting more than one source is a startup error. A blank key is a startup
error.

The key file is read once at startup. Its trailing CR and LF are trimmed. An
empty file, a whitespace-only file or a file over 64 KiB is a startup error.
Restart the server to rotate the key.

Clients send the key as `Authorization: Bearer <key>`. The server compares the
key in constant time. Calls made with this key use the principal `api-key`
and the scopes in `DB_MCP_API_SCOPES` (default `db:read`). The shared key has
no database allowlist.

### OIDC bearer tokens

Set both `DB_MCP_OIDC_ISSUER` and `DB_MCP_OIDC_AUDIENCE`. The audience is
required when the issuer is set. The server:

- runs OIDC discovery at startup and exits if discovery fails,
- verifies each token's signature, expiry, issuer and audience (the `aud`
  claim must contain `DB_MCP_OIDC_AUDIENCE`),
- requires a non-empty `sub` claim, which becomes the principal,
- reads scopes from the claims `scope`, `scp`, `permissions` and `db_scopes`.
  Each claim may be a space-separated string or an array of strings.

When an issuer is set, OIDC replaces the shared key. `DB_MCP_API_KEY` is then
not used for requests.

Every request must carry every scope listed in `DB_MCP_API_SCOPES` (default
`db:read`). A token that lacks one of them gets `403`.

Example: an OIDC configuration for a pilot.

```bash
DB_MCP_OIDC_ISSUER=https://idp.example.com/realms/agents
DB_MCP_OIDC_AUDIENCE=<client-id>
DB_MCP_API_SCOPES=db:read
```

### Protected resource metadata

Set `DB_MCP_RESOURCE_URL` to the absolute `http` or `https` URL of the `/mcp`
endpoint. It requires `DB_MCP_OIDC_ISSUER`. Startup fails without the issuer or
when the URL is not an absolute `http` or `https` URL.

With the variable set, the server does two things.

- It serves the RFC 9728 metadata document at
  `/.well-known/oauth-protected-resource`. When the resource URL has a path, the
  document is also served at that path appended to the well-known prefix. The
  document is outside the authentication check. It lists `resource`,
  `authorization_servers` (the issuer), `scopes_supported` (`db:read`,
  `db:write`, `db:admin`) and `bearer_methods_supported` (`header`).
- It adds a `WWW-Authenticate` challenge to `401` and `403` responses from
  `/mcp`. The challenge is `Bearer resource_metadata="<metadata-url>"` with
  `scope="..."` set to the scopes in `DB_MCP_API_SCOPES` (default `db:read`).
  On `403` it adds `error="insufficient_scope"`.

Without `DB_MCP_RESOURCE_URL`, `/mcp` sends no `resource_metadata` challenge.

### Per-agent keys file

Give each agent its own key, identity, scopes and database allowlist. Set
`DB_MCP_AGENT_KEYS_FILE` to a JSON list. Only the SHA-256 digest of each key is
stored in the file.

```json
[
  {
    "id": "analytics-agent",
    "key_sha256": "<sha256-hex>",
    "scopes": ["db:read"],
    "databases": ["analytics"]
  },
  {
    "id": "ops-agent",
    "key_sha256": "<sha256-hex>",
    "scopes": ["db:read", "db:write"],
    "databases": ["*"]
  }
]
```

Generate a digest from the key you give the agent:

```bash
printf '%s' "$KEY" | shasum -a 256 | cut -d' ' -f1
```

Rules:

- `id` is required. It must not have leading or trailing spaces, and it must be
  unique. The audit log records it as the principal.
- `key_sha256` is 64 hex characters. It must be unique and must not be the
  SHA-256 of an empty key.
- `scopes` is required. Each value must be `db:read`, `db:write` or `db:admin`.
- `databases` is required. List database ids from the config, or use `"*"`.
  Matching is exact and case-sensitive.
- The file is read once at startup. It must be a regular file of at most 64 KiB.
  An empty file is an error.
- The parser rejects unknown fields (for example `expires_at`), data after the
  list, and an empty list.
- The file applies to Streamable HTTP only. With another transport the server
  ignores it and logs a warning.
- The file cannot be combined with `DB_MCP_OIDC_ISSUER`. Startup fails.
- The file requires `-unified-tools`, whatever its `databases` values are,
  including `"*"`. Startup fails without it. See
  [Database allowlists](#database-allowlists).
- `DB_MCP_API_KEY` or `DB_MCP_API_KEY_FILE` may be set as well. That legacy key
  still works. It uses the scopes in `DB_MCP_API_SCOPES` and has no database
  restriction, so it may also call per-database tool names.
- Startup errors name the entry by its `id` (or by its position in the list when
  the `id` is missing). They never show the key.

An entry may also set `max_query_cost`, a per-key cost ceiling. See [Query limits](#query-limits).

### OIDC roles file

Map one token claim to grants. Set `DB_MCP_OIDC_ROLES_FILE` to a JSON document.
It requires `DB_MCP_OIDC_ISSUER`. Startup fails without it.

```json
{
  "claim": "groups",
  "roles": [
    {
      "value": "analytics-readers",
      "scopes": ["db:read"],
      "databases": ["analytics"],
      "max_query_cost": 50000
    },
    {
      "value": "platform-admins",
      "scopes": ["db:read", "db:write"],
      "databases": ["*"]
    }
  ]
}
```

Rules:

- The top-level `claim` is required. The claim may be a string or an array. The
  server splits strings on whitespace.
- The first role in file order whose value appears in the claim applies. Put
  the most specific role first.
- A token that matches no role gets no scopes and no databases. Every call is
  denied.
- Each `value` is required, must be unique and must not contain whitespace.
- Each role needs `scopes` and `databases`. `max_query_cost` is optional and
  must not be negative.
- When the roles file is set, the token's own scope claims are ignored, and the
  transport does not require the `DB_MCP_API_SCOPES` scopes on each request.
- The roles file requires `-unified-tools`, whatever its `databases` values are,
  including `"*"`. Startup fails without it.

## Authorization

### Tool scopes

Every tool call needs a scope. The scope depends on what the call does.

| Scope | Allows |
|-------|--------|
| `db:read` | Read calls: schema, describe, health, list_databases and read-only queries. |
| `db:write` | Write calls: execute, transaction, and queries whose statement is a write. |
| `db:admin` | Every tool. |

A scope does not include another scope. A token with `db:write` alone cannot
run a read call. The full table of tools and their scopes is in
[Tool scopes](../README.md#tool-scopes) in the README.

How the query tool picks its scope:

- The query tool and the explain tool classify the SQL or statement with the
  write check. A statement that passes the check needs `db:read`. Any other
  statement needs `db:write`.
- A missing or blank statement needs `db:write`.
- The execute and transaction tools always need `db:write`.
- An unknown tool name needs `db:write`.
- Arguments that are not a JSON object are rejected before the scope check.

The write check is a text classifier, not a SQL parser. It is best effort.
Connect read-scoped clients with a database account that has only the
privileges they need.

A denied call returns an error to the client. The message is
`the authenticated principal lacks the required database scope`. The database
is not reached. The denial is written to the audit log (see [Audit log](#audit-log)).

### Database allowlists

An allowlist limits an agent key to some databases.

The same allowlist rules apply to OIDC roles. An OIDC token with no roles file has no allowlist.

- A principal with an allowlist may call only the unified tools: `query`,
  `execute`, `transaction`, `performance`, `explain`, `describe`, `health`,
  `schema` and `filter_tables`. Each call must name a `database` argument on
  the allowlist. TimescaleDB tools (`timescaledb_*`) are not in that list, so
  they are denied.
- Per-database tool names such as `query_<id>` are denied.
- `"*"` grants every database. It does not lift the unified-tools rule: a key
  with `["*"]` also calls only the unified tools, with a `database` argument.
- `list_databases` is always allowed and still needs `db:read`. Its output lists
  only the databases on the allowlist.
- `tools/list` still names every configured database id, in the description of
  each unified tool and of its `database` parameter. An allowlisted key can
  therefore see database names, but its calls are restricted to its allowlist.
  Do not treat database ids as secret.
- A call to a database outside the allowlist is refused with
  `the authenticated principal is not allowed to use this database`. The
  database is not reached. The refusal is written to the audit log.
- Principals without an allowlist (stdio and the shared key) are not restricted
  by database.
- The agent keys file and any roles file require `-unified-tools`, including
  for `"*"`.

## Read-only connections

Set `"read_only": true` on a database. The server then enforces read-only in
several places.

- The query tool refuses write statements before it reaches the database.
- The execute tool refuses every statement on a read-only database.
- A transaction started with `begin` in write mode is refused. A write
  statement inside a transaction is refused.
- PostgreSQL and TimescaleDB: the database session is read-only. The connection
  sets `default_transaction_read_only=on`, and this stays in effect when the
  connection also sets its own `options`. Each query also runs in a read-only
  transaction that is rolled back.
- MySQL: sessions open with `transaction_read_only=1`.
- SQLite: the file opens with `mode=ro`.
- Oracle has no session read-only switch. The connector checks the privileges of
  each new session. It refuses a read-only database whose credentials hold a
  write privilege. If the check cannot run, the session is refused.

Read-only is defense in depth. The text check is best effort. The engine
settings are session defaults, not a security boundary on their own. The
strong control is a database account that has only the SELECT privileges the
agent needs.

## PII masking

Two settings mask query results before they leave the server: column rules
(`masking_rules`) and content rules (`mask_pii`). Both are set by the operator
in the config. They apply to every query result the server formats, including
the catalog queries that the explain, describe, performance and health tools
send. The per-query `mask_pii` parameter applies to the query tool only.

### Column rules: `masking_rules`

Each rule has a `pattern` (a regex matched against the result column name the
database returns), a `strategy` and optional settings.

| Strategy | Effect |
|----------|--------|
| `fixed_string` | Replaces the value with `value`. |
| `null` | Replaces the value with null. |
| `partial` | Keeps the last `keep_last` characters visible. Shorter values are fully masked. |

For each result column, the first rule whose pattern matches the column name
applies. An invalid regex or an unknown strategy is a startup error. The server
does not parse SQL, so rules match names only.

### Content rules: `mask_pii`

`"mask_pii": true` does two things.

- Columns whose names contain a fixed fragment such as `email`, `phone`, `ssn`,
  `card_number`, `credit_card` or `iban` are replaced whole. The full fragment
  list is in `internal/usecase/pii_mask.go`.
- Values in every column are scanned for common PII formats. A format is
  replaced when it appears as a whole pattern inside a cell:

  - emails, with `[EMAIL]`
  - phone numbers, with `[PHONE]`
  - card numbers that pass a Luhn check, with `[CREDIT_CARD]`
  - SSN-shaped values (`###-##-####`), with `[SSN]`
  - IPv4 addresses, with `[IP_ADDRESS]`
  - digit runs of 19 or more digits, with `[LONG_NUMBER]`

The operator sets `mask_pii` in the config. A caller cannot turn it off. A query
can only ask for masking on top of it. A missing or non-boolean `mask_pii`
parameter means the caller did not ask.

Limits: detection is best effort. Values split across columns, unusual formats,
names and street addresses are not detected. Masking runs after the database
returns the data. The statement still reads the raw values.

Masking reduces accidental exposure of PII in query results. Where data must
never leave the database, also restrict access with database grants or views.

## Query limits

### Row cap: `max_rows`

`"max_rows": N` truncates a result at N rows. The result ends with a notice:
`Truncated: showing first N rows (max_rows=N)`. `0` means no cap.

On PostgreSQL (including TimescaleDB), MySQL and SQLite, the query tool adds
`LIMIT N+1` where the statement can be rewritten, so the engine stops at the cap
instead of sending every row. Otherwise the result is truncated after N rows on
the client.

### Cost limit: `max_query_cost`

`"max_query_cost": <number>` denies a query whose planner estimate is above the
limit. The unit is planner cost, not time.

- Supported engines: PostgreSQL, TimescaleDB and MySQL. The server refuses to
  start if the limit is set on another engine, or if the value is negative.
- The gate runs on the query tool. It estimates each `SELECT` or `WITH`
  statement with `EXPLAIN` before the statement runs. The estimate uses the
  statement the server will run, including any row cap. The gate's own
  `EXPLAIN` is not written to the audit log.
- PostgreSQL reads the root `Total Cost`. MySQL adds up the `query_cost` values
  in the plan.
- If the estimate is above the limit, the call fails with
  `estimated query cost <n> exceeds max_query_cost <limit> for database <id>`.
- The gate fails closed. If the estimate cannot be obtained or read, the query
  is denied.
- The query tool refuses `EXPLAIN ANALYZE` when a limit applies, because it
  runs the statement.
- Explain with `analyze=true` is estimated first and refused above the limit.
- Other statement types are not gated by this limit.
- `0` (default) means off.

A caller grant from an agent key or an OIDC role can also carry
`max_query_cost`. The effective limit is the smaller non-zero value of the
database limit and the caller limit, and the same check covers explain with
`analyze=true`. On an engine without a cost estimate, such as SQLite or Oracle,
a caller limit denies every `SELECT` and `WITH` for that caller.

### Query timeout: `query_timeout`

`"query_timeout": N` cancels a statement after N seconds. The default is 30
seconds. `-1` disables the timeout. The `QUERY_TIMEOUT_SECONDS` variable fills
connections that do not set their own value. The JSON value takes precedence.

## Audit log

Set `DB_MCP_AUDIT_LOG=/path/audit.jsonl`. The server appends one JSON line per
record. The file is created with mode `0600`. The server opens the file when it
writes the first record. If the file cannot be opened, the server logs a warning
and writes no records. Set `DB_MCP_AUDIT_REQUIRED` to open the file at startup
and make a failure a startup error (see below).

Records cover every statement the server sends through the query, execute and
transaction paths, and every call refused by a tool scope or by an agent key's
database allowlist. Calls that run no SQL write no record. The explain,
describe, performance and health tools send their own SQL through the query
path, so each statement they send is recorded as a `query` or `execute` record.

| Field | Present when | Meaning |
|-------|--------------|---------|
| `ts` | always | UTC time, RFC 3339 with nanoseconds. |
| `op` | always | `query`, `execute`, `denied`, or `tx_*` for each transaction tool action. `denied` marks a call refused before it reached the database. |
| `database` | always | Database id. |
| `statement` | always | Statement text as sent. Literal values appear in it. Bound parameter values are not written. Capped at 10,000 bytes, with `...[truncated]` appended. |
| `duration_ms` | always | Duration in milliseconds, with microsecond precision. |
| `principal` | the caller has one | OIDC `sub`, the agent key `id`, or `api-key` for the shared key. Stdio has none unless `DB_MCP_AUDIT_PRINCIPAL` is set. |
| `session` | the transport has a session ID | MCP session ID (Streamable HTTP). |
| `tool` | the call came through Streamable HTTP | MCP tool name, such as `query`. Stdio and SSE records have none. |
| `rows` | the count is known | Query: rows shown, capped at `max_rows`. Execute: rows affected. Transaction execute: rows affected, counted before the transaction ends, so a later rollback does not change it. |
| `error` | the call failed or was denied | The error text. This includes read-only refusals, cost-limit denials, and the reason for a `denied` record. |

The statement text can contain personal data. Protect the file like any other
log that holds query text.

- `DB_MCP_AUDIT_REQUIRED=true` (or `1`) makes startup fail unless
  `DB_MCP_AUDIT_LOG` is set and the file can be opened.
- `DB_MCP_AUDIT_PRINCIPAL` labels records that have no principal, such as stdio
  records. A principal from a token always wins.

Writes are best effort. A failed write is logged and the query still returns.

## Worked example: read-only analytics pilot

Goal: an analytics agent reads one PostgreSQL database. Its results are
masked, its queries have a cost ceiling, and every statement is logged with the
agent's name.

Config (`/etc/db-mcp/config.json`):

```json
{
  "schema_version": 1,
  "connections": [
    {
      "id": "analytics",
      "type": "postgres",
      "host": "db.example.com",
      "port": 5432,
      "name": "analytics",
      "user": "analytics_reader",
      "password_env": "DB_MCP_ANALYTICS_PASSWORD",
      "read_only": true,
      "max_rows": 1000,
      "query_timeout": 30,
      "max_query_cost": 50000,
      "mask_pii": true,
      "masking_rules": [
        { "pattern": "(?i)(ssn|tax_id)", "strategy": "null" }
      ]
    }
  ]
}
```

Agent keys file (`/etc/db-mcp/agent-keys.json`), with a digest of the key the
agent holds:

```json
[
  {
    "id": "analytics-agent",
    "key_sha256": "<sha256-hex>",
    "scopes": ["db:read"],
    "databases": ["analytics"]
  }
]
```

Start the server:

```bash
export DB_MCP_ANALYTICS_PASSWORD="<from your secret store>"
DB_MCP_AGENT_KEYS_FILE=/etc/db-mcp/agent-keys.json \
DB_MCP_AUDIT_LOG=/var/log/db-mcp/audit.jsonl \
DB_MCP_AUDIT_REQUIRED=true \
./bin/server -t streamable -c /etc/db-mcp/config.json -unified-tools
```

What this gives you:

- The agent presents its key. It has `db:read` on `analytics` only.
- Its query tool calls need `database: "analytics"`. Writes are refused by the
  scope check and by the read-only database.
- Each result is capped at 1,000 rows, masked by shape and by column rule, and
  estimated before it runs against the 50,000 cost ceiling.
- Each statement lands in the audit log with `principal: "analytics-agent"`
  and `tool: "query"`. A refused call is logged with `op: "denied"`.

Put the Streamable endpoint behind a TLS gateway and a private network. The
server does not provide TLS.

An audit line for a query (placeholder values):

```json
{"ts":"2026-11-22T10:15:30.123456789Z","op":"query","database":"analytics","statement":"SELECT region, count(*) FROM orders GROUP BY region","duration_ms":4.218,"principal":"analytics-agent","session":"<session-id>","tool":"query","rows":12}
```
