# Cycle 40 — Content-Based PII Detection

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- Name heuristics (cycle 39) miss PII hiding in innocuous columns — `notes`, `payload`, `description` routinely carry emails/phones/cards in free text. MCPg's audit tooling samples content; combining name + content detection is the complete discovery story.

## Shipped
- `ScanContentPII(ctx, dbID, sampleRows)`: per table, samples up to N rows (bounded 1–500, default 50 — never a full-table scan on operator behalf), runs the masking content patterns (email, SSN, Luhn-validated cards, IPv4, phone) against textual columns only (`text`/`char`/`varchar`/`clob`/`json`).
- Columns already flagged by name heuristics are excluded — no double-reporting; unreadable tables skip silently so one bad table never fails the scan.
- Findings carry categories plus `samples_scanned` for transparency about evidence volume.

## Verification
- TDD RED-first: hidden email+phone in `notes` both detected, benign columns clean, name-flagged exclusion, sample-cap respected (asserted via SamplesScanned ≤ cap).
- Test-contract fixes: Categories slice vs single Category field; missing phone check in the checks table caught by the matrix.
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- Merge content findings into the format=sensitive report as a "content-detected" section.
- Threshold tuning (flag only when ≥K matches) if noise appears on real schemas.
