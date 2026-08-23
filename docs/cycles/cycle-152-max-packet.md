# Cycle 152 — max_allowed_packet Audit (performance action=max_packet)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- max_allowed_packet is the largest single statement the server
  accepts. Too small and large blob writes or big multi-row INSERTs
  fail with "MySQL server has gone away" / "Packet too large" —
  errors that look like network problems but are pure configuration.
  Legacy defaults (1–4 MB) predate modern JSON/blob payloads.
  Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/packet_size.go`:
  - `maxAllowedPacketQuery` — @@GLOBAL.max_allowed_packet;
    mysql/mariadb only.
  - `maxAllowedPacketVerdict` — pure classifier: ≤0/unreadable →
    config-check note; <16MB → WARNING naming 'gone away'/'Packet too
    large' symptoms with SET GLOBAL fix (64MB) plus client-DSN
    reminder; comfortable sizes render "" (audit adds explicit clean
    line). `humanMB` helper for byte rendering.
  - `AuditMaxAllowedPacket` — renders verdict or an explicit healthy
    line; unparseable values log and fall to the unreadable path.
- Performance tool: new action `max_packet` (both per-db and unified
  constructors) served via capability interface `maxPacketUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestMaxPacketProbe`: probe shape + engine gating.
  - `TestMaxPacketVerdict`: 64MB → empty (clean line added by audit);
    1MB → WARNING with "gone away"; 0 → config note.
  - `TestAuditMaxPacket_Unsupported`: explicit error.
- Self-catch during RED→GREEN: test expected non-empty clean verdict
  while the design renders "" for brevity (same pattern as cycle 151)
  — test aligned to design.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=max_packet.
- Post-merge: verify npm v1.12.0 + docker tags published.
