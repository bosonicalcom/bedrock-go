# Testing

How to add unit and integration tests in `bedrock-go`. When in doubt, read the existing tests —
these three cover the patterns that recur:

- `persistence/pagex/token_test.go` — table-driven subtests over a pure, generic API.
- `syserr/error_test.go` — table-driven tests over constructors and option functions.
- `persistence/pqx/migration_test.go` and `persistence/pqx/seed_test.go` — integration tests
  against a real Postgres container.

Because this is a library, tests carry a second job beyond catching regressions: they are the
executable specification of the contract consumers depend on. A test that pins down observable
behavior of an exported symbol is what makes a later refactor safe to ship.

## Stack

- **`package testing` + `testify`** (`require`/`assert`, and `testify/suite` where per-suite
  setup is needed). This is the repo's established convention: `testify` is a direct dependency
  and every existing test file uses it.
  > **Deliberate deviation from `docs/go-style-guide.md`.** §7.1 "No assertion libraries" and
  > §7.7a "Use package `testing`" ban third-party test frameworks. This repo overrides that. The
  > deciding factor is `testify/suite`: `pqxtest.NewContainer`/`NewConn` take a `testing.TB` and
  > register teardown via `tb.Cleanup`, which a bare `TestMain` (`*testing.M`, no `TB`) has
  > nowhere to hook — so suite-scoped container fixtures need a `TB`-carrying setup hook that the
  > stdlib doesn't provide. Having accepted `suite`, holding the line on `require`/`assert`
  > elsewhere would buy inconsistency rather than compliance. Don't file this as a style finding
  > and don't migrate existing tests away from it.
  >
  > What the guide's reasoning still buys you, and is worth keeping: a failure message should name
  > the function and its inputs. `assert.Equal(t, tt.want, got)` in a named subtest does that
  > adequately; a bare `assert.True(t, ok)` does not. Prefer `assert.Equal` over `assert.True` on
  > a comparison, and reach for `require` only when the rest of the test genuinely can't continue.
  >
  > Plain stdlib assertions (`if got != want { t.Errorf(...) }`, `t.Fatalf`) are fine too and
  > appear in the repo — see `persistence/pqx/seed_test.go`. Neither style needs justifying.
