---
name: go-style-review
description: Review the current Go changes against docs/go-style-guide.md and report violations. Run manually, or automatically after any agent finishes a Go-related change.
---
Review the current changes to Go code against `docs/go-style-guide.md`.

1. Scope the review to what actually changed — `git diff` (uncommitted changes first; if the
   tree is clean, `git diff $(git merge-base origin/master HEAD)..HEAD` for the current branch).
   Only review `.go` files touched by that diff; this is not a full-repo audit.
2. Read the relevant sections of `docs/go-style-guide.md` for the constructs the diff actually
   touches (naming, error handling, concurrency, testing, etc. — see its outline) rather than the
   whole document for a narrow diff.
3. Flag concrete violations, citing the specific style guide section *and its name* (e.g.
   "§4.4 Error wrapping (`%w` vs `%v`)" — never a bare "§4.4") and file/line. Don't flag nitpicks
   the guide doesn't cover, and don't re-litigate code the diff didn't touch.
4. Report directly to the developer as plain text, most severe first (say so in one line if the
   diff is clean). One entry per finding, in this shape:

   ```
   **`path/to/file.go:LINE`** — problem, one sentence, citing "§N <rule name>" in full.

   Fix:
   ```go
   // before
   <minimal snippet of the offending code>

   // after
   <minimal snippet of the fix>
   ```
   ```

   Keep the fix snippet minimal — just enough to show the change, not the whole function or
   surrounding context. No preamble, no restating the diff, no explaining *why* the rule exists
   beyond naming it.

**This review never edits code.** Report and stop — the fix snippet is guidance for the developer
to act on, not a patch, an offer to apply one, or a staged change. That holds even for a one-line,
obviously-correct fix, and even when you wrote the code under review. Applying a fix is the
developer's decision; if they want one applied they'll ask separately.
