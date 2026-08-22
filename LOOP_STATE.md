# LOOP STATE — Autonomous Build Loop

> **Any agent session reading this file MUST continue the loop. Do not ask
> for permission. Do not summarize and stop. Pick up at NEXT_CYCLE below.**

## Current State

- **NEXT_CYCLE:** 53
- **Branch:** `hackathon` (worktree `.worktrees/hackathon`)
- **Remote:** `origin/hackathon` — push after EVERY cycle
- **PR:** #87 → `main` (CI green; merge = npm/docker release of v1.12.0)
- **Registry:** `docs/cycles/INDEX.md` — single source of truth, update per cycle
- **Completed:** cycles 01–48 (01–16 pre-crash session, 17–48 this loop)

## Per-Cycle Protocol (mandatory)

1. **Research** — fed-forward items from latest `docs/cycles/cycle-NN-*.md`
   ("Fed Forward" section), else fresh competitive scan
2. **Plan** — pick highest-leverage item; write it into the cycle doc
3. **TDD RED** — failing tests FIRST, on real in-memory SQLite
4. **GREEN** — implement minimally
5. **Verify** — `go build ./... && go vet ./... && go test ./...` (9 pkgs)
   + `golangci-lint run`; fix everything
6. **Document** — `docs/cycles/cycle-NN-slug.md` (research/shipped/
   verification/fed-forward) + INDEX.md row
7. **Push** — commit (hooks run gofmt+lint) then `git push`

## Hard Rules

- ❌ NEVER start Docker containers for testing
- ✅ SQLite in-memory (`openSQLiteForTest`) for e2e; cloud DBs via
  `pkg/db` cloud harness (`NEON_DATABASE_URL` env or `cmd/registerdb`)
- ✅ TDD always; suite green always; one doc per cycle always
- ✅ Multiple cycles per response — only stop when the harness forces it

## Open Fed-Forward Threads (seed ideas)

- Durable query-history sink (mirror masking-audit file option)
- Content-PII match thresholds if noisy
- Oracle auto-limit syntax (ROWNUM wrap)
- Engine-aware rewrite estimates via table-size catalogs
- Post-merge: verify npm 1.12.0 + docker tags published
