# Go Style Guide — for developers and coding agents

## Source hierarchy

This guide merges seven sources into one document. They do **not** carry equal weight:

**Tier 1 — Official Go project docs (highest authority, always wins on any conflict):**
- Effective Go — https://go.dev/doc/effective_go
- Go Proverbs — https://go-proverbs.github.io/
- The Go Memory Model — https://go.dev/ref/mem

**Tier 2 — Google Go Style Guide (wins over Uber on any conflict, per explicit instruction; subordinate to Tier 1):**
- Google Go Style Guide (`guide`) — https://google.github.io/styleguide/go/guide
- Google Go Style Decisions (`decisions`) — https://google.github.io/styleguide/go/decisions
- Google Go Style Best Practices (`best-practices`) — https://google.github.io/styleguide/go/best-practices

**Framework tier — general testing philosophy, not Go-specific, not part of either style-guide family, doesn't override anything but explains the "why" behind §7's mechanical rules:**
- Google Testing on the Toilet: "Effective Testing" (Rich Martin, May 2014) — https://testing.googleblog.com/2014/05/testing-on-toilet-effective-testing.html

**Tier 3 — Uber Go Style Guide (kept wherever it doesn't conflict with Tier 1 or Tier 2):**
- https://github.com/uber-go/guide

**Resolution rule applied throughout:** on any direct conflict, the higher tier wins outright. Tier 1 is the Go language authors' own guidance and is treated as ground truth — it is rarely prescriptive at the level of "name your variable this way," so most of its content sets *principles and mechanics* (semantics of `:=`, what `panic`/`recover` actually do, what synchronization actually guarantees) that Tier 2 and Tier 3 style rules must not contradict. Where Tier 1 is silent (which is most concrete style questions — naming conventions, import grouping, test structure), Tier 2 (Google) is the default, with Tier 3 (Uber) filling gaps Tier 2 doesn't cover. The Framework tier sits alongside this hierarchy rather than inside it: it's a rationale/philosophy layer specifically for §7 (Testing), not a source of style rules that could conflict with the others. Every resolved conflict is marked `[CONFLICT → wins]` inline, naming which source wins and why.

**Known gaps from source retrieval** (flagged for transparency, not because they change any rule below):
- Google's `decisions` doc: fully retrieved as of this revision, via a user-supplied file upload after four prior fetch attempts (rendered HTML, raw GitHub markdown at two token limits) each truncated at the same point. The full document is now reflected below, including its final "Non-decisions" section (§7.13) which wasn't visible in any earlier fetch.
- Google's `best-practices` doc was truncated near the end of its "Global state" section, past the point where all substantive guidance had already been captured; this gap remains open but is not believed to contain conflicting content.

---

## 0. Go Proverbs (Tier 1 — Rob Pike, source of highest authority)

These are terse, load-bearing maxims — not decoration. Each one below is cross-referenced to the fuller rule elsewhere in this doc that explains its mechanics. Treat these eighteen lines as the summary a developer or agent should hold in mind before writing a line of Go:

1. **Don't communicate by sharing memory, share memory by communicating.** → §5 (Concurrency), §11 (Memory Model)
2. **Concurrency is not parallelism.** → §5.7; concurrency is a way of structuring independently-executing components, parallelism is running calculations simultaneously for speed — Go's tools make the former easy, which sometimes enables but is not the same as the latter.
3. **Channels orchestrate; mutexes serialize.** → Use channels when coordinating/sequencing work across goroutines; use a mutex when simply protecting a shared value from concurrent access. Reaching for a channel to do a mutex's job (or vice versa) is a design smell.
4. **The bigger the interface, the weaker the abstraction.** → §6.1. Reinforces Google's "keep interfaces small" and "consumer defines the interface" guidance.
5. **Make the zero value useful.** → §6.13, §6.14, Tier-1 elaboration in §12 (Effective Go: Allocation with `new`). Design types so `var x T` is immediately usable without a constructor.
6. **`interface{}` says nothing.** (Modern equivalent: `any` says nothing.) → A parameter typed `any`/`interface{}` carries no information to the reader about what it actually accepts — prefer a concrete type or a small, named interface wherever possible. Ties into §6.2's generics guidance: generics exist partly to avoid this proverb's failure mode.
7. **Gofmt's style is no one's favorite, yet gofmt is everyone's favorite.** → Never hand-format Go source or argue about formatting in review; run `gofmt` and move on. See §3 (Formatting).
8. **A little copying is better than a little dependency.** → Weigh a small amount of duplicated code against pulling in a new package/dependency for a trivial helper. Direct tie to Google's "least mechanism" principle (§1).
9. **Syscall must always be guarded with build tags.**
10. **Cgo must always be guarded with build tags.**
11. **Cgo is not Go.** → (9)–(11) are platform/FFI-boundary discipline: code using `syscall` or `cgo` is inherently non-portable and must be isolated behind build constraints so the rest of the codebase stays portable.
12. **With the unsafe package there are no guarantees.** → `unsafe` opts out of the type system and the memory model's guarantees entirely. Treat any use as a documented, reviewed exception, never a default tool.
13. **Clear is better than clever.** → The single highest-order tiebreaker in this entire document when no other rule resolves a question. Directly reinforces Google's Clarity-first principle (§1).
14. **Reflection is never clear.** → `reflect` trades compile-time checking and readability for dynamic flexibility; treat it as a last resort, and when used, document heavily why it was necessary.
15. **Errors are values.** → §4. Errors are ordinary Go values that can be constructed, inspected, and composed with regular code — not a special control-flow channel bolted on top of the language.
16. **Don't just check errors, handle them gracefully.** → §4.6. Checking `if err != nil { return err }` everywhere is necessary but not sufficient — the proverb is a reminder to think about what handling actually means at each call site (see the linked Dave Cheney post: "Don't just check errors, handle them gracefully," referenced directly in Uber's guide too).
17. **Design the architecture, name the components, document the details.** → A top-down reminder for how to approach writing a new system: structure first, names next, then fill in doc comments (§6, Commentary).
18. **Documentation is for users.** → §6. Write doc comments for the person calling your code, not for yourself six months from now re-deriving what it does — though in practice that's often the same person.
19. **Don't panic.** → §4.8. The proverb's own canonical link points to the exact rule already stated there — recover, return an `error`, and reserve `panic` for genuinely unrecoverable programmer-error conditions.

---

## 1. Guiding principles

Google's `guide` doc is philosophy-only (no equivalent in Uber). It ranks five properties of readable code, in order: **Clarity → Simplicity → Concision → Maintainability → Consistency**. Two points worth keeping visible because they set the tone for everything below:

- **Least mechanism**: prefer a core language construct (slice, map, channel, loop) over a stdlib tool, and a stdlib tool over a new dependency or custom abstraction. This is Google-only and has no Uber equivalent, but it doesn't conflict with anything Uber says — kept as a standing principle.
- **Local consistency**: where the guide is silent, match the surrounding file/package. If a local deviation would spread (touch more files, more API surface), fix it instead of extending it.

---

## 2. Naming

### 2.1 Package names
`[CONFLICT → Google wins, but all three tiers mostly agree]`
All three sources converge: all-lowercase, no underscores, no `util`/`common`/`helper`/`model`. Effective Go (Tier 1) supplies the original rationale Google and Uber both build on: a package name is an **accessor** — after `import "bytes"`, callers write `bytes.Buffer`, so the name should be short, concise, evocative, and exported identifiers inside the package should be chosen assuming the package-name prefix is already doing work (`ring.New`, not `ring.NewRing`, because the caller already sees `ring.` — this is the "constructor is named `New`" convention, stated independently by both Tier 1 and Tier 2). Tier 1 also gives the naming convention for source layout: the package name is the base name of its directory (`src/encoding/base64` → package `base64`, not `encoding_base64`).

Google (Tier 2) adds detail Tier 1 doesn't state explicitly:
- Avoid names likely to be **shadowed** by common local variables (`usercount`, not `count`).
- Generated protobuf packages *must* be renamed to drop underscores, local name gets a `pb` suffix (e.g. `foopb`); best-practices refines this further — prefer a full descriptive word over historically-common ultra-short forms like `xpb` or bare `pb` in new code (existing short names aren't worth mass-renaming).
- `_test` suffix packages (black-box tests, integration tests) are the sanctioned underscore exception.
- Best-practices' package-size guidance (no direct Tier 1/Tier 3 equivalent, kept as-is): there's no "one type, one file" rule in Go. Split a package into multiple files when it groups genuinely distinct responsibilities (`net/http`'s `client.go`/`server.go`/`cookie.go`); keep related types in one package specifically when their *implementations* are tightly coupled and a hypothetical caller would need both to do anything useful — that coupling is a signal they belong together, not a reason to force separate packages "for cleanliness."

### 2.2 Receiver names
Google-only, no conflict. Short (1–2 letters), consistent across all methods on the type, never `this`/`self`, omitted if unused.

### 2.3 Constant names
Google-only, no conflict. MixedCaps always, no `k`-prefix, no `SCREAMING_CASE`. Name constants for their **role**, not their value (`ExecuteBit`, not `Twelve`).

### 2.4 Initialisms
Google-only, no conflict. `URL`/`url`, never `Url`. Multi-initialism names (`XMLAPI`) keep each initialism internally consistent but don't need to match each other's case.

### 2.5 Getters
`[All three tiers agree, no conflict]`
No `Get`/`get` prefix unless the domain concept itself is "get" (HTTP GET). Effective Go states the original rationale: Go has no built-in getter/setter support, and putting `Get` in a getter name is neither idiomatic nor necessary — a field `owner` gets an exported getter method `Owner()` (capitalization alone provides the export/access distinction), with a setter (if needed) named `SetOwner`. Google's elaboration: prefer `Counts()` over `GetCounts()`; use `Compute`/`Fetch` instead of `Get` specifically when the call is expensive or can fail/block, to warn the reader.

### 2.6 Variable name length and repetition
Google-only, no conflict, but overlaps in spirit with Uber's terser guidance. Google's fuller model: name length should scale with scope size and inversely with use-frequency. Omit type-like words (`userCount` not `numUsers`), omit words already given by context (inside `UserCount()`, a local called `count` beats `userCount`).

### 2.7 Errors are named with `Err`/`err`, custom types with `Error` suffix
Both guides agree — no conflict. See §4.4.

### 2.8 Function and method naming — avoiding repetition at the call site
Google best-practices-only, no Tier 1/Tier 3 equivalent at this level of detail, kept as-is. Omit from a function/method name whatever is already implied by the call site: input/output types (when unambiguous), whether a value is a pointer, the receiver's type, the names of parameters passed in, and the package name itself (`yamlconfig.Parse`, not `yamlconfig.ParseYAMLConfig`; `(*Config) WriteTo`, not `(*Config) WriteConfigTo`). Functions that *return* something get noun-like names (`JobName`); functions that *do* something get verb-like names (`WriteDetail`). When near-identical functions differ only by the concrete type involved, put the type name at the *end* (`ParseInt`, `ParseInt64`) — the "primary" or most common variant can omit the type suffix entirely (`Marshal` vs. `MarshalText`).

### 2.9 Shadowing vs. "stomping"
Google best-practices-only, no Tier 1/Tier 3 equivalent, kept as-is — but directly informed by Tier 1's own explanation of `:=` redeclaration semantics (Effective Go, "Redeclaration and reassignment," folded in here since it's the same topic). Two distinct things can happen with `:=`:
- **"Stomping"** (informal term, not in the language spec): in `f, err := os.Open(name)` followed later by `d, err := f.Stat()` in the *same scope*, `err` is not re-declared — it's reassigned. This is legal specifically because at least one new variable (`d`) is being introduced in the second statement, and it's the idiomatic way to thread a single `err` through a long chain of operations.
- **Shadowing**: using `:=` inside a *new* block (an `if`, a `for`) introduces a genuinely new variable that only exists inside that block, even if it has the same name as an outer variable. Code after the block still refers to the outer one. This is a common source of bugs — e.g. conditionally trying to shorten a `context.Context`'s deadline inside an `if` block using `ctx, cancel := context.WithTimeout(...)` silently creates a block-scoped `ctx` that vanishes at the closing brace, leaving the *outer* `ctx` untouched, while the code still compiles because both new variables were used inside the block. Fix: declare `cancel` with `var` outside the block and use plain `=` (not `:=`) for the assignment that must reach the outer scope.
- Related, no Tier 1/Tier 3 equivalent: avoid naming a local variable the same as a standard-library package (`url := "..."` shadows `net/url` for the rest of that scope) except in very small scopes. Symmetrically, when naming your *own* package, avoid names likely to cause this problem for your callers.

---

## 3. Formatting & structure

### 3.0 `gofmt` is not optional
`[Tier 1, overrides everything below on any actual formatting dispute]`
Effective Go and the Proverbs are unambiguous and take precedence over any stylistic preference in this document: run `gofmt` (or `go fmt`) and accept its output. Do not hand-align struct field comments, do not manually manage indentation, do not debate brace placement — `gofmt` resolves all of it mechanically. If `gofmt`'s output looks wrong for some new situation, the fix is to restructure the code or file a bug against `gofmt`, never to work around it by hand-formatting. Tabs for indentation (not spaces) is what `gofmt` emits by default and is not to be overridden. Every other rule in §3 concerns things `gofmt` does *not* decide for you (line-breaking judgment calls, import grouping, when to introduce a variable) — none of them license deviating from `gofmt`'s actual formatting output.

### 3.1 Line length
`[CONFLICT → all three tiers actually agree once compared directly; Google's wording is used as primary]`
- **Effective Go (Tier 1)**: "Go has no line length limit. If a line feels too long, wrap it and indent with an extra tab."
- **Google (Tier 2)**: no fixed length at all — if a line feels too long, refactor instead of wrapping; don't split before an indentation change or to shorten a URL/long string.
- **Uber (Tier 3)**: soft limit of 99 characters; explicitly stated as non-enforced ("not a hard limit").
- **Merged rule**: no hard line-length limit — this is Tier 1's own position, so it isn't really a "Google wins" case, it's Tier 1 confirming Tier 2 and superseding Uber's numeric guideline entirely. Prefer refactoring over wrapping when a line feels too long. Don't break lines immediately before an indentation change (function signatures, `if` conditions).

### 3.2 Grouping declarations
Both agree: group related `const`/`var`/`type`/`import` blocks; don't group unrelated ones. Uber adds a specific case Google doesn't state explicitly: **adjacent variable declarations inside a function should still be grouped even if the variables are unrelated** — kept, no conflict.

### 3.3 Import grouping
`[CONFLICT → Google wins]`
- **Uber**: two groups — standard library, then everything else.
- **Google**: four groups, in order — standard library; other (project/vendored); protocol buffer imports (`fpb "path/to/foo_go_proto"`); blank/side-effect imports (`_ "path/to/pkg"`).
- **Merged rule**: use Google's four-group ordering.

### 3.4 Import aliasing / renaming
`[CONFLICT → Google wins, mostly compatible]`
Both agree renaming should be rare and driven by collision or generated code. Google is stricter and more specific:
- Renaming is *mandatory* to resolve a collision (prefer renaming the more local/project-specific import).
- Protobuf imports *must* be renamed to a `pb`-suffixed name.
- Uninformative names (`util`, `v1`) *may* be renamed, sparingly — prefer fixing the package name at the source.
- If a package name collides with a variable you want (`url`, `ssh`), rename the **import** with a `pkg` suffix (`urlpkg`), not the variable.
- Uber's version ("alias if the name doesn't match the last path element") is a subset of this and is superseded.

### 3.5 `import .` (dot imports)
Google-only, no conflict (Uber doesn't mention it). Never use in application code — it obscures where symbols come from.

### 3.6 Literal formatting
`[CONFLICT → Google wins]` on the struct-literal field-name question specifically; everything else is additive.
- **Uber**: "almost always" name fields when initializing structs (enforced loosely, exception for ≤3-field test tables).
- **Google, more precise**: field names are **required** for struct literals of types defined **outside the current package**; **optional** for package-local types (but still encouraged when it aids clarity, e.g. many-field structs).
- **Merged rule**: field names mandatory for external types, optional-but-encouraged for local types. Uber's ≤3-field test-table exception still applies as a local-type case.
- Google-only additions (no Uber equivalent, kept as-is): matching-brace indentation rules, "cuddled braces" rules for slice/array literals, omitting repeated type names in slice/map literals, omitting zero-value struct fields when clarity isn't lost.

### 3.7 Function formatting
Google-only, no direct Uber equivalent, kept as-is:
- Signature stays on one line to avoid indentation confusion.
- Don't line-break call sites based purely on length — factor out a local variable instead.
- Avoid inline comments on individual arguments (`New(ctx, 42 /* Port */)`); use an option struct or fuller docs instead.

### 3.8 Conditionals, loops, switch
Google-only, no direct Uber equivalent, kept as-is:
- Don't line-break `if` conditions (indentation confusion); extract booleans into named locals instead if you need to shorten them.
- `switch`/`case` clauses stay on one line each; if a case list must wrap, indent all cases and separate with a blank line.
- Comparisons: variable on the left (`if result == "foo"`), never "Yoda conditionals" (`if "foo" == result`).

### 3.9 Reduce nesting / indent error flow
`[All three tiers agree — Tier 1 states the original rationale]`
Handle the error/terminal case first, return/continue early, keep the "normal path" unindented. Don't use `else` when the `if` branch already returns. Effective Go states the underlying reason directly: in Go libraries, when an `if` body ends in `break`/`continue`/`goto`/`return`, the subsequent `else` is dropped — code reads better when the successful control-flow path runs straight down the page and error cases are eliminated as they arise, rather than nesting the success path inside an `else`.

### 3.9a Semicolon insertion and brace placement
Tier 1-only mechanical rule, foundational to why Go's formatting looks the way it does, kept as-is. Go's grammar uses semicolons to terminate statements, but the lexer inserts them automatically at the end of a line if the last token could end a statement (identifier, literal, or one of `break continue fallthrough return ++ -- ) }`). The practical consequence every developer and agent must internalize: **an opening brace can never go on its own line** for `if`/`for`/`switch`/`select`/func bodies — `if x < 0\n{` triggers automatic semicolon insertion after `x < 0`, silently breaking the statement. This is why Go's canonical brace style (`if x < 0 {`) is not a stylistic choice — it's the only form the grammar accepts without a spurious semicolon.

### 3.10 Unnecessary else
Uber-only, no Google equivalent, kept as-is. If a variable is set in both branches of an `if`, collapse to a single assignment + conditional override.

### 3.11 Reduce scope of variables
Uber-only, no Google equivalent, kept as-is. Prefer `if err := f(); err != nil` when the result isn't needed outside the block; don't force this if the value *is* needed later — declare normally instead.

### 3.12 `switch`/`break`
Google-only, no Uber equivalent, kept as-is. Never use a bare `break` at the end of a `case` — it's redundant in Go (unlike C/Java). Use a labeled `break` only to exit an enclosing `for`.

### 3.13 Copying values
Google-only, no Uber equivalent (Uber only discusses this narrowly under mutexes — see §5.1), kept as-is and broadened. Don't copy a value of type `T` if `T`'s methods are defined on `*T` (e.g. `bytes.Buffer`). This generalizes Uber's mutex-copying rule.

---

## 4. Errors

### 4.0 Errors are values (Tier 1 — Rob Pike, foundational)
Before any rule below: `error` is a plain built-in interface —
```go
type error interface {
    Error() string
}
```
— and Go's multi-value returns exist largely to make this pattern natural: a function can return both a normal result *and* a detailed error value in one signature, instead of the C convention of smuggling an error code through an in-band sentinel (§4.7 covers this in more depth). Effective Go's example is `os.Open`, which returns `nil, error` — and that `error` is typically a concrete `*os.PathError` under the hood, carrying the operation, path, and underlying syscall error as structured fields, not just a string. This is the origin of §4.1's "does the caller need to match the error" framework and §4.9's "extra information in errors" guidance below: a well-designed error carries data a caller (or a human debugging weeks later) can actually use.

Effective Go's convention, still current: **error strings should identify their origin**, typically with a prefix naming the operation or package (`"image: unknown format"`), so an error printed far from where it was created is still traceable. This is the direct ancestor of §4.3's lowercase/no-punctuation rule below — errors are designed to be embedded inside other errors and other text, not read standalone.

### 4.1 Choosing an error type
Uber-only structured guidance (2×2 matrix: match-needed × static/dynamic message), no Google equivalent, kept as-is:

| Needs matching? | Message | Use |
|---|---|---|
| No | static | `errors.New` |
| No | dynamic | `fmt.Errorf` |
| Yes | static | top-level `var` + `errors.New` |
| Yes | dynamic | custom `error` type |

### 4.2 Returning errors
`[CONFLICT → Google wins, but largely additive]`
Both agree `error` is the idiomatic last return value. Google adds a rule Uber doesn't state explicitly: **exported functions must return the `error` interface, never a concrete error type** (`*os.PathError`), because a concrete `nil` pointer wrapped in an `error` interface becomes non-nil. Adopted as-is.

### 4.3 Error string formatting
Google-only, no Uber equivalent, kept as-is. Error strings: lowercase (unless starting with an exported name/proper noun/acronym), no trailing punctuation — they get embedded in other messages. Full displayed messages (logs, test output) follow normal capitalization.

### 4.4 Error wrapping (`%w` vs `%v`)
`[UPDATED — Google's best-practices doc, once fully retrieved, turns out to have a detailed, explicit position; supersedes the earlier "Google is silent" read]`
- **Uber**: explicit default — use `%w` unless you deliberately want to obfuscate the cause, in which case use `%v`. Also: avoid "failed to" boilerplate that piles up across layers.
- **Google best-practices (Tier 2, authoritative here)**: the choice is nuanced and context-dependent, not a blanket default either way:
  - **Use `%v`** for: adding genuinely new, non-redundant context; logging/display output where the caller isn't expected to programmatically inspect the result; and translating an error at a system boundary (RPC/IPC/storage) into a canonical error space, deliberately hiding the internal cause from callers who shouldn't depend on it.
  - **Use `%w`** for: adding context while preserving the ability for a caller to `errors.Is`/`errors.As` the underlying cause — the primary case *within* an application's internal call chain — and for cases where your package's documented contract explicitly promises a particular underlying error can be unwrapped.
  - **Never annotate purely to signal failure with no new information** — `fmt.Errorf("failed: %v", err)` should just be `return err`.
  - **Placement of `%w` matters for readability**, not just semantics: wrapped errors form a chain traversed newest-to-oldest via `Unwrap()`, but the *string* only reads newest-to-oldest if `%w` is placed at the **end** of the format string (`"new store: %w"`, not `"%w: new store"`). This directly confirms Uber's `[...]: %w` convention — not a coincidence, both guides converge on the same form independently. **Exception**: sentinel/category errors (e.g. `ErrParse`, `os.ErrInvalid`) should have `%w` placed at the **front** instead, so the category is the first thing a reader sees (`fmt.Errorf("%w: invalid header", ErrParse)`) — this is additive detail Uber doesn't cover.
- **Merged rule**: adopt Google's fuller framework as primary since it directly addresses the question with worked examples; Uber's "`%w` unless obfuscating" is a reasonable simplification of the same underlying logic and isn't contradicted, so it survives as a quick heuristic for the common internal-call-chain case. Keep Uber's "avoid 'failed to' stacking" as-is — Google's redundant-annotation rule above generalizes it.

### 4.5 Error naming
Both agree, no conflict: `Err`/`err` prefix for sentinel error vars (exported/unexported), `Error` suffix for custom error types. Uber explicitly carves this out as an exception to its own unexported-globals-get-`_`-prefix rule — kept.

### 4.6 Handle errors once / handle errors deliberately
`[Merged — same principle, Google's version is more prescriptive, used as primary]`
- **Uber**: "handle each error once" — don't log-and-return (double handling up the stack); wrap-and-return, or log-and-degrade, or match-and-degrade, are all fine.
- **Google**: same spirit, framed as a closed list of valid responses to a returned error — handle immediately, return it (wrapped or not), or (rare) `log.Fatal`/panic. Discarding via `_` needs an explaining comment.
- **Merged rule**: Google's three-option framing is the primary statement; Uber's specific "log-and-return is bad, log-and-degrade is fine" examples are kept as illustrations since they don't conflict.

### 4.7 In-band errors
Google-only, no Uber equivalent, kept as-is. Don't signal failure via sentinel return values (`-1`, `""`, `nil` used ambiguously). Return an explicit second value (`(value, ok)` or `(value, error)`) instead — this also happens to make the API un-compile if a caller forgets to check it.

### 4.8 Don't panic
`[CONFLICT → Google wins on the exception scope, otherwise identical]`
Both agree: no `panic` for normal error handling in production code; return `error` instead.
- **Uber**: allows panic at program-initialization time if startup failure should abort the program.
- **Google**: same idea but names the mechanism — prefer `log.Exit`/`log.Fatal` (their internal `log`, not stdlib) over a bare `panic` in `main`/`init`, specifically because a stack trace usually isn't useful for a config error. Reserves actual `panic` for genuinely "impossible" conditions (bugs that should've been caught in review/testing).
- **Merged rule**: at startup, prefer `log.Fatal`-style exit over `panic` when there's nothing a caller could recover from; reserve `panic` for programmer-error invariants. (Note: Uber's examples use stdlib `log`; Google's internal `log` package with `Fatal`/`Exit` is a Google-internal detail — external readers should read "log.Fatal" as "your logging library's fatal-exit call," not literally stdlib `log.Fatal`, which *does* skip deferred cleanup exactly like `os.Exit`.)

### 4.9 Handle type assertion failures
`[Tier 1 confirms and extends Uber's rule]`
Always use the two-value "comma ok" form (`t, ok := i.(string)`); the single-value form panics on mismatch. Effective Go frames this as the general mechanism for interrogating an error's concrete type when callers need more than "did it fail": a caller can type-assert (or type-switch) on the returned `error` to extract a concrete type like `*os.PathError` and read its structured fields — this is the mechanical basis for §4.1's "custom error type" row and for structured-error design generally (§4.10 below).

### 4.10 Error structure — giving callers something to match against
Google best-practices-only, no Tier 3 equivalent at this depth, kept as-is. If a caller needs to distinguish between different error conditions programmatically, that needs to be possible **without string matching** — never `regexp.MatchString` or `strings.Contains` against `err.Error()` to detect an error's category. Two structuring approaches:
- **Sentinel values**: package-level `var ErrX = errors.New("...")`, compared with `errors.Is` (handles wrapping) or `==` (only for unwrapped errors). Simplest option, adequate for most cases — see the `Ping`/`fs.ErrNotExist`-style examples throughout this doc.
- **Structured error types**: when the caller needs actual *data* out of the error (not just "which category"), give the error type fields the caller can read directly — `os.PathError`'s `Path` field is the canonical example. Document explicitly whether the returned type is a pointer or value receiver (`*PathError` vs. `PathError`), since callers doing `errors.Is`/`errors.As`/`cmp.Equal` need to know which to compare against.
- Prefer documenting error contracts **at the package level** ("errors returned by this package are usually of type `*PathError`...") rather than repeating the same note on every function, when the behavior is package-wide.

### 4.11 Logging errors — avoid double handling
`[Directly extends §4.6's "handle once" principle with concrete logging guidance, Google best-practices-only, no Tier 3 equivalent, kept as-is]`
If a function returns an error, prefer **not** logging it yourself — let the caller decide whether to log, degrade, retry, or propagate further. Logging *and* returning the same error is the most common violation of "handle each error once": it produces duplicate log noise as the error is logged again by every layer up the stack. Additional logging-specific guidance:
- Be deliberate about PII in error/log content — many log sinks aren't appropriate destinations for sensitive user data.
- Error-level logging is comparatively expensive (can trigger a flush) — reserve it for genuinely *actionable* conditions, not just "more serious than a warning."
- For structured verbosity control (if your logging library supports leveled verbose logging), guard expensive-to-compute log arguments behind an explicit level check rather than relying on the logging call itself to short-circuit — a call like `log.V(2).Infof("%v", sql.Explain())` still evaluates `sql.Explain()` even when V(2) logging is disabled, unless guarded with `if log.V(2) { ... }`.

### 4.12 Program initialization failures vs. programmer-error panics
`[Extends §4.8, Google best-practices-only, no Tier 3 equivalent, kept as-is]`
Two genuinely different failure categories that call for different responses, refining §4.8's merged rule:
- **Configuration/startup failure** (bad flags, missing required config): propagate the error up to `main`, which exits with a clear, human-actionable message. A stack trace is *not* useful here — the fix is "provide a required flag," not "here's where the check failed."
- **Broken invariant / "impossible" condition** (internal state has become unrecoverable, a bug that code review or tests should have caught): terminate immediately rather than attempting to continue or `recover`. Continuing to run with a violated invariant risks corrupting further state in ways that are harder to diagnose than the original failure. Do **not** try to `recover` from this class of panic to "avoid crashing" — the further a `recover` is from the originating panic, the less that recovery code actually knows about what state might be corrupted (locks possibly still held, invariants possibly still broken elsewhere).
- **Narrow, documented exception**: a package's *internal* implementation may use `panic`/`recover` as a control-flow shortcut for deeply nested logic (a hand-written parser is the classic case), provided the panic **never crosses the package's public API boundary** — the public entry point defers a `recover` that converts the internal panic into a normal returned `error`, and that recovery logic must distinguish "one of our own sentinel panics" from an unrelated panic and re-`panic` anything unrecognized rather than swallowing it.

---

## 5. Concurrency

### 5.0 The governing principle (Tier 1)
Every rule in this section is downstream of one Proverb, stated formally in the Memory Model and informally in Effective Go: **"Do not communicate by sharing memory; instead, share memory by communicating."** Concurrent access to a shared value must be serialized — either by passing the value over a channel so only one goroutine touches it at a time, or by explicit synchronization primitives (`sync`/`sync/atomic`). Effective Go's own caveat, worth keeping: this principle can be taken too far — a simple shared counter is often genuinely better implemented as a mutex-protected integer than as a goroutine-plus-channel construction. The channel-based model is the right *default lens*, not a rule that every piece of shared state must literally flow through a channel. See §11 for the full formal treatment (what "happens before" actually means, and why "it worked in my testing" does not imply "it's race-free").

### 5.1 Mutexes
Uber-only, no Google equivalent (Google's copying rule in §3.13 covers the same ground more generally), kept as-is:
- Zero-value `sync.Mutex`/`sync.RWMutex` is valid — don't use `new(sync.Mutex)` or a pointer field for it.
- If embedding a struct by pointer, the mutex should be a plain (non-embedded) field, never embedded — embedding leaks `Lock`/`Unlock` into the exported API.

### 5.2 Atomics
Uber-only, no Google equivalent, kept as-is. Prefer `go.uber.org/atomic` types over raw `sync/atomic` on primitive types — raw atomics are easy to accidentally access non-atomically since the type system doesn't distinguish `int32` (plain) from `int32` (atomic-only).

### 5.3 Channel size
Uber-only, no Google equivalent, kept as-is. Buffered channels should be size 1 or unbuffered (size 0); any larger size needs explicit justification (what bounds it, what happens if it fills).

### 5.4 Goroutine lifetimes
`[Merged — same principle, different default idiom, both kept as valid]`
Both guides independently arrive at the same requirement — every goroutine must have an observable stop condition and a way for the caller to wait for it — but reach for different default plumbing:
- **Uber**: `chan struct{}` (`stop`) + a second `chan struct{}` (`done`) that gets closed, or a `sync.WaitGroup` for multiple goroutines. Explicit example pattern with `select` on `stop`/ticker.
- **Google**: leans on `context.Context` cancellation + `sync.WaitGroup`, i.e. the goroutine takes a `ctx` and returns when it's cancelled, with `wg.Wait()` at the call site blocking until all spawned goroutines exit.
- **Merged rule (Google-wins as default)**: when a `context.Context` is already in scope (the common case in most real codebases), use context cancellation + `WaitGroup` as the default pattern per Google. Uber's raw `chan struct{}` signal pattern remains valid and is the right choice specifically when no context is available or appropriate (e.g. a long-lived background worker not tied to a single call's cancellation) — kept as the documented fallback, not deprecated.

### 5.5 No goroutines in `init()`
Both agree, no conflict (Uber states it directly; Google implies it via "prefer synchronous functions" + general `init()` caution, see §6.4). If a package needs a background goroutine, expose a type with an explicit `Start`/`Close`-style lifecycle instead of spawning at import time.

### 5.6 Don't fire-and-forget
Uber-only as an explicit named rule, no Google equivalent, kept as-is (though Google's §5.4 goroutine-lifetime guidance covers it implicitly). Every spawned goroutine needs either a predictable stop time or an explicit stop signal, and the caller needs a way to block until it's done. Use `go.uber.org/goleak` in tests for packages that spawn goroutines.

### 5.7 Prefer synchronous functions
Google-only, no Uber equivalent, kept as-is. Functions should do their work and return, not kick off background work and return early. Push the decision to add concurrency to the *caller* — it's easy for a caller to wrap a sync call in `go`, but very hard to strip unwanted concurrency out of an API that imposes it.

### 5.8 Channel direction in signatures
Google best-practices-only, no Tier 3 equivalent, kept as-is. Specify channel direction wherever possible: `func sum(values <-chan int) int` (receive-only) instead of a bidirectional `chan int`. This is a compiler-enforced ownership signal, not just documentation — it makes a whole category of casual mistakes (like a *reader* accidentally calling `close()` on a channel it doesn't own) a compile error instead of a runtime panic.

### 5.9 Core channel idioms (Tier 1 — Effective Go)
Foundational mechanics, no Tier 2/3 equivalent at this level, kept as-is since agents and developers alike need the actual semantics, not just the style rules built on top of them:
- `make(chan T)` is unbuffered (size 0) by default; `make(chan T, n)` is buffered.
- **Unbuffered channels combine communication with synchronization** — a send blocks until a receiver is ready, so a send/receive pair on an unbuffered channel is itself a synchronization point (formalized in §11). This is the mechanism behind the common "signal completion" idiom: `c := make(chan int); go func() { doWork(); c <- 1 }(); <-c` blocks the caller until the goroutine finishes.
- **A buffered channel can act as a counting semaphore**: send to acquire a slot, receive to release it; the channel's capacity caps the number of concurrent holders. This is the mechanism, not just a metaphor — the Memory Model (§11) formally guarantees the Kth receive happens-before the (K+capacity)th send completes, which is exactly the invariant a semaphore needs.
- **`select` with a `default` case never blocks** — it's the standard idiom for "try to send/receive, but don't wait" (e.g. a best-effort free-list: try to grab a pooled buffer, allocate fresh if none is available).
- A goroutine that spawns other goroutines without any way to observe or wait for their completion (`go someFunc()` with nothing else) is the anti-pattern this entire section warns about — see §5.4–§5.6.

---

## 6. Language-level rules

### 6.1 Interfaces
`[Merged — all three tiers address this, largely complementary, no hard conflict]`
- **Effective Go (Tier 1)**, foundational framing: an interface specifies behavior — "if something can do *this*, then it can be used *here*." Small (one-, two-method) interfaces are idiomatic and conventionally named after the method with an `-er` suffix (`Reader`, `Writer`, `Stringer`). Canonical method names (`Read`, `Write`, `Close`, `String`, etc.) carry established meaning — don't reuse one of these names for a method with a different signature or meaning, and conversely, if your type's method genuinely matches an established meaning, use the established name and signature rather than inventing your own (`String() string`, not `ToString() string`). If a type exists only to implement an interface and has no exported behavior beyond it, don't export the concrete type — export only the interface and have the constructor return the interface type directly (`crc32.NewIEEE` returns `hash.Hash32`, not a concrete unexported struct type); this makes swapping the underlying implementation a localized, non-breaking change for callers.
- **Uber** (pointer-to-interface, embedding-related): almost never take a pointer to an interface — pass interfaces by value, let the underlying data be a pointer if needed. Verify interface compliance at compile time with `var _ http.Handler = (*Handler)(nil)` for exported types implementing external interfaces. Avoid embedding types (concrete or interface) in public structs — it leaks implementation details and constrains future evolution (adding a method to an embedded interface, or removing one from an embedded struct, is a breaking change).
- **Google** (design/ownership guidance, no Tier 1/3 equivalent at this level of prescriptiveness, kept as-is): don't create interfaces speculatively — wait for a real need. The **consumer**, not the producer, should typically define the interface, and it should only include the methods that consumer actually uses. Keep interfaces small and unexported unless the interface itself is the product (a shared protocol). Adage: *accept interfaces, return concrete types* — a concrete return type gives the caller the full API surface, and they can still narrow it to an interface themselves at the call site if needed. (This is in some tension with Tier 1's own "export only the interface" example above — reconciled by scope: Tier 1's guidance is about types that exist *purely* to implement one interface with zero other exported behavior; Google's is the general-case default for everything else. When a concrete type genuinely has no purpose beyond satisfying one interface, Tier 1's pattern applies; otherwise, return the concrete type.)
- All three agree, no conflict: don't wrap things in interfaces "just in case" or purely for testability — Google names this explicitly for RPC clients (use real transports, don't hand-roll interfaces around them for mocking, expanded in §7.9 below); Uber's embedding-avoidance is the structural version of the same instinct; this is also a direct application of Proverb #4 ("the bigger the interface, the weaker the abstraction") and Proverb #6 (avoid `any`/`interface{}` params that say nothing).
- **Compile-time interface satisfaction is a runtime mechanism, not automatic** (Tier 1 mechanics, underlies Uber's `var _ http.Handler = ...` rule): Effective Go notes most interface conversions are checked statically at compile time (passing the wrong concrete type where an interface is expected simply fails to compile), but some checks are inherently runtime — e.g. `encoding/json`'s `Marshaler` interface, which the encoder detects via a type assertion (`m, ok := val.(json.Marshaler)`) since it can't know statically whether an arbitrary value implements it. Uber's compile-time-check idiom (`var _ Interface = (*Type)(nil)`) exists specifically to convert what would otherwise be a *silent* runtime gap (a type quietly failing to satisfy an interface it was meant to implement, discovered only when the interface's dynamic type-assertion fails at runtime) into a compile error.

### 6.2 Generics
Google-only, no Uber equivalent (predates Uber's last major revision on this topic), kept as-is. Allowed where they solve a real requirement — don't reach for them if only one concrete type is ever instantiated; add polymorphism later if it turns out to be needed. Don't use generics to build error-handling DSLs or test-assertion frameworks (ties into §7.1).

### 6.3 Pass values vs. pointers
`[CONFLICT → Google wins on framing, not really contradictory]`
Google states this as a general function-argument rule (no Uber equivalent at this scope, though Uber's receiver-related content in Uber's original guide narrows to methods specifically): don't pass a pointer purely to save bytes if the function only ever dereferences it — pass the value directly (this applies to `*string`, `*io.Reader`, etc.). Does *not* apply to large structs or protobuf messages, which should generally be pointers regardless.

### 6.4 Receiver type (value vs. pointer)
Google-only as an exhaustive rule set, no direct Uber equivalent beyond the interface-satisfaction mechanics Uber covers (kept separately — see §6.1's compile-time-interface-check content came from Uber; the receiver *choice* rules below are Google's), kept as-is. Decision list, roughly in priority order:
1. Must mutate the receiver → pointer.
2. Receiver holds fields that can't safely be copied (e.g. `sync.Mutex`) → pointer.
3. Receiver is "large" → pointer, for efficiency.
4. Receiver holds a pointer to something mutable → pointer, to signal mutability to the reader even though a value receiver would technically compile.
5. Receiver is a built-in type, small struct/array, or naturally value-like with no mutable fields → value.
6. Receiver is a slice/map/function/channel and the method doesn't reslice/reallocate → value.
7. When in doubt → pointer.

General rule: keep all methods on a given type consistently pointer- or value-receiver — don't mix.

### 6.5 `init()`
`[Merged — same core position, Google adds detail]`
Both agree: avoid `init()` where possible. Uber gives the fuller "why" (determinism, no dependency on other `init()` ordering, no global/env state access, no I/O) plus concrete before/after examples. Google's treatment is folded into its `don't-panic`/`crypto-rand` sections rather than a standalone rule, no contradiction — Uber's fuller treatment is kept as primary.

### 6.6 Exit in main / exit once
Uber-only, no Google equivalent (Google's Contexts/Flags sections assume this but don't state it as a rule), kept as-is. Only `main()` should call `os.Exit`/`log.Fatal`; everything else returns an error. Prefer a single exit point in `main()` — factor logic into a `run() error` function so tests can exercise it without terminating the test binary.

### 6.7 Must functions
Google-only, no Uber equivalent, kept as-is. `MustXYZ`/`mustXYZ` naming convention for setup helpers that panic/`t.Fatal` on failure — reserved for early program startup (package-level var init) or test setup, never for paths that handle user input or ordinary runtime errors.

### 6.8 Field tags in marshaled structs
Uber-only, no Google equivalent, kept as-is. Any struct field marshaled to JSON/YAML/etc. should carry an explicit tag — makes the wire-format contract explicit and decouples it from Go field-renaming refactors.

### 6.9 Avoid built-in name shadowing
Uber-only, no Google equivalent, kept as-is. Don't name locals/params/fields `error`, `string`, etc. — shadows the predeclared identifier and makes `grep`-based auditing ambiguous, even when the compiler stays silent.

### 6.10 Nil slices
`[Merged — all three tiers cover this, converging on the same rule; used as primary is Google's most complete phrasing, but it's really Proverb #5 ("make the zero value useful") in action]`
`nil` is a valid, usable, zero-length slice — this is not a special case to work around, it's the whole point of designing a good zero value. `len(nil slice)` and `cap(nil slice)` both return 0, ranging over one is a no-op, and `append`ing to one allocates fresh backing storage exactly as it would for a non-nil empty slice. Effective Go demonstrates this directly: a hand-written `Append` function relies on `len`/`cap` being legal on a `nil` slice to decide whether to grow it. Practical rules, agreed by all three tiers: prefer `var s []T` over `s := []T{}` in most cases (Uber/Google); check emptiness with `len(s) == 0`, never `s == nil` (all three, implicitly — Effective Go's own idiom of ranging/`len`/`cap` on nil slices only works because code doesn't special-case nil). Google adds a rule Tier 1/3 don't state explicitly: **don't design APIs where callers must distinguish nil from empty** — if a function returns "no results," represent that as an empty slice (or empty + error), not by making nil mean something semantically different than `[]T{}`.

### 6.11 Naked parameters
Uber-only, no Google equivalent, kept as-is. Add a `/* paramName */` comment for non-obvious literal `bool`/scalar arguments, or better, replace the naked bool with a small named type that can grow beyond two states later.

### 6.12 Raw string literals
Uber-only, no Google equivalent, kept as-is. Use backtick raw strings to avoid hand-escaping quotes, especially in test "want" strings and regexes.

### 6.13 Struct initialization: `var` for zero-value, `&T{}` over `new(T)`
`[Merged — Tier 1 supplies the mechanics, Uber and Google's best-practices converge on the same style rule]`
`var user User` (not `User{}`) when every field is zero-valued — visually distinguishes "deliberately empty" from "populated," per Uber. Google best-practices agrees with the same rationale and extends it: use zero-value declaration specifically when the value is meant to be **ready for later use** (e.g. as an unmarshal target — `var coords Point; json.Unmarshal(data, &coords)`), and prefer `new(T)`/`&T{}` (equivalent — Effective Go states `new(File)` and `&File{}` produce the same result) over hand-rolled field-by-field construction. `&T{Name: "x"}` is preferred over `new(T)` + separate field assignment for consistency with non-pointer struct literal style.

Tier 1 mechanics underlying all of this (Effective Go, "Allocation with `new`"): `new(T)` allocates *zeroed*, not *initialized*, memory — it returns `*T` pointing at a zero value. This is why designing a type's zero value to be immediately useful (Proverb #5) matters so much: `sync.Mutex`'s zero value is already an unlocked mutex, `bytes.Buffer`'s zero value is already an empty, usable buffer, and this property is transitive — a struct composed entirely of zero-value-safe fields is itself zero-value-safe without any explicit constructor.

**When to prefer a pointer type at construction time** (Google best-practices, no Tier 1/3 equivalent at this level, kept as-is): value types are fine for local variables, even ones containing fields that can't safely be copied (§3.13), as long as nothing forces a copy — but if the value will be *returned* from the function, or every access to it will eventually need to take its address anyway, declare it as a pointer from the start rather than constructing a value and taking `&v` at the return statement. Protobuf messages specifically should always be pointer-typed, since `*T` satisfies `proto.Message` while `T` does not.

### 6.14 Map initialization
`[Merged — Tier 1 supplies the underlying "why," Uber's rule is the practical style guidance]`
`make(map[K]V)` for maps that will be populated programmatically — visually distinct from a zero-value (`nil`) map, which is important because **reading from a nil map is fine but writing to one panics** (Effective Go states this as a load-bearing asymmetry, unlike nil slices where both reads and writes/appends are safe). Literal `map[K]V{...}` for a fixed, known-at-init-time set of entries. Capacity hints on `make` where the eventual size is knowable — shared ground with the Performance section (§8) and Google best-practices' size-hints guidance (§8.1 below); note map capacity hints are an *approximation* the runtime uses to size hash buckets, not a hard preallocation guarantee the way slice capacity is.

### 6.15 Format strings outside `Printf`, and naming `Printf`-style functions
Uber-only, no Google equivalent, kept as-is. Declare externalized format strings as `const` so `go vet` can still statically check them. Custom `Printf`-style functions should end their name in `f` (`Wrapf`, not `Wrap`) so `go vet -printfuncs` can find them.

### 6.16 `%q`
Google-only, no Uber equivalent, kept as-is. Prefer `%q` over manually wrapping strings in quotes with `%s` — makes an empty string or control characters visually obvious in output.

### 6.17 `any` over `interface{}`
Google-only, no Uber equivalent, kept as-is. Prefer the `any` alias in new code (Go 1.18+).

### 6.18 Type aliases vs. type definitions
Google-only, no Uber equivalent, kept as-is. `type T1 T2` (definition) to create a genuinely new type; `type T1 = T2` (alias) only for source-location migrations. Don't reach for aliasing otherwise.

### 6.19 `crypto/rand` for keys
Google-only, no Uber equivalent, kept as-is. Never use `math/rand` — even for "throwaway" keys/tokens — it's predictable when unseeded and low-entropy when seeded with a timestamp. Use `crypto/rand`.

### 6.20 Contexts
Google-only, no Uber equivalent, kept as-is:
- `context.Context` is always the **first parameter**, conventionally named `ctx`. Exceptions: HTTP handlers (`req.Context()`), streaming RPC (context comes off the stream), tests (`t.Context()` on Go 1.24+), and true entrypoints (`main`, `init`) which may use `context.Background()`.
- Never store a `Context` on a struct — pass it explicitly through every method that needs it.
- Never define a custom context type or an interface other than `context.Context` in a function signature — no exceptions, per Google. This is specifically about interoperability across an entire codebase, not a style preference.

### 6.21 Avoid mutable globals
Uber-only, no Google equivalent, kept as-is. Prefer dependency injection (a struct field set at construction) over a package-level mutable `var`, including for function-pointer-style "hooks" like a mockable `time.Now`.

### 6.22 Top-level variable declarations
Uber-only, no Google equivalent, kept as-is. Don't restate a type `var _s string = F()` when `F`'s return type already makes it obvious (`var _s = F()`); do specify the type when it differs from the expression's natural type.

### 6.23 Prefix unexported globals with `_`
Uber-only, no Google equivalent, kept as-is (with the `err`/`Err` exception from §4.5). Makes it visually clear at the use site that an identifier is package-scoped, not local.

### 6.24 Embedding — mechanics and when it's actually appropriate
`[Merged — Tier 1 supplies the mechanics and the good use case, Uber supplies the "avoid it in public APIs" caution; genuinely complementary, no contradiction]`
Effective Go's mechanics (Tier 1, foundational): embedding a type in a struct or interface promotes the embedded type's fields and methods to the outer type "for free" — `bufio.ReadWriter` embeds `*Reader` and `*Writer` and thereby satisfies `io.Reader`, `io.Writer`, *and* `io.ReadWriter` without writing a single forwarding method by hand. When an embedded method is invoked through the outer type, the **receiver is still the inner (embedded) value**, not the outer one — embedding is promotion of methods, not inheritance in the OOP sense. Interfaces can only embed other interfaces, not structs. Name conflicts resolve simply: a shallower field/method always hides a same-named one nested deeper; two identically-named fields at the *same* depth is an error only if that name is actually referenced somewhere outside the type definition.

This is genuinely useful — Effective Go's own recommended use is exactly this kind of interface-satisfaction composition, and also simple convenience embedding (`type Job struct { Command string; *log.Logger }` gives `Job` a `Println` method for free). Where Uber's caution (§ previously covered under "Avoid Embedding Types in Public Structs" / "Embedding in Structs" in the Uber-Google merge) applies: **specifically for exported/public API types**, embedding leaks implementation details and constrains future evolution — every exported inner method becomes part of the outer type's permanent contract, and swapping the embedded type later (even for something behaviorally equivalent) is a breaking change. Reconciliation: embedding is a legitimate, idiomatic Go mechanism (Tier 1) — the caution is specifically about doing it in **exported struct types where the outer API surface matters to external callers**, which is a narrower claim than "avoid embedding." Internal, unexported composition and interface embedding (`io.ReadWriter`-style) are exactly what Tier 1 recommends and carry none of Uber's concern.

### 6.25 The blank identifier (Tier 1 — Effective Go, no Tier 2/3 equivalent at this depth)
`_` is a write-only placeholder wherever a variable is syntactically required but the value is irrelevant. Uses worth knowing explicitly, since they show up constantly in idiomatic code and in this document's own examples:
- **Discarding one of several return values**: `if _, err := os.Stat(path); ...` — discard the value you don't need rather than inventing a dummy variable name for it. Never discard an `error` this way to *avoid* handling it — that's a bug, not a style choice (ties directly to §4.6's "handle errors deliberately").
- **Compile-time interface satisfaction checks**: `var _ json.Marshaler = (*RawMessage)(nil)` — this is the formal mechanics underneath Uber's `var _ Interface = (*Type)(nil)` idiom covered in §6.1.
- **Import for side effects only**: `import _ "net/http/pprof"` — makes it explicit in the source that a package is imported *only* to run its `init()`, with no symbols from it ever referenced. Package `math/rand` uses this pattern in some tooling contexts too.
- **Suppressing "declared but not used" during active development**: temporarily assigning an unused import or variable to `_` so code compiles while a function is mid-edit. By convention, place these right after the imports, commented, as a visible reminder to remove them — this is a deliberate, temporary workaround, not a permanent pattern.

### 6.26 Global state — the deepest-treatment topic in Google best-practices
`[Google best-practices-only, no Tier 1/3 equivalent at this depth of treatment, kept as-is in full — this is one of the most substantive additions from best-practices]`
Libraries should not force clients into APIs that depend on package-level mutable state. Default to dependency injection: expose a constructor (`sidecar.New()`) that returns an instance, and have clients pass that instance around explicitly, rather than a package exposing bare functions that mutate a package-level `var` behind the scenes.

**Why this matters in concrete terms**: package-level state creates order-dependent tests (a test that mutates a global for one scenario silently affects every test that runs after it in the same binary), makes it impossible to run independent configurations in the same process, and creates ambiguity about *when* a stateful registration function is safe to call (before flags are parsed? before `init()` finishes? after `main` starts?) — which in turn tends to push the API's error-handling design toward aborting the program (`Must`-style panics) since the author can't guarantee a safe context to return an ordinary error from.

**Litmus test for whether global state is actually safe** (all must hold, or redesign):
- The global state is logically constant after initialization (never mutated again).
- The package's *observable* behavior is stateless — an internal cache is fine precisely because the caller can't tell a cache hit from a miss.
- The state doesn't leak into anything external to the process (a shared file, another service).
- There's no expectation of predictable behavior (e.g. `math/rand`'s legacy global source).

A concrete example that passes the litmus test: `image.RegisterFormat` — registration collisions are rare, callers essentially never need to substitute a test double for a codec, and the decoders themselves are stateless and pure. A concrete example that fails it: a package-level `var client pb.SomeServiceClient` lazily initialized on first use — different callers can't get isolated instances, tests can't cleanly substitute a fake, and there's no way to reset it between test runs without a footgun.

**If you must provide a convenience default-instance API** (e.g. matching `net/http`'s `http.Handle` / `http.DefaultServeMux` pattern): the package must still expose the instance-based constructor as the primary API; the package-level convenience function must be a thin proxy to it; the convenience API should generally only be used from `main`-level/binary code, not imported by other libraries; and the package must document (and ideally provide a way to reset) the global's lifecycle invariants.

### 6.27 Function argument lists, option structs, and variadic options
`[Google best-practices-only for the option-struct/variadic-options mechanics; ties directly into §9.1's functional-options pattern from Uber — genuinely complementary, not a conflict, both are legitimate answers to "too many function parameters"]`
As a function's parameter count grows, individual parameters become harder to tell apart at the call site and easier to transpose by accident (especially adjacent same-typed parameters). Two Google-specific mechanisms for taming this, beyond Uber's functional-options pattern already covered in §9.1:
- **Option struct**: collect some or all arguments into a single struct type, passed as the (typically last) argument — `EnableReplication(ctx, ReplicationOptions{Config: cfg, PrimaryRegions: [...]})`. Self-documenting (every value has a field name at the call site), lets zero-value/default fields be omitted entirely, and the struct can grow over time without changing every call site. Best suited when most callers need to set several of the options, or when the full option set is fairly stable.
- **Variadic options** (Uber's functional-options pattern, §9.1, is this same technique): best suited when most callers need *none* of the options, the option set is large and only sparingly used per call, or third-party/other-package callers need to be able to define their own options.
- Shared rule regardless of which mechanism: never encode a binary or enumerated setting as a presence/absence-only option (`EnableFailFast()`) — always take an explicit parameter (`FailFast(enable bool)`) so callers who need to choose the value programmatically (not just decide whether to include the option literal in source) actually can.
- `context.Context` is never part of an option struct or option value — it's always a separate, explicit leading parameter (§6.20).

---

## 7. Testing

### 7.0 Effective testing — the three qualities every test should maximize
`[New framework tier — Google Testing on the Toilet (TotT), "Effective Testing," May 2014, by Rich Martin. Not part of Tier 1 (not a go.dev doc, not Go-specific) or Tier 2 (not part of the google.github.io/styleguide/go family) as previously defined — treated here as a standalone rationale layer that sits above §7's mechanical rules and explains *why* they exist. It doesn't override anything below; every mechanical rule in the rest of §7 can be read as a specific technique for maximizing one or more of these three qualities.]`

Whether writing one unit test or designing a whole test suite, evaluate it against three qualities in tension with each other. A test that's easy to make good on one axis (an empty test is maximally resilient — it never fails) is often weak on the others; the discipline is holding all three at once.

- **Fidelity** — does the test actually fail when the code under test is broken? A high-fidelity test is sensitive to real defects. Maximize it by covering all the paths through the code and asserting on all the state that actually matters, not just a token subset.
- **Resilience** — does the test *avoid* failing when the code under test isn't actually broken? A resilient test only breaks on a genuinely breaking change; refactors and other non-breaking changes shouldn't force the test to be touched. Maximize it by testing only the exposed API, never reaching into internals; favor stubs and fakes over mocks; don't assert on interactions with a dependency unless that interaction is specifically what's under test. A flaky test has near-zero resilience by definition — it fails (or passes) independent of whether the code is actually correct.
- **Precision** — when the test fails, does it tell you *where* the defect is, not just *that* one exists? A well-written unit test should point at the exact line at fault. Large end-to-end tests are the canonical low-precision case: useful for fidelity, often useless for localizing the actual bug. Maximize precision by keeping tests small and tightly focused, using descriptive test/subtest names that convey exactly what's being validated (this is the same instinct behind §7.2's subtest-naming rules and §7.3's "identify the function/input" failure-message format), and for larger integration-style tests, asserting state at every boundary rather than only at the very end.

**How this maps onto the rest of §7**: §7.1's ban on assertion libraries is partly a resilience argument (custom assertion helpers tend to either over-abort or under-report) and partly a precision argument (a generic `assert.Equal` failure is less precise than a `t.Errorf("YourFunc(%v) = %v, want %v", ...)` that names the function and inputs). §7.2's warning against branching table-test logic is a resilience argument — a table test with `shouldCallX`-style conditional paths is fragile to structural changes even when behavior hasn't actually changed. §7.9–§7.10's test-helper-vs-assertion-helper distinction is precision-driven: keeping the pass/fail decision in the `Test` function itself is what keeps failures attributable to a specific line. When a new testing decision in a real codebase isn't obviously covered by an existing rule below, these three qualities are the right lens to fall back on.

### 7.1 No assertion libraries
`[CONFLICT → Google wins — this is close to an outright ban, Uber has no equivalent stance]`
- **Google**: explicit, strong position — do **not** build or use "assertion libraries" (testify-style `assert.Equal(t, ...)` wrappers). Reasoning: they tend to either abort the test early via embedded `t.Fatalf`/`panic`, or suppress useful pass/fail context; and a proliferation of ad hoc assertion helpers fragments the test-reading experience across a codebase. Package `testing` plus `cmp.Equal`/`cmp.Diff` is treated as sufficient, and is stated as **the only testing framework permitted** in Google's codebase. (Per §7.0 above: this is fundamentally a resilience-and-precision argument — assertion libraries tend to trade away exactly those two qualities for writing convenience.)
- **Uber**: has no stated position on assertion libraries either way (Uber's own examples elsewhere in industry code often use `testify`, but the fetched Uber guide doesn't take a side on this).
- **Merged rule (Google wins, and since Uber is silent this isn't really a compromise — it's an addition)**: don't build custom assertion helpers that combine validation + failure-message production. Prefer direct `if got != want { t.Errorf(...) }` checks, or `cmp.Diff`/`cmp.Equal` for structural comparisons. If your org already depends on `testify` or similar, treat this as the stricter of the two source guides and flag it for a deliberate decision rather than silently keeping existing usage — this is the one rule in this merge most likely to require an actual codebase-wide policy call.

### 7.2 Table-driven tests
`[Merged — both cover this in depth, complementary, no real conflict]`
- **Uber's contribution**: naming convention (`tests` for the slice, `tt` for the loop var, `give`/`want` field prefixes), the ≤3-field-table field-name-omission exception (see §3.6), and an explicit **complexity ceiling** — table tests should not contain conditional/branching assertion logic (`shouldCallX` boolean fields, `setupMocks func(*Mock)` fields); split into separate `Test...` functions once a table needs branching per-row logic. Also covers parallel-test loop variable capture with `t.Parallel()`.
- **Google's contribution**: subtest naming rules (avoid slashes — they collide with `go test -run` and Bazel `--test_filter` path-matching syntax; avoid overly long descriptive names; put long descriptions in a separate field printed on failure, not in the subtest name itself), and **never use the row index as the sole test identifier** — always print inputs or a descriptive name in the failure message.
- Both agree on the core shape (slice of anonymous structs, loop with `t.Run`) and both warn against the same failure mode (opaque failures that require reading test source to debug) — no contradiction, both kept in full.

### 7.3 Useful test failures — general format
Google-only, no direct Uber equivalent (Uber doesn't specify a message format), kept as-is:
- Standard failure format: `YourFunc(%v) = %v, want %v` — identify the function, identify the input, then **got before want** in that order.
- For diffs: always state the diff direction explicitly in the message (`diff (-want +got)`) since convention on this is inconsistent even within Google's own codebase.
- Prefer `t.Error` over `t.Fatal` when checking multiple independent properties of one output, so a single test run surfaces every failing check, not just the first. Reserve `t.Fatal` for when a later check would be meaningless or misleading after an earlier one failed (e.g. don't try to decode output that failed to encode).

### 7.4 Equality comparison
Google-only, no Uber equivalent, kept as-is. Prefer `cmp.Equal`/`cmp.Diff` (`github.com/google/go-cmp/cmp`) over `reflect.DeepEqual` (sensitive to unexported-field changes) for structural comparisons. Needs `protocmp.Transform()` specifically when comparing protobuf messages.

### 7.5 Test error semantics
Google-only, no Uber equivalent, kept as-is. Don't string-compare error messages to determine error *type* — that turns the test into a change-detector against wording. Use `errors.Is`/`errors.As`, or `cmp` with `cmpopts.EquateErrors`. If a test's only concern is "did an error occur," a plain `bool` `wantErr` field compared with `!=` is preferable to pulling in `cmp` machinery for a presence check (an application of Google's "least mechanism" principle from §1).

### 7.6 Test helpers
`[Enriched from primary source — decisions.md "Test helpers" section, now fully retrieved]`
Google-only, no Uber equivalent (Uber's `t.Fatal`-over-panic guidance in §4.8 is adjacent but doesn't cover helper attribution), kept as-is. A **test helper** is specifically a function that performs *setup or cleanup* — failures inside one are assumed to be environment failures (a test DB couldn't start because the machine is out of free ports), not failures of the code under test. Call `t.Helper()` in any function taking a `*testing.T` so failures attribute to the *caller's* line rather than the helper's internals. Parameter ordering convention: `t *testing.T` comes first, then a `context.Context` parameter if the helper takes one, then any remaining parameters — i.e. `func readTestFile(ctx context.Context, t *testing.T, path string) string` is the canonical shape when both are present (this refines §6.20's "context is always first" rule specifically for the test-helper case, where `t` takes precedence). Don't use this pattern to implement assertion libraries — `t.Helper()` existing doesn't license building the pattern §7.1 prohibits; the distinction is about what the function *does* (setup/cleanup vs. validate-and-report), not whether it calls `t.Helper()`. The same guidance largely carries over to benchmark and fuzz helpers, not just `*testing.T`.

### 7.7 Test package placement
`[Enriched from primary source — decisions.md "Test package" section, now fully retrieved]`
Google-only, no Uber equivalent, kept as-is. Two placements, each with a specific mechanical shape:
- **Same-package tests** (the default): file `foo_test.go`, declared as `package foo` (matching the code under test), and — notably — **do not explicitly import the package being tested**, since the test file is compiled as part of that same package. This gives the test access to unexported identifiers, which can mean better coverage and more concise tests, but comes with a documentation caveat: any runnable [example functions](#examples) declared in a same-package test file won't show the package-qualifier prefix a real caller would need to write, since they're already "inside" the package.
- **`_test`-suffixed external package** (`package foo_test`): the sanctioned exception to the no-underscores naming rule (§2.1). Use this specifically when either (a) an integration test doesn't have one single obvious library it belongs to (`package gmailintegration_test`), or (b) defining the test in the same package as the code would create a circular import (a `fireworkstestutil` helper package that itself imports `fireworks` can't be imported *by* a same-package `fireworks` test without a cycle — moving the test to `package fireworks_test` breaks the cycle since it's a distinct compiled package from `fireworks` itself).

### 7.7a Use package `testing` — closing the loop on §7.1
`[New — directly ties together §7.1's assertion-library ban with its source rule, decisions.md "Use package `testing`"]`
Google-only, no Tier 1/3 equivalent, kept as-is. Stated as an explicit, singular rule rather than just implied by §7.1: the standard library's `testing` package is **the only testing framework permitted** in the Google codebase — assertion libraries and third-party testing frameworks are both out, not just discouraged. The stated reasoning is completeness, not lack of alternatives: `testing` already provides top-level tests, benchmarks, runnable examples, subtests, logging, and failure/fatal-failure reporting, and these are designed to compose cleanly with ordinary language features (composite literals, `if`-with-initializer) rather than requiring a DSL on top. This is the rule §7.1's ban is actually derived from — §7.1 describes *why* assertion libraries are a bad idea in Go specifically; this section is the *categorical* policy statement.

### 7.8 Prefer `t.Fatal`/`t.FailNow` over panic in tests
Both agree, no conflict. Uber states this directly with a before/after example; Google's `Must`-function convention (§6.7) covers the same ground for test *helpers* specifically. Merged as one rule: never `panic()` to fail a test — use `t.Fatal`.

### 7.9 `t.Error` vs. `t.Fatal` — precise decision rule
`[Extends §7.3, Google best-practices-only, no Tier 3 equivalent, kept as-is]`
General principle (§7.3 already states this): tests should keep running past a failure to surface everything wrong in one run, not force a fix-rerun-fix cycle. The precise rule for *when* `t.Fatal` actually is correct: reserve it for setup failures the rest of the test cannot meaningfully continue past. In a table-driven test specifically:
- Table-wide setup failure (before the loop starts) → `t.Fatal` is correct, it aborts the whole test function.
- A single row's failure, **without** subtests (no `t.Run`) → use `t.Error` + `continue`, so the loop proceeds to the next row.
- A single row's failure, **with** subtests (inside `t.Run`) → `t.Fatal` is actually fine here, because it only ends the current subtest closure and execution naturally proceeds to the next `t.Run` call — it does not abort the whole table.
- **Never call `t.Fatal` (or `t.FailNow`, `t.Skip*`) from any goroutine other than the one running the test/subtest function** — this is a hard rule documented directly in package `testing`, not a style preference: `t.FailNow` works by calling `runtime.Goexit`, which only unwinds the calling goroutine, so calling it from a spawned goroutine produces undefined/confusing behavior rather than actually failing the test cleanly. Inside a spawned goroutine, use `t.Error` (safe from any goroutine) and `return`, never `t.Fatal`.

### 7.10 Test helpers vs. assertion helpers — the actual distinction
`[Extends §7.1/§7.6, Google best-practices-only, no Tier 3 equivalent, kept as-is]`
Google best-practices draws a sharper line than the earlier merge captured: **test helpers** (setup/cleanup functions — spin up a test DB, load fixture data) are expected to fail only due to *environment* problems, not the code under test, and calling `t.Helper()` in them is appropriate so failures attribute to the caller's line. **Assertion helpers** (functions that check correctness and fail the test on mismatch) are the thing §7.1 says not to build. The distinguishing design test: does the helper *return a value* (an `error`, a `bool`, a diff string) for the `Test` function itself to act on — or does it take a `*testing.T` and call `t.Error`/`t.Fatal` internally? The former is fine and encouraged (e.g. a `cmp.Transformer` or a function returning `(got, error)` for the caller to check); the latter is the assertion-helper anti-pattern, because it moves the pass/fail decision and its message out of the one place (the `Test` function) where it's easiest to read and debug.

**Corollary — designing acceptance/conformance test suites for other people's implementations of your interface** (e.g. a `chesstest.ExercisePlayer(b, p)` that validates any `chess.Player` implementation): this is a legitimate, different use case from an assertion library, because the caller isn't trying to bypass Go's normal test-writing model — they're outsourcing validation of a black-box implementation they didn't write. Even here, keep `t.Fatal` calls inside such a validator reserved for genuine setup failure, and prefer returning an aggregated error/failure list over calling `t.Error` internally, so the *caller's* `Test` function stays the place that actually reports to `testing`.

### 7.11 Use real transports for integration-style tests
Google best-practices-only, no Tier 1/3 equivalent, kept as-is. When testing a component that talks to another one over HTTP/RPC, prefer wiring the real client to a test double of the *server*, rather than hand-writing a fake client. A hand-rolled fake client is its own, separately-buggy reimplementation of nontrivial client behavior (retries, serialization, connection handling) — using the production client against a test server exercises far more real code and avoids that whole extra maintenance burden.

### 7.12 Scope test setup to the tests that need it
Google best-practices-only, no Tier 3 equivalent, kept as-is. Prefer setup that's called explicitly inside each test function that needs it (a `mustLoadDataset(t)` helper called from the top of each relevant test) over a package-level `init()` or unconditional global setup that runs for every test in the file, including ones that don't need it. This keeps `go test -run SomeSpecificTest` fast and avoids unrelated tests failing because of an unrelated fixture's setup breaking. Two sanctioned exceptions, in order of preference:
- **`sync.Once`-amortized lazy setup**, when the setup is expensive, only some tests need it, and it requires no teardown — the cost is paid once regardless of how many tests use it, but tests that don't need it pay nothing.
- **Custom `TestMain`**, only when *every* test in the package needs the same setup *and* that setup requires teardown (e.g. a real database connection). This should be a last resort, not a default — reach for it only after confirming a plain helper or `sync.Once` doesn't fit, since `TestMain` setup runs unconditionally for the whole package regardless of which individual test you're trying to run.

### 7.13 Non-decisions — where Google deliberately declined to pick a side
`[New — decisions.md "Non-decisions" section, only surfaced once the full document was retrieved. No Tier 1/3 equivalent. This is a distinct and useful category: not "unaddressed," but "addressed and explicitly left open."]`
A style guide can't (and, per Google's own framing, shouldn't try to) prescribe everything — some points have been debated by Google's own Go readability mentors without reaching consensus, and are called out explicitly as free choices rather than left ambiguous by omission. Four specific ones, stated in the source:
- **`var i int` vs. `i := 0`** for zero-value local initialization — genuinely equivalent, pick either.
- **`&File{}` vs. `new(File)`** (and equivalently, `map[string]bool{}` vs. `make(map[string]bool)`) for empty composite construction — genuinely equivalent, pick either. (Note: §6.13/§6.14 above give *guidance* on which construction style fits which situation — zero-value vs. populated, map vs. slice capacity hints — but the two spellings within a given situation are still an explicit non-decision, not a resolved rule with a "wrong" answer.)
- **`got`/`want` argument order in `cmp.Diff` calls** — genuinely unresolved even within Google's own codebase (§4.4/§7.3's "always state your diff direction explicitly in the failure message" rule exists precisely *because* this ordering isn't standardized — the explicit direction label is what makes either ordering safe to read).
- **`errors.New("foo")` vs. `fmt.Errorf("foo")`** for a static, non-formatted error string — interchangeable; use whichever reads better locally. (This sits *inside* the space already covered by §4.1's error-selection matrix — the matrix tells you *when* a static-string, non-matched error is the right category; this non-decision says the two ways of constructing that string are equally fine once you're in that box.)

**Why this category matters for a merged guide like this one**: knowing something is a genuine non-decision (rather than a gap in this document, or a point Tier 3/Uber might have a different default on) means don't manufacture a rule here that Google itself declined to set — local consistency (§1) is the correct and sufficient tiebreaker for all four of these, not a "pick the merged doc's preferred one."

---

---

## 8. Performance

`[UPDATED — Google's best-practices doc, now fully retrieved, has directly relevant size-hint content; merged with Uber's original section rather than left Uber-only]`

- **`strconv` over `fmt`** for primitive-to-string conversions (Uber) — measurably faster, fewer allocations (`strconv.Itoa`, not `fmt.Sprint`).
- **Avoid repeated `[]byte(string)` conversions** in hot paths (Uber) — convert once, reuse the resulting slice, rather than re-converting a fixed string on every loop iteration or call.
- **Specify container capacity up front** where the eventual size is knowable (both Uber and Google agree, Google's framing used as primary since it's more precise about the map/slice distinction):
  - `make([]T, 0, n)` / `make([]T, length, capacity)` — slice capacity is a **real preallocation guarantee**: the compiler allocates for the full stated capacity, so `append`s up to that capacity are allocation-free.
  - `make(map[K]V, n)` — a **hint only** (unlike slices): it helps the runtime right-size the initial hash bucket count, reducing but not eliminating allocations as the map grows, since map growth isn't a single contiguous-array resize the way slice growth is.

### 8.1 Google's size-hint framing — an explicit caveat Uber doesn't state
`[Adds nuance, doesn't contradict — kept as-is]`
Google best-practices frames preallocation/size-hinting as something that should follow **empirical analysis**, not be applied reflexively: "most code does not need a size hint or preallocation, and can allow the runtime to grow the slice or map as necessary." It's fine to preallocate when a final size is genuinely known upfront (e.g. converting a map to a slice of known length), but this is explicitly *not* a readability requirement, and Google's own warning is sharp — **preallocating more than needed can waste memory or even harm performance**, so don't guess at a "safe-sounding" capacity without measurement. This directly reinforces (rather than repeats) Uber's own scope note: both sources agree this class of optimization is for measured hot paths, not a blanket mandate, and Google adds the additional caveat that over-preallocating has its own real cost.

---

## 9. Patterns

Uber-only section (Google's `best-practices` doc likely has parallel content, e.g. option-struct guidance referenced in §3.7, but wasn't in the fetched `guide`/`decisions` docs). Kept in full:

### 9.1 Functional options
For constructors/APIs with several optional parameters (rule of thumb: 3+), use the functional-options pattern rather than a long positional-parameter list or a boolean-flag parameter:

```go
type Option interface {
    apply(*options)
}

func WithCache(c bool) Option    { /* ... */ }
func WithLogger(l *zap.Logger) Option { /* ... */ }

func Open(addr string, opts ...Option) (*Connection, error) {
    // ...
}
```

Preferred over a closure-based options implementation specifically because the interface-based version lets options be compared/mocked in tests and lets an option implement extra interfaces (e.g. `fmt.Stringer`) for debuggability.

*(Note: Google's §3.7/§6.27 option-struct guidance — `server.New(ctx, server.Options{Port: 42})` — is not a contradiction of Uber's functional-options pattern; they solve overlapping but distinct problems, and §6.27 above spells out the concrete decision criteria Google's best-practices doc actually gives: option structs suit APIs where most callers set several options and the set is fairly stable; variadic functional options suit APIs where most callers need none of them, the option set is large, or third parties need to define their own options. Both are legitimate; picking between them is a per-API judgment call this document doesn't resolve for you, but §6.27 gives the sharper decision rule Google itself uses.)*

### 9.2 String concatenation — picking the right tool
Google best-practices-only, no Tier 1/3 equivalent, kept as-is. There are several ways to build strings in Go and the right one depends on shape, not just preference:
- **`+`** for a small, fixed number of concatenations — simplest syntactically, no import needed (`key := "projectid: " + p`).
- **`fmt.Sprintf`** once formatting is involved — chained `+` with type conversions (`strconv.Itoa`, `.String()` calls) sprinkled in obscures the result; `Sprintf` reads as one coherent format. If the destination is an `io.Writer`, use `fmt.Fprintf` directly rather than building a string with `Sprintf` just to immediately write it.
- **`strings.Builder`** for building a string incrementally in a loop — `Builder` is amortized-linear-time; repeated `+` or `Sprintf` concatenation in a loop is quadratic, since each step re-copies everything accumulated so far.
- **Backtick raw strings** for constant multi-line text (this is the same rule as §6.12, restated here in its original best-practices context: `usage := \`Usage:\n\ncustom_tool [args]\`` beats a chain of `"...\n" + "...\n" + ...`).

---

## 10. Common libraries (Google-only section, no Uber equivalent)

Kept in full since Uber's guide doesn't address any of this:

- **Flags**: only defined in `package main` (or equivalent) — never as a side effect of importing a general-purpose library. Flag name in snake_case, backing variable in MixedCaps (`poll_interval` flag → `pollInterval` var).
- **Logging**: Google's internal `log` package convention — `log.Fatal` aborts with a stack trace, `log.Exit` aborts without one; no `log.Panic` equivalent exists in their variant. (External readers: substitute your team's structured-logging library's equivalent fatal/exit calls. Note, now confirmed against the complete source document: `decisions.md` contains a `[log/slog]` link definition in this section's source, but it is never actually referenced anywhere in the rendered prose — a dangling/vestigial reference, not a signal of pending guidance. No `log/slog`-specific recommendation exists in this source as of this document's retrieval.)
- **Complex CLIs with subcommands** (best-practices addition, no Tier 1/3 equivalent, kept as-is): if you don't have a strong preference, `github.com/google/subcommands` is the simplest option that's easy to use correctly; `spf13/cobra` is more common outside Google and has more features, but has known pitfalls (in particular: `cobra` command functions should get their `context.Context` from `cmd.Context()`, never construct their own `context.Background()` root context). You don't need a separate package per subcommand — apply the same package-boundary judgment as anywhere else (§2.1's package-size guidance); if the code is usable both as a library and as a CLI, it's usually worth keeping the CLI wrapper and the library logic in genuinely separate packages so the CLI is just one more client of the library.

---

## 11. Documentation and commentary

`[Merged — Tier 1 supplies the mechanics of doc comments, Google best-practices supplies detailed judgment calls about what to document; no conflict, genuinely additive]`

### 11.1 What a doc comment is, mechanically (Tier 1)
A comment immediately preceding a top-level declaration, with no blank line between them, **is** that declaration's documentation — this is how Godoc finds it, and it's why the "no blank line" rule matters mechanically, not just stylistically. Convention: doc comments are full sentences starting with the name of the thing being documented (`// Encode writes the JSON encoding of req to w.`), optionally preceded by an article ("A Request represents..."). A doc comment on a struct applies to the struct as a whole; comments before individual fields (or a comment block before a group of related fields) document just that field/group.

### 11.2 What to document — Google's judgment calls, no Tier 1/3 equivalent at this depth
Not every parameter, field, or option needs its own explanation — document the parts that are **error-prone or non-obvious**, and say *why* they're interesting rather than restating the type signature in prose. Specific recurring cases:
- **Context parameters**: cancellation-interrupts-the-function is implied by the type itself — don't restate it. *Do* document explicitly when: the function returns something other than `ctx.Err()` on cancellation; there's another mechanism that can also interrupt the function (a separate `Stop()` method, say); or the function has unusual expectations about the context's lifetime or attached values (e.g. "the context should not have a deadline") — and if you find yourself writing that last kind of doc comment, treat it as a signal to reconsider the API design, not just a note to add.
- **Concurrency safety**: readers assume read-only operations are inherently safe for concurrent use and mutating operations are not — don't restate either of those defaults. Document explicitly when it's *unclear* whether an operation is read-only (a cache-backed lookup that mutates internal LRU state on a hit, for instance), when the API itself provides synchronization (so callers don't need their own locking), or when the API consumes a user-supplied interface implementation that has its own concurrency contract the implementer must honor.
- **Cleanup obligations**: if a returned value carries a resource the caller must release (`Ticker.Stop()`, `resp.Body.Close()`), document it explicitly — including a short usage snippet if the correct sequencing isn't obvious.
- **Error contracts**: document significant sentinel errors or error types a function can return, including whether a returned concrete error type is used via pointer or value receiver — see §4.10's cross-reference on why that distinction matters for `errors.Is`/`As`/`cmp` comparisons.

### 11.3 Signal boosting
Google best-practices-only, no Tier 1/3 equivalent, kept as-is — but note it's essentially Proverb #13 ("clear is better than clever") applied to comment-writing specifically. Some correct code looks almost identical to a much more common (and subtly different, buggy) pattern — the canonical example is `err == nil` (rare) vs. `err != nil` (everywhere). When code deliberately uses the rarer form, add a short comment calling it out (`if err := doSomething(); err == nil { // if NO error`) so a skimming reader doesn't misread it as the common pattern.

---

## 12. The Go Memory Model (Tier 1 — authoritative on concurrency correctness)

Everything in §5 (Concurrency) is a *style* layer on top of this section's *correctness* layer. If code compiles, passes review against every naming/formatting rule in this document, and is still racy, it is wrong — no amount of style compliance substitutes for the guarantees described here. This section is deliberately kept close to the source document's own structure and language because precision matters more than paraphrase for a correctness specification.

### 12.1 The core instruction, stated exactly as Tier 1 states it
> Programs that modify data being simultaneously accessed by multiple goroutines must serialize such access. To serialize access, protect the data with channel operations or other synchronization primitives such as those in the `sync` and `sync/atomic` packages. **If you must read the rest of this document to understand the behavior of your program, you are being too clever. Don't be clever.**

That warning is deliberate and should be taken at face value by both human developers and coding agents: the formal model exists to let *tooling* (the race detector, the compiler) and language *implementers* reason precisely about edge cases — it is not meant to be a design space that application code operates near the boundary of. Correct concurrent Go code should be obviously correct from §5's style-level rules (own the data, pass it over a channel, or protect it with a mutex/atomic) without needing to reason about the formal "happens before" relation to convince yourself it's safe.

### 12.2 What a data race actually is
A **data race** is a write to a memory location happening concurrently with any other read or write to that same location, *unless* every access involved is an atomic access via `sync/atomic`. In the absence of data races, a Go program behaves as if all its goroutines were multiplexed onto a single processor — this property is called **DRF-SC** (data-race-free programs execute in a sequentially consistent manner). This is the formal justification for §5.0's informal statement that channel/mutex-protected code can be reasoned about "as if" only one goroutine touches the data at a time.

**Practical implication for debugging and code review**: a data race is not "usually harmless" or "probably fine because it's just reading" — reads and writes without synchronization are undefined-behavior-adjacent in Go the same way they are in C/C++, *except* that Go additionally guarantees a race won't corrupt memory arbitrarily for values up to a machine word (each such read observes *some* value actually written, never a torn or "out of thin air" value) — but this guarantee explicitly does **not** extend to multi-word values (interfaces, slices, strings, maps) under a race, where a torn (pointer, length) or (pointer, type) pair absolutely can cause real memory corruption. Always run tests with `go test -race` for any code with concurrent access to shared state; a clean run under casual manual testing proves nothing about race-freedom.

### 12.3 "Happens before" — the relation everything else is built on
Two operations are ordered by **happens before** if there's a chain of either (a) ordinary sequential program order within one goroutine (**sequenced before**) or (b) an explicit synchronizing operation observing another (**synchronized before** — e.g. a channel receive observing the matching send). A read is only guaranteed to observe a particular write if that write happens-before the read *and* no other write to the same location happens-before the read but after the first write (i.e., it's the most recent happens-before write). Without a happens-before relationship between a goroutine's write and another goroutine's later read, **there is no guarantee the read observes the write at all**, no matter how the code appears to behave under casual testing on a particular machine.

### 12.4 What actually establishes synchronization — the concrete rules
These are the specific, load-bearing guarantees that make §5's style rules actually correct (not just conventionally followed):
- **Goroutine creation**: the `go` statement is synchronized-before the start of the new goroutine's execution — so everything sequenced-before the `go` statement is visible inside the new goroutine. (The reverse is *not* true — see below.)
- **Goroutine destruction provides NO guarantee**: a goroutine's exit is not synchronized-before anything. Code that does `go func() { a = "x" }()` and then immediately reads `a` has no guarantee of seeing the write — an aggressive compiler could even legally eliminate the entire goroutine as dead code, since nothing observes it happening. If another goroutine must observe an effect, there must be an explicit synchronization event (channel send/receive, unlock/lock, etc.) — this is precisely why §5.4–§5.6's "every goroutine must have an observable stop signal" rules aren't just about resource leaks, they're about correctness.
- **Channel send/receive**: a send is synchronized-before the *completion* of the corresponding receive (for both buffered and unbuffered channels). For an **unbuffered** channel specifically, the *receive* is also synchronized-before the completion of the corresponding *send* — i.e. unbuffered channel communication is a full two-way rendezvous, not just a one-way handoff. **This distinction matters and is a common source of bugs**: with a *buffered* channel, a goroutine that sends and then immediately does something else has no guarantee the receiver has run yet, only that the value was successfully queued — code relying on "the receiver must have processed this by now" after a buffered send is unsound. Closing a channel is synchronized-before a receive that returns because the channel was closed. Generalized rule for buffered channels: the *k*th receive from a channel of capacity *C* is synchronized-before the completion of the (*k*+*C*)th send — this is the formal basis for §5.9's "buffered channel as counting semaphore" idiom.
- **`sync.Mutex`/`sync.RWMutex`**: for a given lock, call *n* of `Unlock()` is synchronized-before call *n*+1 (or later) of `Lock()` returning. A **failed** `TryLock`/`TryRLock` has no synchronizing effect whatsoever — don't treat "I called TryLock and it returned false" as having observed anything about the lock's state beyond that single boolean.
- **`sync.Once`**: the completion of the one call to `f` inside `once.Do(f)` is synchronized-before the *return* of every call to `once.Do(f)`, including the ones that didn't run `f` themselves. This is what makes `sync.Once`-based lazy initialization (§6.26, §7.12) actually safe across goroutines, not just conventionally reliable.
- **`sync/atomic`**: if atomic operation A's effect is observed by atomic operation B, A is synchronized-before B, and all atomic operations in a program behave as if executed in *some* sequentially consistent global order — matching C++'s sequentially-consistent atomics and Java's `volatile` semantics. This is the formal guarantee underneath §5.2's "prefer typed atomics over raw `sync/atomic` on plain ints" rule: the type-safety concern in §5.2 is about *accidentally* performing a non-atomic access on a variable meant to be atomic-only, silently forfeiting this entire guarantee.
- **Package initialization**: if package `p` imports `q`, completion of all of `q`'s `init` functions happens-before the start of any of `p`'s. Completion of all `init` functions across the whole program happens-before `main.main` starts.

### 12.5 Known-incorrect idioms — patterns that look right and aren't
Directly from the spec, worth stating explicitly because these are exactly the kind of "seems fine, ran clean in testing" code both human developers and coding agents are prone to writing:
- **Double-checked locking without proper synchronization**: guarding a `once.Do(setup)` call behind `if !done { once.Do(setup) }` where `done` is a plain (non-atomic, non-mutex-protected) bool is unsound — observing the write to `done` does not guarantee observing whatever `setup` wrote, because there's no happens-before edge between them absent real synchronization.
- **Busy-waiting on a plain variable**: `for !done { }` spinning on an unsynchronized bool has no guarantee of ever terminating (the compiler is free to assume the loop body never observes an external write, since there's no synchronization telling it otherwise) and even if it does exit, provides no guarantee that other data set before `done = true` is visible after the loop.
- **The general shape of the bug**: observing that *some* flag or pointer has changed does not imply observing *everything* that was written before that flag/pointer was set, unless the flag/pointer's write and read are themselves connected by a real synchronizing operation (channel, mutex, atomic, `Once`). "It's just a pointer/bool assignment, that's atomic-ish anyway" is not a valid argument in Go — the value's atomicity says nothing about ordering guarantees for everything else.

### 12.6 What this means practically for writing and reviewing Go code
- Never rely on "it happened to work when I ran it" as evidence of race-freedom — race conditions frequently only manifest under specific scheduling, load, or hardware, and `go test -race` catches many (not all) at development time.
- Every one of §5's rules (goroutine lifetime signaling, mutex use, channel semantics, "prefer synchronous functions") exists specifically so that ordinary Go code never needs to reason about §12.2–§12.5 directly — treat a need to reason about the formal model as a signal the code's concurrency design is more clever than it needs to be (Proverb #13/#14), and look for a simpler structure first.
- For coding agents specifically: never introduce a goroutine that reads or writes a variable also touched by the spawning goroutine (or any other goroutine) without an explicit, correct synchronization mechanism connecting the two accesses, even if the access "looks read-only" or "only happens once" — both of those intuitions are exactly the ones §12.5's known-incorrect idioms rely on and both are insufficient without a real happens-before edge.

---

## Summary: conflict resolution log

Ordered roughly by where they appear in the doc. "Tier" indicates which source's position ultimately won under the stated hierarchy (Tier 1 > Tier 2/Google > Tier 3/Uber).

| # | Topic | Uber (T3) | Google (T2) | Effective Go / Proverbs / Mem Model (T1) | Resolution |
|---|---|---|---|---|---|
| 1 | Line length | Soft 99-char limit | No fixed limit | **No fixed limit** ("wrap it and indent with an extra tab" if it feels too long) — T1 states this explicitly | **T1 confirms T2; Uber's numeric limit superseded** |
| 2 | Import grouping | 2 groups | 4 groups (std, other, protobuf, blank) | not addressed | **Google's 4-group ordering (T2)** |
| 3 | Import aliasing | Alias on last-path-element mismatch | Stricter: mandatory on collision, protobuf `pb`-suffix, `pkg`-suffix for var collisions | not addressed at this detail | **Google's fuller rule (T2)** |
| 4 | Struct literal field names | "Almost always" named | Required for external types, optional for local types | not addressed at this detail | **Google's precise rule (T2)** |
| 5 | Error wrapping verb (`%w`/`%v`) | `%w` default, `%v` to obfuscate | Detailed context-dependent framework (§4.4) — turned out **not** silent once best-practices was fully retrieved | not addressed at this detail | **Google's fuller framework (T2)**; Uber's heuristic survives as a simplification, not contradicted |
| 6 | `%w` placement in format string | End of string (`...: %w`) | Same rule, **plus** an exception for sentinel/category errors (front-loaded) | not addressed | **Both agree on the general case; Google's sentinel exception added (T2)** |
| 7 | Assertion libraries in tests | No stated position | Explicitly disallowed — "the only testing framework permitted" | not addressed | **Google's ban adopted (T2)** — highest-impact rule if your codebase currently uses testify or similar |
| 8 | `panic` at startup | Allowed generally at init/startup | Prefer `log.Fatal`/`log.Exit`; reserve `panic` for programmer-error invariants; narrow documented exception for internal panic/recover that never crosses a package boundary | Panic is for truly unrecoverable conditions; a library "should" avoid `panic`, with initialization named as the one plausible exception | **T1 and T2 agree on substance; Google's mechanism detail (log.Fatal/Exit split, internal-panic pattern) adopted (T2)** |
| 9 | Pointer vs. value function args | Not addressed at this granularity | Don't pass pointer-only-to-save-bytes for small fixed-size values | not addressed | **Google's rule added (T2)** — no prior position to override |
| 10 | Goroutine-stop plumbing | `chan struct{}` + `WaitGroup` | `context.Context` + `WaitGroup` | Memory model formally underwrites *both* (channel ops and goroutine creation both have defined synchronization semantics) — doesn't prefer one plumbing choice over the other | **Google's context-based default (T2)**; Uber's channel pattern kept as documented fallback when no context is in scope |
| 11 | Exported types that exist only to implement one interface | Avoid embedding/exposing implementation details generally | "Accept interfaces, return concrete types" as the general default | Effective Go's own worked example (`crc32.NewIEEE`) returns the **interface**, not a concrete type, specifically when the concrete type has no purpose beyond satisfying that interface | **Reconciled by scope, not overridden**: T1's pattern applies to the narrow "type exists purely to implement one interface" case; T2's "return concrete types" is the general default for everything else |
| 12 | Container capacity hints / preallocation | Recommends for hot paths, with a scope caveat | Same recommendation, **plus** an explicit warning that over-preallocating has a real memory/perf cost of its own | not addressed | **Merged, both kept**; Google's caveat added as genuine new information, not a contradiction |

Everything not listed above is additive — no direct conflict existed between sources, and all sources' content is retained at its appropriate tier.
