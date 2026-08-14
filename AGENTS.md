# AGENTS.md

Orientation for coding agents working in `bedrock-go` (module `github.com/bosonicalcom/bedrock-go`).

This is a **library**, not a service. It provides the foundational building blocks — persistence,
transport, observability, errors, configuration, messaging — that downstream Go services import to
build distributed systems on a common substrate. There is no `main`, no deployable binary, no
container image, and no domain model here: everything is a reusable primitive consumed by other
repositories.

Two consequences shape almost every decision in this repo:

- **Every exported symbol is public API.** A rename or signature change is a breaking change for
  every consumer. Prefer additive changes; when a breaking change is genuinely warranted, it is a
  deliberate, called-out decision, not a drive-by cleanup.
- **Packages must stay decoupled from any particular application's shape.** A helper that only
  makes sense for one consumer's domain belongs in that consumer, not here.

## Layout

Packages are flat and grouped by concern. Each is independently importable; there is no layering
convention to follow beyond keeping dependencies pointing one way (transport and persistence may
depend on `syserr`/`validator`, not the reverse).

**Persistence**

- `persistence` — generic `Repository[K,T]`, `WriteRepository`, `ReadRepository`,
  `WriteBatchRepository` interfaces, plus `ErrEntryNotFound`.
- `persistence/pagex` — pagination primitives: `Page[T]` with offset/token metadata and its
  helper methods, `PageRepository[T]`, `ListOptions`/`ListOption` (`WithPageSize`,
  `WithPageNumber`, `WithPageToken`, `SafePageSize`), and opaque encrypted page tokens
  (`NewToken`/`ParseToken`).
- `persistence/pqx` — PostgreSQL support: `Config`/DSN building, the `DB` interface,
  `RunMigrations` (golang-migrate wrapper), `Seed` for loading SQL fixtures from an `fs.FS`, and
  `pqxtest` for testcontainer-backed integration tests.
- `persistence/sqlx` — minimal `database/sql`-based `DB` interface.

**Transport**

- `transport/grpcx` — gRPC server/client builders, middleware (logging, recovery, OTel), the
  health service, and `syserr` ⟷ gRPC status conversion.
- `transport/httpx` — HTTP server builder, controller wiring, request/response helpers, health
  filter, and `syserr` ⟷ HTTP status conversion.
- `streaming/kafkax` — Kafka (franz-go) consumer framework: `Handler`, `ConsumerRegistrar`,
  controller manager, interceptors, and event publishing.

**Observability**

- `observability/health` — the `Monitor` health-check interface.
- `observability/logx` — `slog.Handler` extensions that chain interceptors to enrich records.
- `observability/otelx` — OpenTelemetry setup: trace/metric providers, OTLP-gRPC exporters,
  runtime instrumentation.
- `observability/tracex` — trace-ID propagation through `context.Context` and into `slog`.

**Cross-cutting**

- `syserr` — the structured application error type (`Error`, `Code`, `Reason`), domain-scoped and
  translatable; the common currency that transport packages map to and from.
- `validator` — the `Validator` interface and a global instance backed by go-playground/validator.
- `sysconf` — environment-based configuration loading with optional validation.
- `sysenv` — the `Environment` enum for deployment environments.
- `sysevent` — the `Publisher` interface for broadcasting system events.
- `proc` — `BackgroundProcess`: lifecycle and signal handling for long-running processes.
- `syncx`, `timex`, `mimex` — small stdlib extensions (`WaitGroup` with context, a date-only
  `Date` type, MIME type constants).
- `internal/containerx/sets` — internal-only generic set. Not public API; anything under
  `internal/` is invisible to consumers by design.

## Mocks

Generated mocks live in a companion package **inside** the package they mock, suffixed `test` —
`persistence` → `persistencetest`, `persistence/pagex` → `pagextest`, `validator` →
`validatortest`, and likewise `grpcxtest`, `httpxtest`, `healthtest`, `proctest`, `kafkaxtest`.
Never a detached top-level `mocks/` directory.

