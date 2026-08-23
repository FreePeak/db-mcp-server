# Safe Migration Workflow (Agent-Driven)

A recipe for letting an agent evolve a schema without gambling production:
capture → preview → apply → verify → undo if wrong. Every step maps to a
tool this server already exposes.

## 1. Capture the baseline

```json
{ "action": "capture_schema_snapshot" }   // transaction_<db_id>
```
→ `Schema baseline schema_snap_12 captured: 7 tables.`

## 2. Preview risky statements before running them

```json
{
  "statement": "ALTER TABLE orders ALTER COLUMN status TYPE text",
  "dry_run": true                              // execute_<db_id>
}
```
→ Risk report without execution: kind, level, advisories (rewrite/lock
warnings, data-loss notes). Critical statements like `DROP TABLE` or
`TRUNCATE` are flagged before they can hurt.

## 3. Apply inside a transaction

```json
{ "action": "begin", "readOnly": false }      // transaction_<db_id>
{ "action": "execute", "transactionId": "tx_...", "statement": "..." }
{ "action": "commit", "transactionId": "tx_..." }
```

## 4. Verify what actually moved

```json
{
  "action": "check_schema_drift",
  "baseline_id": "schema_snap_12"             // transaction_<db_id>
}
```
→ Sorted change list: added/removed tables and columns, type changes —
or `No schema drift detected` when the DDL was a no-op.

## 5. Undo data mistakes

Every DELETE/UPDATE automatically captures affected rows before running:

```json
{ "action": "list_snapshots" }                // transaction_<db_id>
{ "action": "rollback_snapshot", "snapshot_id": "snap_34" }
```

DELETEs re-insert the removed rows; UPDATEs restore old values by id.

## Guardrails active throughout

| Layer | Behavior |
|---|---|
| `read_only: true` | Writes refused at application + engine layer |
| `max_rows` | Result truncation with explicit notices |
| `mask_pii: true` | PII redacted from results, agent cannot opt out |
| `verbosity` | Cell truncation / minimal summaries for token budgets |
| dry_run + advisories | Pre-flight risk analysis; post-execution warnings |
| snapshots | Point-in-time row recovery for every mutation |
