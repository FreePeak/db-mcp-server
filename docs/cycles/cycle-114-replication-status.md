# Cycle 114 — Replication Status (performance action=replication_status)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Stale read-replica data looks like "my query returns old rows" and
  nothing in the tool surface could see replication state or lag.
  Confirmed absent; engine-gated catalog read like the other audits.

## Shipped

- `internal/usecase/replication.go`: `ListReplication(ctx, dbID)` —
  Postgres reads `pg_stat_replication` (client_addr, state, sent/replay
  LSNs, replay_lag); MySQL runs `SHOW REPLICA STATUS` (8.0.22+ naming).
  Rows rendered via the shared bounded renderer; zero rows is a finding
  ("No replicas attached…"), SQLite errors "not available".
- Performance tool: new action `replication_status` (both per-db and
  unified constructors) served via new capability interface
  `replicationUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestReplicationCatalog`: pg hits pg_stat_replication/replay_lag;
    mysql hits SHOW REPLICA STATUS; sqlite none.
  - `TestListReplication_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for performance action=replication_status.
- Post-merge: verify npm v1.12.0 + docker tags published.