Each is driven by a `//go:generate` directive placed directly above the interface it mocks (see
`persistence/repository.go`, `persistence/pagex/repository.go`, `validator/validator.go`):

    //go:generate go tool mockgen -source=repository.go -destination=persistencetest/repository_mock.go -package=persistencetest

`task generate` (alias: `task gen-mocks`) runs `go generate ./...` across the module. Generated
files are committed. These mocks exist primarily for **consumers** — they are the sanctioned way a
downstream service fakes a bedrock-go interface — so treat them as part of the public surface.
Details in `docs/testing.md`.

## Development workflow

See **`docs/development-life-cycle.md`** for the standard developer workflow — testing, linting,
the verification steps to run before a change is done, and commit conventions. Read it before
making changes.

## Keeping this document accurate

If a change modifies anything described here or in a doc this file references (layout,
`Taskfile.yml` commands, tooling, conventions) — update that doc as part of the same change. A
stale AGENTS.md is worse than no AGENTS.md: agents that trust it will act on wrong information.
If you're unsure whether your change affects this file, err on the side of checking it.

If you notice this file (or a doc it references) looks stale in a way unrelated to your current
change — e.g. it was hand-edited out of sync, or drifted over several changes — mention the
`/refresh-harness` skill to the developer rather than doing a speculative full re-sync yourself.

## Go language/stdlib questions

For any question about Go language semantics or standard library behavior, official sources
outrank memory, training data, blog posts, or other secondhand explanations:

1. `go doc <package>[.<Symbol>]` — reflects the exact toolchain installed here, the primary
   source for stdlib API behavior.
2. If that's insufficient, <https://go.dev/doc/#references> (the official Go documentation
   index).

If either source contradicts an assumption you were about to act on, defer to the source.

## Go style guide

`docs/go-style-guide.md` is required reference for any agent reading, writing, or reviewing Go
code in this repo — it defines a source hierarchy (Go Proverbs and stdlib docs outrank the
Google/Uber style guides it's synthesized from) for resolving conflicts between style sources.

After finishing a change that touches `.go` files, review the diff against
`docs/go-style-guide.md` before considering the change done — cite the specific section (number
*and* name) for any violation found, and fix it or flag it; don't invent nitpicks the guide
doesn't cover. A Claude Code Stop hook enforces this mechanically in that tool (see
`.claude/settings.json`); this instruction makes the same expectation apply to any other
agent/tool reading this file. See **Reviews** below for the on-demand counterpart.

One standing exception, documented rather than silently applied: the guide's §7.1 "No assertion
libraries" / §7.7a "Use package `testing`" ban third-party test frameworks, but this repo uses
`testify` throughout. That is a deliberate, repo-wide convention — see `docs/testing.md` for the
reasoning and the boundaries. Don't file it as a finding, and don't rewrite existing tests to
remove it.

## API style guide

`bedrock-go` defines no `.proto` files and no resource-oriented API of its own — that surface lives
in the services that consume it. But this repo *does* build the machinery those APIs are exposed
through, so `docs/apis-style-guide.md` is kept here as the reference for API-shaped work:

- `transport/grpcx` and `transport/httpx` — the wiring, error mapping, and status conventions that
  determine what a consumer's API can express. §7 errors is the section that most often applies.
- `persistence/pagex` — list shaping. §5's rules on page size, page tokens, and opaque cursor
  semantics are the contract `ListOptions`/`Page[T]` exist to satisfy; changing them changes what
  every consumer's `List` method can legally do.

Read the relevant section before changing anything in those areas, and when advising on a
consumer's API design. Unlike the Go guide, this one has **no review skill** — there's no proto or
resource-controller diff surface in this repo to scope an automated review against, so it's a
document you consult directly.

It has two parts plus an appendix: **Part I** is the working guide (resources, standard methods,
list shaping, fields, errors, versioning, proto layout, design patterns, and a review checklist),
**Part II** is a routing index with a digest of each of the 68 source documents in
`docs/api-style-guides/`, and **Appendix A** maps AIP numbers to filenames — needed because the
source files cross-link each other by their original `./0NNN.md` names, which no longer exist in
this fork.

