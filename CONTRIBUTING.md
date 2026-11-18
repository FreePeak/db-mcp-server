# Contributing to DB MCP Server

Thank you for considering a contribution. This guide covers local setup, the
test and lint gates, commit and PR conventions, and the contributor license
agreement (CLA).

By participating you agree to abide by the [Code of Conduct](CODE_OF_CONDUCT.md).
To report a security vulnerability, follow [SECURITY.md](SECURITY.md) and do not
open a public issue.

## Contributor License Agreement

Every contribution must be covered by the [Individual CLA](CLA.md). Sign it once,
electronically, through the CLA Assistant GitHub App:

1. Open your first pull request.
2. The CLA Assistant bot comments on the PR with a link to the CLA and the signing sentence.
3. Reply on the PR with exactly this sentence:

   ```text
   I have read the CLA Document and I hereby sign the CLA
   ```

The bot records the signature and updates its status check. Bot accounts on the
maintainers' allowlist (for example `dependabot[bot]` and `github-actions[bot]`)
do not need to sign. The CLA status check must pass before a pull request can be merged.

## Development setup

Requirements:

- Go 1.23 (see `go.mod`; CI uses Go 1.23)
- Docker, only for integration tests
- golangci-lint v1.64.8, only if you want to run the lint locally (the version CI pins; `.golangci.yml` uses v1 syntax)

```bash
git clone https://github.com/FreePeak/db-mcp-server.git
cd db-mcp-server
go mod download
go build ./...
```

Configuration examples live in `config.json`, `config.test.json` and
`config.sqlite-example.json`. Copy `.env.example` for environment variables.
Never commit real credentials.

## Running tests

Unit tests need no database:

```bash
make test-unit
# or
go test -short -race ./...
```

Integration and regression tests need the test databases, started with
Docker Compose, and the `*_TEST_HOST` environment variables. The full setup,
including Oracle, is described in [docs/TESTING.md](docs/TESTING.md).

## Lint

CI's **Lint** job (golangci-lint v1.64.8, configured by `.golangci.yml`) is the
lint gate. A pull request must pass it. Locally, `make lint` runs
`golangci-lint run ./...` using whichever `golangci-lint` is first on your PATH.
Install a v1.64.x binary so it matches CI and reads `.golangci.yml` correctly.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/), for example:

```text
feat(query): add timeout option to execute tool
fix(pool): close idle connections on config reload
docs: document CLA signing
```

Common types are `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `ci` and
`chore`.

## Pull requests

- Keep one logical change per pull request. Separate refactors, formatting
  changes and features into separate PRs.
- Add or update tests for behaviour changes.
- Update the documentation (README, docs/) when behaviour, configuration or
  tools change.
- Fill in the pull request template. It contains the checklist reviewers use.
- Note any breaking change explicitly in the PR description.

### PR checklist

- [ ] Builds (`go build ./...`)
- [ ] Unit tests pass (`go test -short -race ./...`)
- [ ] Integration tests run where the change touches database code
- [ ] CI Lint job passes
- [ ] Commits follow Conventional Commits
- [ ] Documentation updated where behaviour changed
- [ ] CLA signed

## Questions

Open a GitHub issue for bugs and feature requests, using the issue forms.
