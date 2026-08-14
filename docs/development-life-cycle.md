# Development Life Cycle

Standard developer workflow for `bedrock-go` — the day-to-day commands for testing, linting,
verifying a change before it's considered done, and releasing.

## Testing

`task test` runs unit tests; `task test:integration` runs integration tests (requires Docker for
testcontainers). See **`docs/testing.md`** for the full spec on how to add unit and integration
tests, generate mocks, and use the Postgres testcontainer helpers — read it before adding tests to
any package.

## Linting

`task lint` runs `golangci-lint` (config in `.golangci.yml`) against the whole module. It's
declared as a tool dependency in `golangci-lint.mod` (kept separate from the main `go.mod` to
avoid pulling its large dependency tree into the library's own dependency graph, which every
consumer would otherwise inherit) — `task lint:upgrade` bumps it to the latest version.

`task vet` runs `go vet ./...` separately; `task lint` doesn't subsume it.

## Verification before a change is done

There is no single composite task that bundles these — run them in this order:

1. `go build ./...`
2. `task vet`
3. `task lint`
4. `gofmt -l .` — must print nothing (the linter's formatters are `goimports` + `gofumpt`; this
   catches anything they'd rewrite).
5. `go mod tidy` — must leave `go.mod`/`go.sum` unchanged. An untidy `go.mod` in a library leaks
   phantom requirements into every consumer's build.
6. `task test`
7. `task test:integration` — requires Docker running locally.

CI (`.github/workflows/ci.yml`) delegates to the shared `ci-go.yml` workflow with
`docker-build: false`; this repo publishes no container image, only a Go module.

## Public API changes

Every exported symbol is consumed by downstream services that pin this module by version. Before
changing an existing exported signature, name, or behavior, check whether the change is additive
or breaking, and say which in the commit message. Additive changes (new function, new option, new
field on an options struct) are cheap. Breaking changes force a coordinated bump in every consumer
and need to be a deliberate, stated decision — not a side effect of a refactor.

Renaming or removing anything under a `<pkg>test` mock package counts as breaking too: those
mocks exist for consumers to use.

## Releasing

`task release VERSION=x.y.z` creates and pushes the `vx.y.z` tag, which triggers
`.github/workflows/release.yml` (the shared `release-lib-go.yml` workflow, driven by
`.goreleaser.yml` — no binaries are built, `builds: skip: true`; the release is the tag plus a
changelog grouped by Conventional Commit type). Follow semver against the exported surface. See
`AGENTS.md` for the policy on when an agent may run this.

## Commits

Message format follows **`docs/conventional-commits.md`** (Conventional Commits) —
`type(scope): summary`, e.g. `feat(pagex): add safe page size`. The scope is the package
(`pagex`, `pqx`, `grpcx`, `syserr`, …). See `AGENTS.md` for the policy on when an agent may
actually run `git commit`.
