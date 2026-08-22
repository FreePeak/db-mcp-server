# Cycle 42 — Operator Surface Polish

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- Cycle 31 fed forward "wire risk_warn_at to a server flag when operator surface is next touched" — cycles 32–41 touched everything else; the flag was the last usecase-only control.
- README schema-tool row predated format=sensitive (cycle 39/41).

## Shipped
- `-risk-warn-at` server flag (low|medium|high|critical, default high) wired through SetRiskWarnAt at startup.
- README: schema-tool row documents format=sensitive merged report semantics; flag table lists -risk-warn-at.

## Verification
- Full suite (9 packages), vet, golangci-lint clean. Flag path exercised via existing SetRiskWarnAt threshold matrix. Zero Docker.

## Fed Forward
- Config-file equivalents for both flags if operators prefer declarative setup.
