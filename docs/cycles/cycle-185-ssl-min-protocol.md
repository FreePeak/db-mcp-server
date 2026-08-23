# Cycle 185 — ssl_min_protocol_version Audit (performance action=ssl_min_protocol_version)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `ssl_min_protocol_version` is the floor of TLS versions Postgres
  accepts. TLSv1/TLSv1.1 are deprecated, downgrade-vulnerable, and
  prohibited outright by PCI-DSS; modern servers floor at TLSv1.2
  or TLSv1.3.
- Bigger trap: when `ssl` itself is off every connection is
  plaintext while everyone assumes TLS is on — escalated before
  protocol version even matters, with the hostssl/pg_hba.conf fix
  named (restart required).
- Deprecated-floor fix is reloadable: ALTER SYSTEM +
  pg_reload_conf(), noting old clients will be refused.
- Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/ssl_min_protocol.go`:
  - `sslMinProtocolProbe` — reads both `ssl_min_protocol_version`
    and `ssl`; postgres only.
  - `sslMinProtocolVerdict` — pure classifier: ssl-off warning >
    deprecated-protocol warning > unreadable note; healthy floors
    render "" (audit adds explicit healthy line).
  - `AuditSSLMinProtocol` — runs the probe; case-insensitive "on"
    detection; unsupported engines get an explicit error.
- Performance tool: new action `ssl_min_protocol_version` (both
  per-db and unified constructors) served via capability interface
  `sslMinProtocolUseCase`.

## Verification

- TDD RED first (build fail), GREEN after implementation with no
  test edits needed this cycle.
- Tests: probe reads both settings + engine gating; TLSv1.2/1.3
  quiet; TLSv1 escalated with PCI-DSS + named reloadable fix;
  ssl=off escalated with unencrypted-transit mode; unknown value
  renders unreadable; explicit non-PG unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=ssl_min_protocol_version.
- Post-merge: verify npm v1.12.0 + docker tags published.