- **[go.uber.org/mock](https://github.com/uber-go/mock)** (`gomock`) — the only mock generator
  used here, invoked via `go tool mockgen` (declared as a `tool` dependency in `go.mod`). Do not
  hand-write mocks and do not add another mocking library.
No test file currently reaches for `go-cmp` — it's present only transitively, not a direct
dependency. If a structural diff would read better than `assert.Equal`'s output, adding it is
reasonable, but adding a direct dependency to a library is a decision worth making explicitly
rather than in passing.

## Where test files go

**Default to an external `package <pkg>_test`.** Every test file in this repo does this, and for
a library it's the right default for two reasons beyond avoiding import cycles with the `<pkg>test`
mock packages: it forces the test to consume the package exactly as a downstream service would,
and it makes accidental reliance on unexported internals impossible — so a test passing is real
evidence the public contract holds.

Reach for a same-package (`package <pkg>`) test only when the behavior under test genuinely has no
observable effect through the exported API. If you find yourself wanting one often, that's usually
a signal the package is under-exposing something consumers will need too.

Integration tests are external-package as well, gated with `//go:build integration`.

## Generating mocks

Whenever an interface in this repo needs a mock:

1. Add a `//go:generate` directive directly above the interface definition:
   ```go
   //go:generate go tool mockgen -source=repository.go -destination=persistencetest/repository_mock.go -package=persistencetest
   ```
2. **The generated mock package lives inside the package it mocks**, suffixed `test` —
   `persistence` → `persistence/persistencetest`, `persistence/pagex` → `pagex/pagextest`,
   `validator` → `validator/validatortest`, and likewise `grpcxtest`, `httpxtest`, `healthtest`,
   `proctest`, `kafkaxtest`. Never a detached top-level `mocks/` directory.
3. Run `task generate` (or scope it: `go generate ./persistence/...`) and commit the generated
   file.

These mocks are public API. A downstream service that fakes `persistence.Repository[K,T]` does it
with `persistencetest.NewMockRepository` — so adding a method to an interface changes what every
consumer's regenerated mock must satisfy, and renaming a mock type breaks their tests. Treat both
as breaking changes.

## Table-driven unit test template

```go
func TestConfig_Insecure(t *testing.T) {
    tests := []struct {
        name    string
        modeSSL string
        want    bool
    }{
        {name: "disable", modeSSL: "disable", want: true},
        {name: "verify-full", modeSSL: "verify-full", want: false},
        {name: "empty", modeSSL: "", want: false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            assert.Equal(t, tt.want, pqx.Config{ModeSSL: tt.modeSSL}.Insecure())
        })
    }
}
```

For a generic API, subtests-per-type read better than a table — one `t.Run` per instantiation, as
in `persistence/pagex/token_test.go`'s round-trip test (`string`, `int64`, struct, slice, map).

Per `docs/go-style-guide.md` §7.9, reserve `require`/`t.Fatal` for a failure the rest of the test
can't meaningfully continue past: a table-wide setup failure before the loop starts, or — inside a
`t.Run` subtest specifically — a step whose failure makes later checks on that same row
meaningless (it only ends that one subtest closure; the loop still reaches the next row). For a
row failure with no subtest, use `assert`/`t.Error` and `continue` so the rest of the table still
runs and reports.

## What to test, per kind of package

- **Pure logic** (`syserr`, `timex`, `mimex`, `sysenv`, `pagex`'s tokens and `Page[T]` helpers) —
  unit tests, no mocks, no I/O. Cover the zero value and the nil receiver explicitly: `Page[T]`'s
  helpers are nil-safe on purpose (`HasNextPage`, `PageNumber`, … all return a zero value on a nil
  receiver), and that's a promise to consumers, not an implementation detail.
- **Backed by an external system** (`persistence/pqx`, `streaming/kafkax`) — integration tests
  against the real dependency, gated `//go:build integration`. Never mock the SQL or broker layer
  to test these; the whole point is verifying behavior against the real engine.
- **Interface-only or wiring packages** (`persistence`, `validator`, `sysevent`,
  `observability/health`) — there is little behavior of our own to assert. Test the concrete
  implementations that live alongside the interface (e.g. `validator/goplayground_test.go`), and
  let the generated mock serve consumers rather than testing the mock itself.
- **Transport** (`transport/grpcx`, `transport/httpx`) — the highest-value tests here are the
  `syserr` ⟷ status conversions in both directions, since that mapping is the contract every
  consumer's error handling is built on. `transport/grpcx/client_test.go` drives a real server
  over `bufconn` rather than mocking the transport — prefer that when the wiring itself is what's
  under test.
- **Option functions** — a package with functional options (`pagex.ListOption`,
  `pqxtest.ContainerOpt`, `syserr.WithDetails`) should have a test that applies each option and
  asserts the resulting struct, plus one for the default when no options are passed. Defaults are
  the part consumers hit without knowing it: `ListOptions.SafePageSize()` returning 25 for an
  unset page size is exactly the kind of behavior a consumer builds a SQL `LIMIT` on.

## Integration tests: real Postgres via testcontainers

Gate the file with `//go:build integration` — these need Docker and are excluded from the default
`task test` run. `persistence/pqx/pqxtest` spins up the container and registers its own teardown
via `tb.Cleanup`.

The suite form, when several tests share one container (`persistence/pqx/migration_test.go`):

```go
//go:build integration

package pqx_test

type MigrationSuite struct {
    suite.Suite
    cfg pqx.Config
}

func (s *MigrationSuite) SetupSuite() {
    s.cfg = pqxtest.NewContainer(s.T())
}
```

The plain form, for a single test (`persistence/pqx/seed_test.go`):

```go
func TestSeed(t *testing.T) {
    conf := pqxtest.NewContainer(t)

    connPool, err := pgxpool.New(t.Context(), conf.DSN().String())
    if err != nil {
        t.Fatalf("cannot start postgres pool: %v", err)
    }
    defer connPool.Close()
    ...
}
```

Use `pqxtest.NewConn` when a `*pgx.Conn` is enough; build a `*pgxpool.Pool` via `pgxpool.New(ctx,
cfg.DSN().String())` when the test needs one — `pqx.Seed` requires a pool. Both satisfy `pqx.DB`.

Starting a container costs seconds, so share one per suite and isolate between tests by truncating
or by using distinct table/schema names, rather than starting a fresh container per test.

**Build fixtures in the test, not from files on disk.** Both `RunMigrations` and `Seed` take an
`fs.FS`, so `testing/fstest.MapFS` gives you migration and seed content inline — immune to the
test's working directory, and it keeps each test's fixture visible next to the assertions that
depend on it. This is what `migrationFS()` and `seedFS()` do in the two files above. Consumers
will pass a real `embed.FS` instead; that difference is theirs to exercise, not ours.

## Running tests

- `task test` (`go test -timeout 1m ./...`) — unit tests only, no Docker required.
- `task test:integration` (`go test -tags integration -timeout 1m ./...`) — includes the Postgres
  suites; requires Docker running locally (or in CI).

## Non-goals

- **Don't test generated mock code.** The `<pkg>test` packages are mockgen output; assertions
  about them test mockgen, not us.
- **Don't re-test a wrapped library's own behavior.** `pqx.RunMigrations` wraps golang-migrate:
  test that our config plumbing, ordering guarantee, and idempotency hold — not that
  golang-migrate can apply SQL. Same for franz-go in `kafkax`, go-playground in `validator`, and
  the OTel SDK in `otelx`. The line is our adapter logic, and it's where the bugs actually are.
