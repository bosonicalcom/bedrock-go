---
name: refresh-harness
description: Re-derive AGENTS.md (and docs it references) from the actual repo state and fix any drift. Run manually when harness docs are suspected stale.
---
Audit `AGENTS.md` and every doc it (transitively) references — `docs/development-life-cycle.md`,
which in turn points to `docs/testing.md` and `docs/conventional-commits.md`, plus
`docs/go-style-guide.md` and `docs/apis-style-guide.md` — against the real repo state, and fix
whatever has drifted.

**Find what actually changed with `git diff`, don't just re-derive everything from scratch:**

1. Find the last commit that touched the harness docs themselves:
   `git log -1 --format=%H -- AGENTS.md docs/development-life-cycle.md docs/testing.md docs/conventional-commits.md docs/go-style-guide.md docs/apis-style-guide.md`
2. Diff from that commit to `HEAD` over the paths the docs describe (the package tree,
   `Taskfile.yml`, `go.mod`, `golangci-lint.mod`, `.golangci.yml`, `.goreleaser.yml`,
   `.github/workflows/`) to see what's changed since the docs were last updated:
   `git diff <that-sha>..HEAD -- <paths>`.
3. You may use your own session memory of recent changes as a starting hint, if you have any —
   but always confirm against the `git diff` output before treating something as stale or as
   already covered; memory of what you *think* changed is not a substitute for checking what
   actually did.

Concretely, cross-check what the diff surfaces (plus a baseline structural pass) against:

- The package inventory in `AGENTS.md`'s Layout section against the actual top-level directories.
  A new package, a removed one, or one whose purpose has shifted must be reflected there — this is
  the section that goes stale fastest.
- Every command documented anywhere in this doc chain (`task test`, `task test:integration`,
  `task lint`, `task lint:upgrade`, `task vet`, `task generate`, `task release`) against
  `Taskfile.yml` — commands that no longer exist must be removed, new ones relevant to the
  workflow should be added.
- Tool dependencies (`go.mod`'s `tool` block, `golangci-lint.mod`) against what's described.
- The mock convention: that every `//go:generate mockgen` directive still writes into a `<pkg>test`
  package inside the package it mocks, and that the `<pkg>test` packages `AGENTS.md`/`docs/testing.md`
  enumerate are the ones that actually exist.
- The testing stack claims in `docs/testing.md` against what the test files actually do — in
  particular the documented `testify` deviation from `docs/go-style-guide.md` §7.1/§7.7a, and the
  external `package <pkg>_test` default. If the tests have moved, the doc's justification has to
  move with them.
- That `AGENTS.md` still points at `docs/go-style-guide.md` and `/go-style-review`, and that both
  still exist.
- Whether the repo has grown anything that invalidates the "this is a library, not a service"
  framing — a `cmd/` directory, a `Dockerfile`, `.proto` files, a `docker-build: true` in CI. Any
  of those would mean larger sections need rewriting, not patching. (Note: if `.proto` files ever
  appear, `docs/apis-style-guide.md` stops being reference-only and warrants a review skill —
  flag that to the developer rather than creating one unprompted.)
- Any other structural claims (release flow, CI workflow references) against the current source.

Report a summary of what was stale and what you changed. If nothing was stale, say so explicitly
rather than making cosmetic edits.
