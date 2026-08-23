# Cycle 151 — Aborted-Connections Audit (performance action=aborted_connections)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Aborted_connects climbing means failed handshakes — auth failures,
  TLS mismatches, or probes from hosts that will hit
  max_connect_errors and get blocked until FLUSH HOSTS.
  Aborted_clients climbing means applications tear connections down
  without QUIT (pools closed hard / timeouts too tight). Cumulative
  counters; ratios against Connections are the signal. Confirmed
  absent from the tool surface.

## Shipped

- `internal/usecase/aborted_conns.go`:
  - `abortedConnsQuery` — Connections + Aborted_clients +
    Aborted_connects from performance_schema; mysql/mariadb only.
  - `abortedConnVerdict` — pure classifier with deliberately loose
    thresholds (≥10% of total): pre-auth failures → WARNING naming
    host-blocking risk and FLUSH HOSTS remedy; unclean client exits →
    NOTE on pool teardown paths. Clean input renders "" so reports
    stay actionable; zero history renders explicitly.
  - `AuditAbortedConnections` — renders verdict or an explicit clean
    line ("Connection health clean: N connections, ...").
- Performance tool: new action `aborted_connections` (both per-db and
  unified constructors) via capability interface `abortedConnsUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestAbortedConnsProbe`: probe shape + engine gating.
  - `TestAbortedConnVerdict`: handshake WARNING at 12% connect
    failures; unclean-exit NOTE at 15%; healthy ratios render empty;
    zero-history still renders.
  - `TestAuditAbortedConns_Unsupported`: explicit error.
- Self-catch during RED→GREEN: test expected non-empty clean verdict
  while the design renders "" for brevity — test aligned to design.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=aborted_connections.
- Post-merge: verify npm v1.12.0 + docker tags published.
