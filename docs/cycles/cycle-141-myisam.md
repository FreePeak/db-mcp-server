# Cycle 141 — MyISAM Table Audit (performance action=myisam_tables)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Tables still on the MyISAM engine have no transactions, table-level
  write locks (every writer queues behind every other), and crash-unsafe
  storage — an unclean shutdown can corrupt indexes and lose in-flight
  rows. InnoDB has been the default since MySQL 5.5; a MyISAM survivor
  is almost always an accident. Confirmed absent.

## Shipped

- `internal/usecase/myisam.go`:
  - `myISAMQuery` — information_schema.TABLES for BASE TABLEs with
    ENGINE='MyISAM'; mysql/mariadb only.
  - `ListMyISAMTables(ctx, dbID)` — renders each MyISAM table with row
    estimate and its transaction/lock/corruption consequences plus the
    ALTER TABLE … ENGINE=InnoDB fix; clean result stated explicitly.
    Other engines error "not available".
- Performance tool: new action `myisam_tables` (both per-db and unified
  constructors) served via capability interface `myISAMUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestMyISAMCatalog`: hits information_schema.TABLES + ENGINE +
    'MyISAM' + TABLE_TYPE; postgres/sqlite "".
  - `TestListMyISAMTables_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=myisam_tables.
- Post-merge: verify npm v1.12.0 + docker tags published.
