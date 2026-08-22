# Cycle 39 — Sensitive Column Discovery (`format=sensitive`)

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- MCPg ships `find_sensitive_columns` (PII heuristic) as part of its governance set; every multi-engine SQL server else lacks discovery — they redact only after an operator manually configures rules. Knowing WHAT to protect precedes protecting it.
- Our masking engine (cycles 19–22) already defined the PII taxonomy; discovery reuses the same vocabulary so findings map 1:1 to mask_pii behavior.

## Shipped
- `FindSensitiveColumns(ctx, dbID)`: scans all tables' columns via existing per-engine catalog layer; name heuristics across 9 categories (email, phone, national_id incl. passport, card incl. token/holder forms, personal_name, date_of_birth, address, bank_account); separator normalization (`-`/space/dot → `_`); first-match-wins specificity ordering.
- `FormatSensitiveColumnsReport`: grouped-by-table report ending with explicit `mask_pii` guidance.
- Schema tool: `format=sensitive` renders the report; capability-detected dispatch; format descriptions updated in both variants.

## Verification
- TDD RED-first: 8-column classification matrix (each category asserted), 4 benign columns proven unflagged, clean-schema zero-findings, report rendering. One miss caught by tests (`card_token`) → fragment variants added (`card_`, `_card`).
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- Content-based confirmation (sampling rows through the content patterns from cycle 19) for columns whose names don't reveal PII.
- README schema-tool row mention of the new format value.