`docs/api-style-guides/` is a fork of **Google's** AIP corpus maintained for **Bosonical**. Two
rules follow from that, and every agent must apply both:

- **Translate the brand.** Render the organization as Bosonical in anything you produce. Products
  named in the sources are Bosonical planned products — they were substituted deliberately; don't
  "correct" them back. Residual Google-infrastructure references (GCP launch stages, Cloud Pub/Sub,
  Google Java Style) are analogies: carry the principle, drop the brand.
- **Never rename `google.*` protobuf packages.** `google.protobuf.*`, `google.api.*`,
  `google.rpc.*`, `google.longrunning.*`, and `google.type.*` are protobuf standard types and
  annotations, not branding. Rewriting them breaks the wire format and the build.

The guide also marks a few **[Bosonical]** rules that override upstream guidance — notably `PATCH`
only for Update (no `PUT`), and field masks as a system parameter rather than a `read_mask` field.
Those win over the general AIP text.

## Reviews

When a developer asks for a review of pending work, scope it to what actually changed — not a
full-repo audit:

- **Go changes (`.go`)** — review against `docs/go-style-guide.md`. On demand:
  `/go-style-review` (`.claude/skills/go-style-review/SKILL.md`). This one *also* fires
  automatically via the Claude Code Stop hook in `.claude/settings.json`.
- **API-shaped changes** (transport wiring, error mapping, list/pagination semantics) — there is no
  review skill for these; read the relevant section of `docs/apis-style-guide.md` directly and
  follow its §11 checklist for the parts that apply.

Report findings as plain text, most severe first, one entry per finding:

- **Cite the rule in full** — section number *and* name, never a bare number (e.g.
  "§4.4 Error wrapping (`%w` vs `%v`)", not "§4.4"; for API findings, section, name, and the
  owning AIP: "§3.1 Get (AIP-131 · standard-methods-get.md)"), plus file and line.
- **Show the fix as a minimal before/after code block** — just enough to show the change, not the
  whole function or message. No prose explanation beyond the snippet unless the change is
  genuinely unclear without one.

Don't invent nitpicks the guides don't cover. Generated code (the `<pkg>test` mock packages) is
never a review target: report against the interface that produces it.

**A review never edits code.** The fix snippet is guidance for the developer to act on — not a
patch, not an offer to apply one, not a staged change. Deciding what to do with a finding is the
developer's call, and a review that quietly rewrites the thing it was asked to assess destroys the
developer's ability to trust the report. This holds even when the fix is one line and obviously
correct, and even when the agent itself wrote the code under review. If the developer wants a
finding fixed, they will ask for it as a separate instruction.

Note the boundary: this read-only rule governs the **review flow** — a developer asking for a
review of pending work. It does not govern an agent self-checking its own in-progress change
against a guide before calling that change done (see the two sections above), where correcting
your own violation is exactly the right move.

Agents and tools without the skill mechanism should follow the same flow by reading the relevant
guide directly — the skills are a shortcut, not the definition. These reviews are the on-demand
counterpart to CI, the same way `task lint` is the on-demand counterpart to CI's lint job.

## Releases

Consumers pin this library by version tag, so a release is the moment a change becomes real for
them. Tagging is explicit and manual (`task release VERSION=x.y.z`), which pushes a semver tag and
triggers the GoReleaser workflow. Never tag a release unless the developer asks for one in that
turn. Follow semver against the **exported** surface: a breaking change to any exported symbol is
a major bump, regardless of how small the diff looks.

## Commits

Never run `git commit` (or anything that creates a commit) unless the developer has explicitly
asked for it in that turn and has reviewed and accepted the change first. Finishing a task does
not imply permission to commit it.

When asked to prepare a commit message, write one on demand following the format described in
`docs/development-life-cycle.md` (Conventional Commits, full spec in `docs/conventional-commits.md`).
