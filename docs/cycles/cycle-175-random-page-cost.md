# Cycle 175 — random_page_cost Audit (performance action=random_page_cost)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- PostgreSQL's `random_page_cost` default 4.0 prices random page
  reads at four times a sequential read — calibrated for spinning
  disks where seeks dominate. On SSD/NVMe random reads are nearly
  as cheap as sequential ones; the common tuning ~1.1 stops the
  planner from over-discounting index scans vs seq scans.
- Advice is workload/storage-dependent, so the warning explicitly
  conditions on flash-backed storage and names the fix path:
  ALTER SYSTEM SET random_page_cost='1.1' then pg_reload_conf().
- Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/random_page_cost.go`:
  - `randomPageCostQuery` — current_setting probe; postgres only.
  - `randomPageCostVerdict` — pure classifier: <4.0 → "" (already
    tuned); ==4.0 → WARNING with spinning-disk origin, conditional
    SSD framing, and the ALTER SYSTEM fix; >4.0 → above-default
    note; ≤0 → unreadable note.
  - `AuditRandomPageCost` — runs the probe, parses float
    (unparseable → unreadable verdict), renders verdict or explicit
    healthy line; unsupported engines get an explicit error.
- Performance tool: new action `random_page_cost` (both per-db and
  unified constructors) served via capability interface
  `randomPageCostUseCase`.

## Verification

- TDD RED first (build fail), then GREEN after two test-side fixes
  (malformed compound assertion; case-sensitive grep of "If storage
  is SSD") — implementation unchanged.
  - `TestRandomPageCostProbe`: probe shape + engine gating.
  - `TestRandomPageCostVerdict`: 1.1/2.0 quiet; 4.0 escalated with
    fix path; 0 unreadable.
  - `TestAuditRandomPageCost_Unsupported`: explicit non-PG error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=random_page_cost.
- Post-merge: verify npm v1.12.0 + docker tags published.
