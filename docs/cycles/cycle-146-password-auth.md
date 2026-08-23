# Cycle 146 — MD5 Password Audit (performance action=password_auth)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Roles whose stored hash is still md5… are protected by a broken
  scheme — rainbow-tableable and without channel binding. Postgres has
  shipped SCRAM-SHA-256 as the default since v14; a straggler is
  either an old role nobody re-set or a server still defaulting to
  md5. Two signals: server-level password_encryption (what NEW
  passwords get) and per-role stored hashes in pg_authid (what exists
  today). Confirmed absent.

## Shipped

- `internal/usecase/password_auth.go`:
  - `passwordEncryptionQuery` — current_setting('password_encryption').
  - `md5RolesQuery` — pg_authid WHERE rolcanlogin AND rolpassword
    LIKE 'md5%'.
  - `encryptionVerdict` — pure classifier for the server default.
  - `AuditPasswordAuth(ctx, dbID)` — renders both signals with the
    ALTER ROLE fix and a client-compatibility note; clean result
    stated explicitly. Error messages name the required privileges
    (superuser / pg_read_all_settings+stats). Other engines error
    "not available".
- Performance tool: new action `password_auth` (both per-db and
  unified constructors) served via capability interface
  `passwordAuthUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestPasswordAuthProbes`: both probes' shape + engine gating.
  - `TestEncryptionVerdict`: md5→warning naming scram-sha-256,
    scram→clean.
  - `TestAuditPasswordAuth_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=password_auth.
- Post-merge: verify npm v1.12.0 + docker tags published.
