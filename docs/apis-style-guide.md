# API Style Guide — for developers and coding agents

This document is the required reference for designing, writing, and reviewing APIs in this repo
(protos, gRPC service surfaces, and the request/response contracts they imply).

It is derived from `docs/api-style-guides/` — a local fork of the AIP (API Improvement Proposal)
corpus, 68 documents covering resource design, standard methods, field conventions, errors,
versioning, and design patterns.

---

## Source and rebranding notice — read this first

The corpus in `docs/api-style-guides/` originates from **Google's AIP corpus**
(<https://google.aip.dev>) and is maintained here as a **fork for Bosonical**. Each source file
carries a "Fork notice" line saying so.

**Rule for every agent: translate the brand, keep the protobuf.**

1. **"Google" means "Bosonical."** When producing anything — proto comments, docs, commit
   messages, review findings, generated text — render the organization as **Bosonical**. Never
   emit "Google", "Google Cloud", or "GCP" as the organization owning our APIs.
2. **Products named in the sources are Bosonical planned products.** The developer already
   substituted product names ahead of this work. Treat them as ours; don't "correct" them back to
   Google names.
3. **`google.*` protobuf packages are NOT branding — never rename them.** These are protobuf
   standard/common types and annotations, and they must appear verbatim in `.proto` files:
   `google.protobuf.*` (`Timestamp`, `Duration`, `Empty`, `FieldMask`, `Any`, `Struct`),
   `google.api.*` (`http`, `field_behavior`, `resource`, `resource_reference`, `method_signature`,
   `field_info`), `google.rpc.*` (`Status`, `Code`, `ErrorInfo`, …), `google.longrunning.Operation`,
   `google.type.*` (`Money`, `Date`, `TimeOfDay`, `DateTime`, `Interval`). Rewriting these breaks
   the wire format and the build.
4. **Residual Google-infrastructure references are analogies — carry the principle, drop the
   brand.** A handful of substantive mentions survive in the sources because they describe Google's
   own infra rather than an API rule. When you hit one, extract the principle and restate it in
   Bosonical terms. Known locations:

   | Source file | What's there |
   |---|---|
   | `common-components.md` (~L170–180) | Google Apps Script / Google Maps / Actions on Google / Google Shopping common-component packages; a list of non-conformant `google.cloud.*` packages |
   | `file-and-directory-structure.md` (L50, L119–121) | Google Cloud Pub/Sub cited as a multi-service API; `csharp_namespace`/`php_namespace`/`ruby_package` examples using `Google.Cloud.AccessApproval.V1` |
   | `naming-conventions.md` (L41) | UpperCamelCase authority cited as "Google Java Style" |
   | `stability-levels.md` (L19) | alpha/beta/GA compared to Google Cloud launch stages; "GCP imposes its own additional expectations" |
   | `glossary.md` (L74) | "Google Calendar API" as an API-product example |
   | `unicode.md` | Cloud Translation / Spanner / Datastore billing-unit examples |
   | `errors.md` | Compute Engine worked example (`e2-medium`, `us-east1-a`, `compute.googleapis.com`) |
   | `api-design-review-faq.md` | "Pantheon" internal tool, "Cloud AutoML" |

   Host-style identifiers like `library.googleapis.com/Book` or `pubsub.googleapis.com` in examples
   are **illustrative placeholders**, not instructions to use a `googleapis.com` domain.

5. **A few rules in the fork are already Bosonical-specific overrides** of upstream Google
   guidance. Where they appear they win, and they are marked **[Bosonical]** in Part I below:
   - `PATCH` only for Update — Bosonical APIs do not support `PUT` (AIP-134).
   - Field masks for partial responses are a **system parameter**, not a `read_mask` request field
     (AIP-157).
   - `google.iam.v1.*` is importable (AIP-213).
   - In-place breaking changes to a stable component require API Governance approval (AIP-181).

---

## How to use this document

- **Part I** is the working guide. Read the section covering what you're about to change; it
  states the rules with their strength keywords intact (**must / must not / should / should not /
  may**) and cites the owning AIP.
- **Part II** is the routing index — every AIP, grouped by category, with a digest and its file
  path. Open the source file when Part I isn't enough (edge cases, full proto examples, exhaustive
  tables).
- **Appendix A** maps AIP numbers to filenames. You need it: the source files cross-link each
  other by their *original* names (`./0133.md`, `./0203.md#output-only`), and those links are
  **broken** in this fork because the files were renamed to readable slugs.
- Cite findings as **`AIP-131 · standard-methods-get.md`** plus file/line.
- To review changed API work on demand, run **`/api-style-review`**
  (`.claude/skills/api-style-review/SKILL.md`).

**Conflict resolution.** Bosonical-specific overrides (marked **[Bosonical]**) beat upstream
guidance. Otherwise, a more specific AIP beats a general one (AIP-133's Create rules beat AIP-130's
taxonomy). If you must violate a rule, document it — see §11.3.

---

# Part I — API design guide

## §1. Principles and resource-oriented design

**Owning AIPs:** AIP-121 (`resource-oriented-design.md`), AIP-130 (`methods.md`),
AIP-111 (`planes.md`), AIP-9 (`glossary.md`)

### §1.1 The model

An API is a hierarchy of individually-named **resources** (nouns) acted on by a small set of
standard **methods** (verbs). Custom methods exist for what standard methods can't express.

- An API **should** be modeled as a resource hierarchy where each node is a resource or a
  collection of resources; a collection contains resources of *the same type*.
- API designers **should not** expect the API to mirror the database schema — an API identical to
  the DB schema is an anti-pattern that couples the surface to the storage layer.
- The relationship between resources **must** be representable as a **directed acyclic graph**.
  The parent-child relationship **must** be acyclic; a resource has exactly one canonical parent
  (AIP-124). This does not apply to relationships expressed via output-only fields.
- Resource-oriented APIs **must** operate over a **stateless protocol**: each request is
  independent, and resources are directly addressable without a sequence of requests to "reach"
  them.

### §1.2 Minimum method coverage

- A resource **must** support at minimum **Get**, so clients can validate resource state after
  Create, Update, or Delete.
- A resource **must** also support **List**, except for singletons (AIP-156).
- APIs **should** prefer standard methods over custom methods.

### §1.3 Schema consistency

- If the request or response of a standard method (or a custom method in the same service) **is**
  or **contains** a resource, the resource schema **must** be identical across all such methods.
- Canonical shapes: Create — request contains the resource, response *is* the resource; Get —
  response is the resource; Update — request contains the resource, response is the resource;
  Delete — no response body; List — response contains the resources.

### §1.4 Strong consistency on the management plane

For methods on the management plane, completion (success, error, LRO resolution, or a synchronous
return) **must** mean the resource's existence and all user-settable values have reached
steady-state:

- After a successful create (latest mutation), a get **must** return the resource.
- After a successful update (latest mutation), a get **must** return the final values from the
  update request.
- After a successful delete (latest mutation), a get **must** return `NOT_FOUND` — or the resource
  with a `DELETED` state, for soft delete.

### §1.5 Choosing a method category (AIP-130)

Priority order when adding an RPC:

1. Standard methods on collections (List, Create) and resources (Get, Update, Delete)
2. Standard batch or aggregate methods (§4.4, AIP-159)
3. Custom methods — on a resource, on a collection, or stateless (§4.1)
4. Streaming methods (least preferred; hand-written for most clients)

### §1.6 Planes (AIP-111)

- **Management plane:** uniform, resource-oriented; configures and retrieves resources.
- **Data plane:** reads/writes user data; **may** be heterogeneous where throughput, latency, or an
  external spec (e.g. ANSI SQL) demands it.
- Data-plane resources/methods exposed through a resource-oriented management API **must** adhere
  to AIP-131 through AIP-135.
- Declarative clients operate on the management plane exclusively.

---

## §2. Resources: names, types, association, singletons

**Owning AIPs:** AIP-122 (`resource-names.md`), AIP-123 (`resource-types.md`),
AIP-124 (`resource-association.md`), AIP-156 (`singleton-resources.md`),
AIP-128 (`declarative-friendly-interfaces.md`), AIP-129 (`server-modified-values-and-defaults.md`)

### §2.1 Resource name syntax (AIP-122)

- All resource names defined by an API **must** be unique within that API.
- Names follow the URI path schema without a leading slash:
  `publishers/123/books/les-miserables`. Segments **should** alternate collection identifier /
  resource ID, separated by `/`.
- Non-terminal segments **must not** contain `/`; the terminal segment **should not**.
- Names **should** only use characters available in DNS names (RFC-1123); resource IDs
  **should not** use upper-case letters; characters requiring URL-escaping or outside ASCII
  **should not** be used. If Unicode is unavoidable, names **must** be stored in Normalization
  Form C (AIP-210).
- Resources **must not** expose tuples, self-links, or other forms of resource identification.
- All ID fields **should** be strings.

### §2.2 Collection identifiers (AIP-122)

- **Must** be the plural form of the resource noun, concise American English, `camelCase`, matching
  `/[a-z][a-zA-Z0-9]*/`.
- Where no plural exists or singular and plural are identical, use the non-pluralized form;
  **must not** coin plurals (no "infos").
- **Must** be unique within a single resource name (`people/xyz/people/abc` is invalid).
- Nested collections **may** drop the parent prefix (`users/x/events/y`, not `users/x/userEvents/y`).
  If an API does this it **must** do so consistently across every `pattern` and every reference, or
  not at all; the message type and its `singular`/`plural` stay unshortened.

### §2.3 Resource IDs (AIP-122)

- If IDs are user-specified, the API **must** document allowed formats. They **should** conform to
  RFC-1034 and **should** further restrict to `^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$`.
- If IDs are not user-settable, the API **should** document the basic format and upper bounds
  (e.g. "at most 63 characters").
- APIs **may** provide aliases for common lookups (`users/me`), but all data returned **must** use
  the canonical resource name.
- Identifiers **should not** start with a number — that prefix is reserved for Bosonical-generated
  identifiers (AIP-210). Max length **should** be 64 characters.

### §2.4 Declaring names in protos (AIP-122)

- A resource's **first field should** be the resource name, **must** be `string`, and **must** be
  called `name`. It **should** carry `IDENTIFIER` field behavior and the message **should** carry a
  `google.api.resource` annotation.
- Fields **must not** be called `name` for anything else — use `display_name`, `title`, etc.
- A method acting on an existing resource (`GetBook`, `ArchiveBook`): first request field
  **should** be the resource name, **must** be `string`, **must** be called `name`, and **should**
  carry `google.api.resource_reference` (plus `REQUIRED`).
- A method listing or adding to a collection (`ListBooks`, `CreateBook`): first request field
  **should** be `string parent` with `google.api.resource_reference`; use the `child_type` key when
  more than one parent type is possible. Fields **should not** be called `parent` otherwise.
- A field referencing another resource **should** be `string` holding that resource's name, named
  after the message in snake_case; **should not** use a `_name` suffix unless ambiguous
  (`crypto_key_name`); **should** carry `google.api.resource_reference`.
- Such a field **should not** be a message type embedding the resource, except for internal-only
  APIs with tight lifecycle coupling, or the AIP-162 revisions pattern.
- If only the ID component is strictly necessary, use an `_id` suffix (`shelf_id`).
- Resources **may** expose the ID separately (`book_id`) or a system-generated `uid`; both **must**
  be `OUTPUT_ONLY`.
- Cross-API references **should** use the full resource name — a schemeless URI with the owning
  API's service name (`//library.example.com/publishers/123/books/x`) — only when a field can point
  at resources across multiple APIs. The version is deliberately absent from a full resource name.

### §2.5 Resource types (AIP-123)

- Every resource **must** define a resource type as `{Service Name}/{Type}`. The type name **must**
  match the containing API type's name, start uppercase, be alphanumeric only, be singular, and use
  PascalCase.
- APIs **should** annotate with `google.api.resource` (`type`, `pattern`, `singular`, `plural`).
- `singular` **must** be lower camel case of the type; `plural` **must** be the lower camel case
  plural of the singular.
- Pattern variables **must** use `snake_case`, **must not** use an `_id` suffix, **must** match
  `[a-z][_a-z0-9]*[a-z0-9]`, **must** be unique within a pattern, and **must** be the singular form
  of the resource type (`{topic}` for `Topic`).
- Multi-pattern resources: new patterns **must** be appended at the end; existing patterns **must
  not** be removed or reordered (breaks client-library compatibility). Patterns **must** be mutually
  unique once ID segments are stripped.

### §2.6 Resource association (AIP-124)

- A resource **must** have at most one canonical parent, even when it relates to several resource
  types. Other associations go in ordinary fields.
- `List` requests **must not** require two distinct "parents". `List` **must** treat `string parent`
  as required and **must not** add other required arguments; it **should** offer `string filter` for
  the other associations.
- Reference fields **must** accept the same resource name format as the referenced resource's `name`.
- Many-to-many **should** use a repeated field of resource names (AIP-144). Use a join sub-resource
  with two one-to-many associations only when the relationship carries extra metadata or a repeated
  field is too restrictive.

### §2.7 Singleton resources (AIP-156)

- A singleton **must** always exist by virtue of its parent — exactly one per parent — and **must
  not** have a user-provided or system-generated ID. Its name is the parent's name plus one static
  segment (`users/1234/config`).
- Definitions **must** provide both `singular` and `plural`.
- Singletons **must not** define `Create` or `Delete` (implicit with the parent); **should** define
  `Get` and `Update`; **may** define custom methods. **Must not** define `Update` if every field is
  output-only. **May** define `List`, implemented per AIP-159, with the trailing path segment being
  the `plural` form.
- A parent **may** be deleted even when its singleton children exist (AIP-135).

### §2.8 Declarative-friendly resources (AIP-128)

A resource meant for infrastructure-as-code tooling:

- **Must** use only strongly-consistent standard methods for lifecycle management.
- **Should** declare `style: DECLARATIVE_FRIENDLY` on `google.api.resource`.
- **Should** expose an output-only `bool reconciling` if updates take more than a few seconds;
  it **must** be `true` whenever current state ≠ intended state and the system is reconciling,
  regardless of what caused the divergence. A `GET` **must** return current state, not intended.
- **Should not** employ custom methods (AIP-136). **Must** use `Update` for repeated fields
  (AIP-144). **Must** include the standard fields (AIP-148). **Must** have an `etag` (AIP-154).
  **Should** provide `validate_only` change validation (AIP-163). **Should not** soft-delete —
  but **must** if the ID cannot be re-used (AIP-164). **Should** use LROs for create/update
  (AIP-133/134). **Must** provide `etag` on Delete and **should** expose `allow_missing` (AIP-135).

### §2.9 Server-modified values (AIP-129)

- Every field **must** have a single owner: client or server. Server-owned fields **must** be
  `OUTPUT_ONLY`; all others are client-owned and the server **must** respect their value (or
  absence) and not modify them.
- An attribute with a server-decided "effective value" **must** be two fields: the mutable
  user-settable one, and an `OUTPUT_ONLY` companion named by prefixing `effective_`
  (`ip_address` / `effective_ip_address`).
- The value returned for a user-specified field **must** equal what was provided — except that a
  field with a data-type annotation **may** be returned normalized.
- Normalized fields **must** be annotated with `google.api.field_info` (AIP-202). Allowed
  normalization formats: `uuid`, `ipv4`, `ipv6`, `email`.

---

## §3. Standard methods: Get / List / Create / Update / Delete

**Owning AIPs:** AIP-131, AIP-132, AIP-133, AIP-134, AIP-135
(`standard-methods-{get,list,create,update,delete}.md`)

Common to all five: the request message name **must** be the RPC name with a `Request` suffix; the
request **must not** contain other required fields and **should not** contain other optional fields
except those defined by an AIP; `name`/`parent` **should** be annotated required and **must**
identify the resource type via `google.api.resource_reference`; the comment on `name`/`parent`
**should** document the resource pattern.

### §3.1 Get (AIP-131)

- APIs **must** provide a get method. RPC name **must** begin with `Get`; the remainder **should**
  be the singular resource message name (`GetBook`).
- **The response message must be the resource itself** — there is no `GetBookResponse`.
- Response **should** be the fully-populated resource unless a partial response is justified
  (AIP-157).
- HTTP verb **must** be `GET`. URI **should** contain a single variable, `name`, and it **should**
  be the only path variable; other parameters map to query parameters.
- There **must not** be a `body` key. There **should** be exactly one `method_signature` = `"name"`.
- Errors: AIP-193, notably `PERMISSION_DENIED` before `NOT_FOUND`.

### §3.2 List (AIP-132)

- APIs **must** provide `List` unless the resource is a singleton. RPC name **must** begin with
  `List`; the remainder **should** be the plural resource name (`ListBooks`). Request and response
  **must** be the RPC name + `Request`/`Response`.
- HTTP verb **must** be `GET`; the collection maps to the URI path; `parent` **should** be the only
  path variable; the collection identifier (`books`) **must** be a literal string; `body` **must**
  be omitted.
- `method_signature` **should** be `"parent"` for nested resources; for top-level resources, either
  absent or `""`.
- `parent` **must** be included unless the resource is top-level, and **must** identify the listed
  resource type via `child_type`.
- `page_size` and `page_token` **must** be present on every list request (AIP-158). The `page_size`
  comment **should** document the max and the default. Over-max **should** be coerced down;
  negative or invalid **must** return `INVALID_ARGUMENT`.
- The response **must** include **exactly one** repeated field for the resources, and **should not**
  include other repeated fields except where an AIP says so (e.g. `unreachable`, AIP-217).
- `next_page_token` **must** be present; set when more pages exist, **must not** be set on the final
  page.
- Response **may** include `total_size`; it **may** be an estimate and **should** say so; with a
  filter applied it **should** reflect the post-filter count.
- Sorting: **should** use `string order_by`, a comma-separated field list, ascending by default,
  `" desc"` suffix for descending, `.` for subfields. Ordering **should** follow the field type's
  natural comparator unless documented otherwise.
- Filtering: **should** use `string filter` (AIP-160).
- Only add ordering/filtering when there's an established need — both are non-breaking to add and
  breaking to remove.
- Soft-delete APIs **should not** return deleted resources by default and **should** offer
  `bool show_deleted`.
- List **should** return the same results for any user permitted to list the collection (search
  methods are more relaxed).

### §3.3 Create (AIP-133)

- RPC name **must** begin with `Create`; remainder **should** be the singular resource
  (`CreateBook`).
- **Response must be the resource itself** — no `CreateBookResponse`. It **should** be fully
  populated and **must** include any fields provided, unless input-only or a justified partial
  response. Long-running create **must** return `google.longrunning.Operation` resolving to the
  resource.
- HTTP verb **must** be `POST`; collection maps to the URI path; `parent` **should** be the only
  path variable; collection identifier **must** be literal.
- There **must** be a `body` key and it **must** map to the resource field.
- `method_signature` **should** be `"parent,{resource},{resource}_id"` (or `"parent,{resource}"`
  when the ID isn't required; drop `parent` for top-level resources).
- `parent` **must** be included unless top-level, identifying the created resource type via
  `child_type`.
- The resource field **must** be included and **must** map to the POST body. The `name` field on
  the submitted resource **must** be ignored.
- **`{resource}_id` must be included for management-plane resources** and **should** be included
  for data-plane ones. It **must** live on the request message, not the resource. It **may** be
  `OPTIONAL` with a system-generated fallback. Documentation **should** explain the acceptable ID
  format (AIP-122). For REST it is a query parameter.
- Management plane: an API **must** let the user specify the resource ID on creation. Data plane:
  **should**, except where identical records are allowed without disambiguation or the resource
  isn't exposed to declarative clients.
- Duplicate ID **must** error `ALREADY_EXISTS` — but **must** be `PERMISSION_DENIED` if the user
  can't see the conflicting resource.
- Use an LRO (AIP-151) when creation takes longer than is reasonable synchronously; both
  `response_type` and `metadata_type` **must** be specified, with `response_type` = the resource.
  Declarative-friendly resources **should** use LROs and **may** return an already-`done` operation.

### §3.4 Update (AIP-134)

- RPC name **must** begin with `Update`; remainder **should** be the singular resource message name.
- **Response must be the resource itself.** It **should** be fully populated and **must** include
  fields that were sent and covered by the update mask, unless input-only or a justified partial
  response. Long-running update **must** return an `Operation` resolving to the resource.
- **[Bosonical] The HTTP verb is `PATCH`. Bosonical APIs do not support `PUT`.** Upstream permits
  `PUT` for full-replacement-only methods but strongly discourages it, because adding a field to a
  resource turns `PUT` into a breaking change. Do not add `PUT` bindings.
- `{resource}.name` **should** be the only path variable. There **must** be a `body` key mapping to
  the resource field. `method_signature` **should** be `"{resource},update_mask"`.
- The request **must** contain the resource field (**should** be required) and the resource **must**
  contain `name` identifying its type.
- If partial update is supported, the mask **must** be `google.protobuf.FieldMask` named
  `update_mask`, **must** be optional, and its paths are relative to the **resource**, not the
  request. An omitted mask **must** be treated as covering all populated fields. Masks **must**
  support `*` for full replacement.
- Update **should not** trigger side effects beyond changing resource data — side effects belong in
  custom methods. **State fields must not be directly writable** (AIP-216; use a transition method).
- `bool allow_missing` **may** be exposed for client-assigned names:
  - Resource missing → created, all fields applied regardless of the mask; missing/invalid required
    fields → `INVALID_ARGUMENT`.
  - Resource exists and matches → returned unchanged.
  - Resource exists → only masked fields updated.
  - The caller **must** still have update permission; use IAM conditions to prevent create-via-update.
- `string etag` **should** be present where optimistic concurrency matters. If set, the request
  **must** succeed iff it matches and **must** fail `ABORTED` otherwise. `update_mask` has no effect
  on etag handling.
- An API **may** return only the updated fields when others are expensive or impossible to return
  reliably, and **should** document that.
- Missing resource with valid permission **must** be `NOT_FOUND` (404) unless `allow_missing=true`.

### §3.5 Delete (AIP-135)

- RPC name **must** begin with `Delete`; remainder **should** be the singular resource message name.
- Response **should** be `google.protobuf.Empty`; **should** be the resource itself for soft delete;
  **must** be an `Operation` for long-running delete (both `response_type` and `metadata_type`
  specified, even when `response_type` is `Empty`).
- HTTP verb **must** be `DELETE`; `name` **should** be the only path variable; there **must not** be
  a `body`; `method_signature` **should** be `"name"` (etag/force **may** be added).
- Delete **should** succeed iff the resource was present and was deleted; absent resource **should**
  return `NOT_FOUND`.
- **Child resources present → must fail `FAILED_PRECONDITION`**, unless `bool force` is set (which
  an API **should** provide when children are possible). Exception: singleton children never block
  parent deletion, however many singleton types exist under that parent.
- `etag` **may** be accepted; mismatch **must** fail `ABORTED`. Declarative-friendly resources
  **must** provide it.
- `bool allow_missing` **may** be exposed: missing resource → no-op success, `etag` ignored.
  Declarative-friendly resources **should** expose it.
- **Permission must be checked before existence**: no permission → `PERMISSION_DENIED` (403)
  regardless of existence; permission but absent → `NOT_FOUND` (404) unless `allow_missing=true`.

---

## §4. Custom methods, LROs, jobs, batch, import/export

### §4.1 Custom methods (AIP-136)

- Use only for functionality standard methods can't express; **must** operate on a resource where
  the API can be modeled that way.
- Naming: verb + noun. **Must not** contain prepositions ("for", "with"). The verb **should not**
  be one of the standard verbs. **Must not** include `Async`. The suffix `LongRunning` **may**
  distinguish a long-running variant of a standard method (`CreateBookLongRunning`).
- HTTP verb **must** be `GET` or `POST`. `GET` **must** be used for pure data/state retrieval;
  `POST` **may** be used for retrieval when the payload could exceed URL limits, and **must** be
  used whenever there are side effects or mutations.
- The URI **must** use `:` followed by the custom verb (`:archive`), matching the RPC verb, in
  `camelCase` if word separation is needed. The `body` clause **should** be `"*"`.
- Request **should** be RPC name + `Request`; response **should** be RPC name + `Response`, though a
  resource-scoped method **may** return the resource itself.
- Resource-scoped: the resource name parameter **must** be `name` and be the only path variable.
  Collection-scoped: the parent **must** be `parent` and be the only path variable, with a literal
  collection key. Stateless: the scope field **should** be named after the scope resource
  (`snake_case`), the URI **should** put verb and noun after the `:` (`:translateText`, not
  `text:translate`), and billing-relevant stateless methods **must** use `POST`.
- Declarative-friendly resources **should not** use custom methods, except those sanctioned by other
  AIPs and rare genuinely-imperative operations (`Move`, `Rename`).

### §4.2 Long-running operations (AIP-151)

- Methods that might take significant time (rule of thumb: 10+ seconds) **should** return
  `google.longrunning.Operation`. The response **must not** be a streaming response, and the
  `Operation` proto **must not** be copied into individual APIs.
- The method **must** carry a `google.longrunning.operation_info` annotation defining **both**
  `response_type` and `metadata_type`. Both types **must** be defined in the file where the RPC
  appears or a file it imports; cross-package types **must** use fully-qualified names.
- Neither `response_type` nor `metadata_type` **should** be `google.protobuf.Empty` (except a Delete
  response) — define an empty message instead, so fields can be added later.
- Any API returning `Operation` **must** implement the shared `Operations` service and **must not**
  define its own LRO interface.
- Create/Update/Delete **may** return an `Operation`; `response_type` **must** be that standard
  method's normal response type.
- A resource created or deleted via LRO **should** appear in Get/List but **should** signal it isn't
  usable, generally via a state enum.
- Parallel operations: a resource **may** accept them and **may** queue them. A resource that does
  not permit them **must** return `ABORTED` with an explanatory message. Declarative-friendly APIs
  **may** let a newer update preempt in-flight ones, marking the previous `ABORTED`.
- Operation resources **may** expire after completion (rule of thumb: 30 days).
- Errors preventing start **must** be returned as a normal error response (AIP-193). Errors during
  execution **must** go in `Operation.error` as `google.rpc.Status`. Non-terminal errors **may** go
  in metadata and **must** also be `google.rpc.Status`.
- Validate-only responses **must** be one of: a `done=true` Operation with a valid (possibly empty)
  `Any`-wrapped response (`name` **may** be empty); an immediate error; or a `done=false` Operation
  with `name` set that eventually resolves to success or `error`.
- **Changing `response_type` or `metadata_type` is a breaking change.**

### §4.3 Jobs (AIP-152)

- The `Job` resource name **must** end with "Job"; its prefix **should** be a valid RPC name (verb +
  noun).
- The service **should** define all five standard methods for configuring the job, plus a `Run`
  custom method that executes it immediately.
- `Run` RPC name **must** begin with `Run`; remainder **should** be the singular job resource.
  Request **must** be RPC name + `Request`; the method **should** return an LRO resolving to a
  message named RPC name + `Response` containing the run's result. Metadata **may** be any message.
- HTTP verb **must** be `POST`, `body` **should** be `"*"`, the path **should** carry a single
  `name` variable, and the URI **must** end with `:run`.
- A singular `string name` **must** be in the request, **should** be required, and **should**
  identify its resource type.
- Errors preventing the job from starting **must** be a normal error response; errors during
  execution **may** go in metadata as `google.rpc.Status`.
- Executions **may** be stored as a sub-collection with `Get`/`List`/`Delete`; when used, the
  operation returned by `Run` **should** refer to the execution resource.

### §4.4 Batch methods (AIP-231/233/234/235)

Shared rules:

- RPC name **must** begin with `BatchGet` / `BatchCreate` / `BatchUpdate` / `BatchDelete`; remainder
  **should** be the plural resource. Request/response **must** be RPC name + `Request`/`Response`.
- The URI **must** end with `:batchGet` / `:batchCreate` / `:batchUpdate` / `:batchDelete`, over the
  same collection path used for the singular CRUD methods. A dash (`-`) **may** be a parent
  wildcard.
- **BatchGet uses `GET` with no `body`. BatchCreate/Update/Delete use `POST` (never `DELETE`) with
  `body: "*"`.**
- `parent` **should** be included unless the resource is top-level; when set it **must** match every
  child request/resource name or the request fails. It **should** be required when only one parent
  per request is allowed.
- The comment above `requests`/`names` **should** document the maximum number of entries.
- The request **must not** contain other required fields, and **should not** contain other optional
  fields except as defined by an AIP.
- Fields **may** be hoisted from the singular request; if set at both levels the values **must**
  match. Fields that must be unique per entry (customer-provided IDs, `etag`) can't be hoisted.
  `update_mask` is a good hoisting candidate for BatchUpdate.

Per-method:

- **BatchGet:** the operation **must** be atomic — all succeed or all fail; if any covered location
  is down the operation **must** fail. Use `List` (AIP-132) when partial failure is acceptable.
  The request **must** include a repeated resource-name field, **should** be named `names`; empty
  **should** error `INVALID_ARGUMENT`. Batch get **should not** support pagination. The response
  **must** include one repeated field, **in the same order as the request names**.
- **BatchCreate:** request **must** include a repeated field of singular Create requests, **should**
  be `requests`. Response **must** include one repeated field of created resources.
- **BatchUpdate:** same shape, with singular Update requests.
- **BatchDelete:** request **must** include a repeated resource-name field, **should** be `names`.
  Response **should** be `google.protobuf.Empty`, or the updated resources for soft delete.
  **Filter-based matching must not be supported** — see AIP-165 only where genuinely infeasible
  otherwise.
- Nested-request form (`repeated GetBookRequest requests`) is available when a non-name field must
  vary per resource, but is **discouraged** unless necessary.

Atomicity and partial success:

- **Synchronous batch methods must be atomic.** Asynchronous (Operation-returning) ones **may**
  choose atomic or partial success. Simple pass-through database transactions **should** be atomic;
  operations managing complex resources **should** use partial success.
- `metadata_type` **must** be RPC name + `OperationMetadata`, or a `Batch`-prefixed shared name.
- Partial success **must** carry `map<int32, google.rpc.Status> failed_requests` keyed by request
  index; each value **must** mirror what the singular method would return. Transient,
  server-retryable errors **must not** appear there. When every request fails, `Operation.error`
  **must** be set with `code = google.rpc.Code.Aborted` pointing at `failed_requests`.
- Retrofitting partial success onto an existing batch API: the default **must** stay unchanged. If
  it returns an Operation, add `bool return_partial_success`. If it returns a synchronous response,
  the existing version **must not** adopt partial success — cut a new version returning an
  Operation.

### §4.5 Import and export (AIP-153)

- Multi-resource import/export **must** return an LRO unless completion is guaranteed within a few
  seconds. HTTP verb **must** be `POST` with `body: "*"`.
- `parent` **should** be in the URI for multi-resource operations, and **should** accept `-` for
  multiple parents (AIP-159). On import, a specified parent **must** cause rejection of resources
  belonging elsewhere. URI suffix **should** be `:import` / `:export`.
- Single-resource data import/export: the URI field **should** be named after the resource and
  **should not** be `name`; the suffix carries verb + data noun (`:importPages`, `:exportPages`).
- Source/destination configuration **must** live in a `oneof source` / `oneof destination`, even
  with only one option today. Data-level configuration common to all sources **must** be at the top
  level of the request.
- Inline variants **should** be named `InlineSource` / `InlineDestination` and **should** carry a
  repeated field of the resource; the same inline format **must** be used for both directions.
- Import/export **should** report partial failures in the operation metadata, each as a
  `google.rpc.Status`.

### §4.6 Criteria-based delete (AIP-165)

Rarely appropriate. Most APIs **should** use only `Delete` (AIP-135) or BatchDelete (AIP-235) and
**should not** delete by criteria.

- RPC name **must** begin with `Purge`; remainder **should** be plural (`PurgeBooks`). Response
  **must** be an LRO resolving to RPC name + `Response`. `POST`, `body: "*"`.
- `string filter` **must** be included with List semantics (AIP-160) and **should** be required;
  `"*"` **may** mean everything. `parent` **should** be present unless top-level.
- **`bool force` must be included**; when unset the API **must** perform a dry run — returning what
  would be deleted, deleting nothing.
- `int32 purge_count` **should** report the count (may be an estimate, document it).
  `repeated string purge_sample` **should** provide a sample when `force` is false (rule of thumb:
  100 entries, documented as a maximum; document whether it's random or deterministic) and
  **should not** be populated when `force` is true.

---

## §5. List shaping: pagination, filtering, masks, partial results

### §5.1 Pagination (AIP-158)

- **RPCs returning collections must provide pagination from the outset** — adding it later is
  backwards-incompatible.
- Request **should** define `int32 page_size`; it **must not** be required. Unset or `0` → the API
  picks a documented default and **must not** error. Above the maximum → **should** coerce down.
  Negative → **must** return `INVALID_ARGUMENT`.
- The API **may** return fewer results than requested, including zero, even mid-collection.
- Request **should** define `string page_token`; **must not** be required. A changed `page_size` on
  a later page **must** be honored; other changed arguments **should** get `INVALID_ARGUMENT`.
- Response **should** define `string next_page_token`. **Empty `next_page_token` is the only way to
  signal end-of-collection**; if the end has not been reached (or can't be determined in time) the
  API **must** provide one.
- The results field **should** be the first field, number `1`, repeated, holding one page.
- Responses **must not** be streaming. `int32 total_size` **may** be provided, **may** be an
  estimate, and **should** be documented as such.
- **Page tokens must be opaque, URL-safe, and not user-parseable.** Base-64 encoding a transparent
  token is explicitly not sufficient obfuscation. Tokens **must** convey position only — they
  **must not** grant any authorization, and authorization **must** be performed on every request
  regardless. Database-backed tokens **may** expire (rule of thumb: three days), undocumented.
- `int32 skip` **may** be defined; it **must** count individual resources, not pages. An
  unfulfillable `skip` **must** return `200 OK` with an empty result set; a `skip` known to be past
  the end **must not** include `next_page_token`.
- Pagination fields **must** actually be implemented with a non-infinite default — declaring them is
  not enough.

### §5.2 Filtering (AIP-160)

- Filtering **may** be offered on `List`/`Search`; when offered it **should** follow this grammar
  and the request **should** have exactly one `string filter` field.
- Bare literals **should** match anywhere in an object's field values; an API restricting which
  fields are considered **must** document them, and **may** widen the set over time judiciously.
- **Logical operators:** `AND` and `OR` **should** be provided. **`OR` binds tighter than `AND`** —
  the opposite of most languages; docs **should** encourage explicit parentheses but **should not**
  require them.
- **Negation:** `NOT` and `-` **should** be provided; supporting negation means supporting **both**.
- **Comparison:** `=`, `!=`, `<`, `>`, `<=`, `>=` **should** be provided for string, numeric,
  timestamp, and duration fields, and **should not** be for booleans or enums. Field names **must**
  be on the left-hand side.
- Literal formats: enums use the case-sensitive string name; booleans `true`/`false`; numbers
  standard int/float with exponents (`2.997e9`); durations numeric with an `s` suffix (`20s`,
  `1.2s`); timestamps RFC-3339 (`2012-04-21T11:30:00-04:00`) with UTC offsets.
- `*` **should** be supported as a wildcard in string equality comparisons.
- **Traversal (`.`)** **should** be provided for messages, maps, and structs, written with the
  resource's field names; "implicit fields" **must** only be exposed through documented functions.
  Traversal to an undefined field **should** return `INVALID_ARGUMENT`. Undefined map keys **may**
  be permitted (document it). An unset non-primitive intermediate **should** cause the entry to be
  skipped, including for `!=`. `.` **must not** traverse a repeated field except with `:`.
- **The has operator (`:`) must be provided** for collections and messages. Indexed access
  (`e.0.foo`, `e[0].foo`) is invalid. For messages, presence under `:` means a non-default value.
  Top-level field presence is checked with `r:*`. For maps and repeated fields, "unset" and "set
  empty" are indistinguishable.
- Functions **may** be defined with `call(arg...)` syntax and **must** be documented.
- Additional structural limits **may** be imposed but **must** be documented and **must not**
  violate this spec.
- An invalid `filter` **should** error `INVALID_ARGUMENT`; any relaxation **must** be documented.
  Schematic validation covers: referenced fields exist; values match field types; enum values are
  valid; standardized types conform.

### §5.3 Reading across collections (AIP-159)

- A `List` **may** accept `-` as a wildcard collection segment. The URI pattern **must** still be
  declared with `*` and **must not** hard-code `-`. The method **must** document the support.
  Returned resources **must** use canonical names with real parent identifiers.
- A `Get` **may** accept `-` for unique-ID lookup — but **must not**, if child IDs could collide
  across parents. Same pattern/documentation/canonical-name rules apply.
- Cross-parent requests **should not** support `order_by`; if they do, the field **must** document
  that ordering is best-effort.
- If cross-collection listing introduces partial failures, the method **must** follow AIP-217.

### §5.4 Partial responses (AIP-157)

- **[Bosonical] Field masks for partial responses are specified as a system parameter** — an HTTP
  query parameter, header, or gRPC metadata entry — **not** as a request-message field.
  `google.protobuf.FieldMask read_mask` as a request field is **DEPRECATED**; retained guidance
  exists only for legacy and external usage. Do not add `read_mask` to new APIs.
- The parameter's value **must** be a `google.protobuf.FieldMask` and **must** be optional. `"*"`
  **should** be supported and **must** return all fields; when omitted it **must** default to `"*"`
  unless documented otherwise. Changing that default is a breaking change.
- Read masks **may** allow non-terminal repeated fields (unlike update masks).
- Alternatively, an API **may** use a **view enum**: a `view` request field, an enum named ending in
  `-View`, with at minimum `BASIC` and `FULL` values. The `UNSPECIFIED` value **must** be valid (not
  an error) and its behavior **must** be documented; for `List` it **should** default to `BASIC`,
  and for Get/Create/Update/soft-Delete/custom methods to `BASIC` or `FULL`. Define the enum at the
  top level of the file (AIP-126), or nested in the resource if names would collide.
- Fields **may** be added to a view over time; **removing a field from a view is a breaking change**.

### §5.5 Field masks (AIP-161)

- A field mask **must** be `google.protobuf.FieldMask` and **must** always be relative to the
  resource.
- **Read/write self-consistency:** update with a mask then read with the same mask **must** return
  the same data (output-only fields excepted); read with a mask then update with that data and mask
  **must** be a no-op; any mask valid for read **must** be valid for write and vice versa.
- Masks **must** allow specifying a field as a whole or a subfield (`author` and `author.given_name`
  both valid). Map keys **may** be addressed when keys are strings or integers; problematic string
  keys **should** be backtick-quoted (`` reviews.`John Smith` ``). `*` **may** be used across a
  repeated field or map (`authors.*.given_name`).
- **Indexed access must not be permitted** (`authors.0`) and **must** return `INVALID_ARGUMENT`.
- Output-only fields named in an update mask — directly or via a wildcard/parent — **must** have
  their input values ignored, even when cleared, so one mask works for both directions.
- Reading: masks **may** ignore entries pointing at values that can't exist. Writing: **should**
  return `INVALID_ARGUMENT` for such entries, with deletions as a permitted exception.

### §5.6 Unreachable resources (AIP-217)

- A data-retrieval method that can partially fail **must** signal it via a repeated string field,
  **should** be named `unreachable`, annotated `UNORDERED_LIST` (AIP-203), holding
  **service-relative resource names** (never full resource names, URIs, or bare IDs) of the
  unreachable resources or of what blocks reaching them.
- The response **must not** carry any other information about the failure. The service **must**
  provide a way to get a real error via a more specific request, and **must** allow repeating the
  original call with tighter parameters.
- Entries **may** be heterogeneous; the field comment **should** document which resource types can
  appear and note the set may grow. The list **must not** have meaningful ordering.
- If a single unreachable resource makes any result impossible (a List scoped to one unreachable
  parent), the service **must** fail the whole request.
- Unreachable names **must** be included as they're encountered while building a page, scoped as
  tightly as possible, regardless of restrictive paging parameters. Previously-unreachable results
  **must** appear on a later page once availability returns and paging allows.
- If the count exceeds a documented maximum even after up-scoping, the service **must** limit it;
  the maximum **must** be documented on the field, independent of `page_size`.
- Retrofitting: the existing hard-failure default **must** be preserved. Add `bool
  return_partial_success` to the request and `unreachable` to the response **simultaneously**. A
  `return_partial_success=true` request scoped beyond what the API can report **should** get
  `INVALID_ARGUMENT`, and the supported granularity **must** be documented on the field.

---

## §6. Fields: naming, types, behavior

### §6.1 Field naming (AIP-140)

- **Must** use `lower_snake_case`. No word may begin with a number. No leading, trailing, or
  adjacent underscores.
- **Should** be correct American English, precise, and free of unnecessary words (especially
  adjectives that always apply).
- The same concept **should** get the same name across APIs; different concepts **should** get
  different names.
- Repeated fields **must** use the proper plural (`books`); non-repeated **should** be singular.
  Use `repeated Book books`, never `Books books = 1`.
- **Must not** include prepositions (`error_reason`, not `reason_for_error`; `author`, not
  `written_by`). Exception: "per" as part of a unit (AIP-141).
- Well-known abbreviations **should** be used (`config`, `id`, `info`, `spec`, `stats`), as
  **should** unit abbreviations (`distance_km`, `width_px`).
- Adjective before noun (`collected_items`, not `items_collected`).
- **Field names must be nouns, never verbs** (`collected_items`, not `collect_items`; `disabled`,
  not `disable`).
- Booleans **should** drop the `is_` prefix (`disabled`, `required`), except where the bare name
  would be a reserved word (`is_new`).
- Binary content **should** use `bytes`, not manually base64-encoded `string` — unless the data is
  meant to be base64-encoded at rest, where `string` avoids double encoding.
- `uri` for arbitrary URIs (a prefix **may** be added); `url` only when the value can only be a URL.
- **Should** avoid names likely to collide with common language keywords (`new`, `class`,
  `function`, `import`).
- A message **should not** contain a field with the same name as the enclosing message.
- Human-readable name **should** be `display_name` (no uniqueness requirement); `title` **may** be
  used for an official/formal name.

### §6.2 Standard fields (AIP-148)

Use these names for these concepts, and **not** for anything else:

| Field | Type | Meaning |
|---|---|---|
| `name` | `string` | Resource name (AIP-122). Every resource **must** have it; **should** be the first field. |
| `parent` | `string` | Parent's resource name; **should** appear on most `List` and `Create` requests. |
| `display_name` | `string` | Mutable, user-settable human-readable name for UIs. **Should** be ≤ 63 chars, no uniqueness requirement. |
| `title` | `string` | Official/formal name — a more formal `display_name`. |
| `given_name` | `string` | A person's or animal's given name. **`first_name` must not be used.** |
| `family_name` | `string` | A person's or animal's family name. **`last_name` must not be used.** |
| `create_time` | `Timestamp`, output-only | When the resource was created. |
| `update_time` | `Timestamp`, output-only | Last update. User changes **must** refresh it; internal changes **may**. |
| `delete_time` | `Timestamp`, output-only | Soft-delete time; **must** be empty when not soft-deleted. |
| `expire_time` | `Timestamp` | When the resource/attribute stops being valid. **May** be inexact, but the resource **must not** expire earlier. |
| `purge_time` | `Timestamp` | When a soft-deleted resource will be purged. **May** be inexact, but purge **must not** happen earlier. |
| `annotations` | `map<string, string>` | Small amounts of arbitrary client data. **Must** use the Kubernetes limits for wire compatibility; keys **should** be dot-namespaced. |
| `ip_address` (or `*_ip_address`) | `string` | **Must** be `string` and **must** declare `IPV4`, `IPV6`, or `IPV4_OR_IPV6` via `google.api.field_info` (AIP-202). |
| `uid` | `string`, output-only | System-assigned unique ID; **must** be a UUID4 with the `UUID4` format extension. |

Declarative-friendly resources **should** include `display_name`, `create_time`, `update_time`, and
`uid`. Soft-delete resources **should** provide `delete_time` and `purge_time`. Related standard
fields owned elsewhere: `etag` (AIP-154), `request_id` (AIP-155), `filter` (AIP-160),
`validate_only` (AIP-163), revision fields (AIP-162), soft-delete fields (AIP-164).

### §6.3 Field behavior annotations (AIP-203)

- **APIs must apply `google.api.field_behavior` on every field of every message (and sub-message)
  used in a request.** Omission is not allowed (though unannotated fields are read as `OPTIONAL`
  for compatibility). At minimum use one of `REQUIRED`, `OPTIONAL`, `OUTPUT_ONLY`.
  `FIELD_BEHAVIOR_UNSPECIFIED` **must not** be used.
- Exceptions: the AIP-154 `etag` field on a **resource** **should not** carry any behavior
  annotation; `oneof` fields **may** omit it but **should** document behavior in comments.
- Annotations on a nested message are independent of the parent field's annotations.
- **`IDENTIFIER`** **must** be on the `name` field and **must not** be on any other field, including
  references to other resources.
- **`IMMUTABLE`**: on update, a matching value **should** be ignored; a changed value **should**
  error `INVALID_ARGUMENT`. Conditionally-immutable fields **must not** be marked `IMMUTABLE`.
- **`INPUT_ONLY`** **should** only be used inside resource messages; request messages imply it.
- **`OUTPUT_ONLY`**: the server **must** clear any input value and **must not** error on its
  presence; services **must** ignore output-only fields in update masks (AIP-161). **Should** only
  be used inside resource messages; response messages imply it. Output-only fields **may** be empty.
- **`REQUIRED`** means present with a "truthy" value: for primitives, anything other than `0`,
  `0.0`, `""`, empty bytes, `false`; for repeated/maps, at least one entry; for messages, at least
  one truthy field. On create the value **must** be provided; on update it **may** be omitted if
  also absent from the mask. A missing required request field **must** error (usually
  `INVALID_ARGUMENT`). **Should not** be used to mean "always present in a response",
  "conditionally required", or "never user input".
- **`UNORDERED_LIST`** **should** be on a repeated resource field whose element order isn't
  guaranteed to match what the user sent.
- **Breaking:** adding `REQUIRED` to an `OPTIONAL` field; adding a new `REQUIRED` request field;
  adding `OUTPUT_ONLY` to a field previously accepted as input; adding `INPUT_ONLY` to a field
  previously emitted; adding `IMMUTABLE` to a mutable field; removing `OUTPUT_ONLY`; removing
  `IDENTIFIER`.
- **Non-breaking:** adding `OPTIONAL`; adding `IDENTIFIER` to `name`; `REQUIRED` → `OPTIONAL`;
  `OUTPUT_ONLY`/`IMMUTABLE` → `IDENTIFIER`; removing `REQUIRED`; removing `INPUT_ONLY` from a field
  already excluded from responses; removing `IMMUTABLE`.

### §6.4 Enumerations (AIP-126)

- Enums **may** be used for value sets expected to change infrequently (rule of thumb: no more than
  one new value a year). Enums **should** document whether they're frozen or expected to grow.
- All values **must** be `UPPER_SNAKE_CASE`. The first value **should** be the enum name +
  `_UNSPECIFIED` (`FORMAT_UNSPECIFIED = 0`) — unless a genuinely useful zero value exists
  (`UNKNOWN`), in which case use that instead of having both.
- Single-message enums **should** be nested in that message, declared immediately before use, with
  values **not** prefixed by the enum name. Multi-message enums **should** be package-level, defined
  at the bottom of the file (AIP-191), with values prefixed by the enum name to avoid collisions in
  languages that hoist them.
- **Use a `string` instead** when the value set changes frequently (document the allowed values;
  values **should** be `kebab-case`), or when a widely-adopted standard representation exists
  (BCP-47, media types) — enums **should not** compete with those.
- Shared enums across APIs **must** use identical value/integer assignments.
- Booleans **may** be used where no further flexibility will be needed; the default **must** be
  `false`. Where "false" and "unset" must be distinguished, prefer an enum (or
  `google.protobuf.BoolValue`).

### §6.5 Repeated fields (AIP-144)

- **Must** use a plural name. Where singular and plural are identical, use the dictionary word
  rather than coining a plural.
- **Should** have an enforced upper bound (rule of thumb: 100 elements); if the data could grow
  large, use a sub-resource instead.
- **Must not** inline the body of another resource — carry resource names instead.
- Use a scalar type only when additional data is certainly never needed; otherwise use a message
  proactively to avoid parallel repeated fields.
- **Declarative-friendly resources must use `Update` for repeated fields and must not add
  `Add`/`Remove` methods.**
- Where atomic modification is genuinely required, define `Add`/`Remove` custom methods: RPC name
  **must** begin with `Add`/`Remove`, remainder **should** be the singular field name; request
  **must** be RPC name + `Request`; response **should** be the resource itself (fully populated),
  or RPC name + `Response` if extra context is needed; HTTP verb **must** be `POST`; URI **must**
  end with `:add*` / `:remove*` using the snake-case singular field name; `body` **should** be `"*"`.
  The path variable **should** be the resource's own name (`book`), not `name`/`parent`, and be the
  only variable. Adding an existing value **must** error `ALREADY_EXISTS`; removing an absent value
  **must** error `NOT_FOUND`.
- Prefer a map + `Update` for complex structures with a primary key; added/removed data **should**
  be primitive (usually `string`).

### §6.6 Quantities (AIP-141)

- A quantity with a unit **must** carry the unit as a field-name suffix (`distance_miles`);
  abbreviations **should** be used where accepted and **should not** be pluralized (`distance_km`).
- Item counts **should** use the `_count` suffix (`node_count`), **not** a `num_` prefix.
- **Fields must not use unsigned integer types.**
- Compound units: unabbreviated components **must** be underscore-separated; abbreviated ones
  **should not** be unless ambiguous; metric prefixes **must not** be separated from their base
  unit; the final component **should** be plural unless abbreviated (`energy_kwh`,
  `energy_kw_fortnights`).
- Inverse units go last, joined by "per", singular (`speed_miles_per_hour`, `event_count_per_hour`)
  — unless a named derived unit exists (use "hertz").
- Specialized quantity messages (`google.type.Money`) **may** be used; the message name **should**
  be the field-name suffix where intuitive.

### §6.7 Time and duration (AIP-142)

- Absolute points in time **should** use `google.protobuf.Timestamp` with a `_time` suffix
  (`_times` when repeated). Activity timestamps use the imperative form (`publish_time`) and
  **should not** be past tense (`published_time`, `created_time`, `last_updated_time`).
- Spans **should** use `google.protobuf.Duration`. A span relative to a point in a stream **should**
  use `Duration` with an `_offset` suffix, and **must** document what the offset is relative to.
- Calendar dates **should** use `google.type.Date` (`_date`); wall-clock times
  `google.type.TimeOfDay` (`_time`); timezone-aware civil timestamps `google.type.DateTime`
  (`_time`).
- Non-standard representations **may** be used only for legacy/compat reasons: integer fields
  **should** name the meaning plus a unit suffix (`send_time_millis`), string fields the meaning
  only. In every such case the format and rationale **must** be documented.

### §6.8 Standardized codes (AIP-143)

- Where a standard code exists, use it for input and output, with the appropriate type (usually
  `string`), and **must not** use an enum — even for a small subset. The field **must** indicate
  which standard it follows. The name **should** end in `_code` or `_type` unless a clearer suffix
  exists. Validation **should** be case-insensitive unless ambiguous; output **should** use
  canonical case (`en-GB`).

| Concept | Standard | Field name |
|---|---|---|
| Media/content type | IANA media types | `mime_type` (legacy name, mandated) |
| Country/region | Unicode CLDR region codes | **`region_code`** (never `country_code`) |
| Currency | ISO-4217 | `currency_code` (use `google.type.Money` for an amount) |
| Language | IETF BCP-47 | `language_code` |
| Time zone | IANA TZ | `time_zone` |
| UTC offset | ISO-8601 | `utc_offset` |

### §6.9 Ranges (AIP-145)

- Ranges **should** be two same-typed fields prefixed `start_` and `end_`, **inclusive start,
  exclusive end**.
- Timestamp ranges fitting those semantics **should** use `google.type.Interval`; separate fields
  **may** be used where the containing message already describes an interval and nesting is
  undesirable.
- Where colloquial precedent is inclusive on both ends (dates, days of the week), use `first_` /
  `last_` prefixes — only for those ranges.
- **Every range must be documented as inclusive or exclusive.**

### §6.10 Generic fields (AIP-146)

- Generic fields **should** be rare; prefer the *least generic* option that works.
- `oneof` **should** generally be preferred (type-safe, semantically clear), including for
  same-typed alternatives. It's ill-suited to unbounded option sets. Adding a field to an existing
  `oneof` is non-breaking; **moving a field into or out of a `oneof` is breaking**.
- Maps **may** be used when keys are unknown or user-determined.
- `google.protobuf.Struct` **may** carry arbitrary nested JSON; use JSONSchema if the service must
  reason about its shape.
- **`google.protobuf.Any` should not be used** unless everything else is infeasible.

### §6.11 Sensitive fields (AIP-147)

- Required sensitive data **should** be an input-only field with no output counterpart.
- Optional sensitive data **should** have an output-only boolean with a `_set` suffix indicating
  presence.
- Where the value must be identifiable without being readable, a same-typed field with an
  `obfuscated_` prefix **may** replace the boolean. The obfuscation method is out of scope.

### §6.12 Unset field values (AIP-149)

- Use proto3 `optional` on a primitive **if and only if** distinguishing "explicitly set to the
  default" from "unset" is necessary — in practice, only for integers and floats. Usually a design
  that avoids the distinction is better.
- `optional` (presence) and `REQUIRED` (behavior) are orthogonal and **may** coexist.
- **Adding or removing `optional` on an existing field is backwards-incompatible.**

### §6.13 Field format annotations (AIP-202)

- `google.api.field_info` is only needed where this or another AIP calls for it.
- `UUID4`, `IPV4`, `IPV6`, `IPV4_OR_IPV6` **must** only be applied to `string` fields. Services
  **may** normalize (lowercase a UUID, strip leading zeros from IPv4 octets, RFC-5952-compress
  IPv6).
- **Equivalence comparison must not be primitive text comparison** — use an RFC-4122 / RFC-791 /
  RFC-4291 compliant implementation.
- Adding a format specifier to an existing unspecified field is **not** backwards compatible unless
  the field has always conformed; changing an existing specifier is not compatible in all cases.
- Any new `FieldInfo.Format` value **must** be governed by an IETF-approved RFC or a
  Bosonical-approved AIP.

### §6.14 States (AIP-216)

- Lifecycle state **should** be an enum called `State` (or ending in `State`), nested in the message
  it describes. **Use `State`, never `Status`** — that's reserved for HTTP/gRPC statuses.
- Values: `ACTIVE` for usable; past participles ending `-ED` for terminal states (`SUCCEEDED`,
  `FAILED`, `DELETED`, `SUSPENDED`); present participles ending `-ING` for transitional states
  (`RUNNING`, `CREATING`, `DELETING`) that resolve without further user action. Only add states with
  a real customer use case.
- The state field **should** be output-only.
- **State must not be directly writable via Create or Update** — use a state-transition custom
  method: verb + singular resource name; request = RPC name + `Request`; response **should** be the
  resource (or an LRO resolving to it); `POST`; URI `:verb` in `camelCase`; `body: "*"`; the request
  `name` field **should** be the only path variable. A disallowed transition **must** error
  `FAILED_PRECONDITION` (400).
- Zero value **should** be `<EnumName>_UNSPECIFIED = 0`; resources **should not** surface it and it
  **should not** be used. Values **should not** carry a blanket `STATE_` prefix, except the default.
- Top-level enums in one package **must not** share value names (C++ flattens them).
- **Adding a new state is not a breaking change** — documentation **should** actively tell users to
  code defensively against new values. Local consistency beats global consistency when precedent
  conflicts.
- Don't use a `State` enum where a simpler field fits (prefer `delete_time` over a two-value
  `ACTIVE`/`DELETED`).

### §6.15 Unicode (AIP-210)

- In all documentation and length limits, **"character" must mean a Unicode code point**, and string
  length limits **must** be measured and enforced in code points.
- Billing/quota **may** use code points or bytes; undefined means per character.
- Unique identifiers **should** be limited to ASCII (`[a-zA-Z][a-zA-Z0-9_-]*`), **should not** start
  with a number (reserved for Bosonical-generated IDs), and **should** cap at 64 characters.
- If full Unicode is allowed, inputs not in **Normalization Form C must be rejected**. Unique
  identifiers **must** be stored in NFC, **must** be normalized to NFC before uniqueness checks, and
  two byte sequences with the same NFC form **must** be treated as identical.

---

## §7. Errors

**Owning AIPs:** AIP-193 (`errors.md`), AIP-211 (`authorization-checks.md`),
AIP-194 (`automatic-retry-configuration.md`)

### §7.1 Error shape (AIP-193)

- Services **must** return `google.rpc.Status` and **must** use canonical `google.rpc.Code` values.
  `Status.code` **must** be the numeric enum value (`5` = `NOT_FOUND`).
- **Every error response must include an `ErrorInfo` in `details`.** Each detail payload type
  **must** appear at most once (no two `BadRequest`s; a `BadRequest` plus a `PreconditionFailure` is
  fine). Use the standard payloads (`ErrorInfo`, `BadRequest`, `PreconditionFailure`,
  `QuotaFailure`, `RetryInfo`, `LocalizedMessage`, `Help`) where feasible.
- `ErrorInfo.reason` **must** be ≤ 63 chars matching `[A-Z][A-Z0-9_]+[A-Z0-9]`.
  `ErrorInfo.domain` **must** be globally unique — normally the service name. The same
  `(reason, domain)` pair **must** be used for the same error and **must not** be used for logically
  different errors; "the same" is judged by the client action needed to resolve it.
- `ErrorInfo.metadata` keys **must** be ≤ 64 chars matching `[a-z][a-zA-Z0-9-_]+`. **Any dynamic
  detail appearing in an error message must also be in `metadata`.** Extra programmatic context
  **may** be added. New keys **may** be added over time, but **every key ever emitted must continue
  to be emitted** (possibly empty) — clients rely on their presence.

### §7.2 Error message text (AIP-193)

- `Status.message` is the developer-facing debug string and **should** be English. Messages
  **should** be brief but actionable, in plain descriptive language; **should not** assume API
  expertise; **must not** assume knowledge of the implementation. Extra information belongs in
  `details`; a link **should** be offered when more is needed.
- `LocalizedMessage` **should** be localized to the user's locale; both `locale` (BCP-47) and
  `message` **must** be populated. It **should** contain the complete resolution — if it can't,
  that information **must** go in a `Help` payload.
- `Help` **must** be provided when other text can't give enough actionable context or there are
  multiple failure points, **in addition to** (not instead of) a clear problem statement.
  `Help.description` **must** be plain text suitable as hyperlink text; `Help.url` **must** be an
  absolute URL. A page covering multiple errors **must** be navigable via `ErrorInfo.reason`. Public
  services' help links **must** be reachable without authentication.
- **Stability:** if an RPC has always returned machine-readable `ErrorInfo`, `Status.message` **may**
  change over time. Otherwise (brownfield) `Status.message` **must** stay byte-stable for a given
  error — add a `LocalizedMessage` instead of editing it. `LocalizedMessage` content **may** change.

### §7.3 Permission and existence ordering (AIP-193, AIP-211)

- **Services must check authorization before validating any request.**
- If a request fails authorization for any reason, the service **must** error `PERMISSION_DENIED`
  (403). **Permission must be checked before existence.**
- The message **should** be phrased to avoid leaking existence:
  `Permission '{p}' denied on resource '{r}' (or it might not exist).`
- With valid permission and a missing resource/parent → `NOT_FOUND` (404).
- When authorization can't be determined because the resource doesn't exist, the service **should**
  check the parent's read-children permission and return `NOT_FOUND` if that passes.
- Where two operations carry two different permissions that could each reveal existence, the service
  **should** check only the permission for the operation being called, and **should not** "help out"
  by checking related permissions — those algorithms leak.

### §7.4 Partial errors (AIP-193)

- **APIs should not support partial errors** — they force specialized client error handling.
- Methods that genuinely need them **should** use LROs, putting partial-failure information in the
  operation metadata, with each error still a `google.rpc.Status`.

### §7.5 Automatic retry (AIP-194)

- Clients **should** auto-retry only requests that are unary, non-transactional, and safe to repeat.
  Transactional requests **should** be retried at the application level, restarting the whole
  transaction block.
- **Retryable:** `UNAVAILABLE`.
- **Never auto-retry:** `OK`, `CANCELLED` (a cancellation **must** be honored), `DEADLINE_EXCEEDED`
  (the deadline **must** be honored), `INVALID_ARGUMENT`, `DATA_LOSS` (unrecoverable — surface it),
  `NOT_FOUND`, `ALREADY_EXISTS`, `PERMISSION_DENIED`, `UNAUTHORIZED`, `UNAUTHENTICATED`,
  `FAILED_PRECONDITION`, `OUT_OF_RANGE`, `UNIMPLEMENTED`.
- **Generally not retryable, context-dependent:** `RESOURCE_EXHAUSTED` (quota exhaustion may take
  hours and has billing implications; retryable only for short-lived non-quota constraints);
  `INTERNAL` (usually a bug — surface it); `UNKNOWN` (may not be safe — surface it); `ABORTED`
  (retry the enclosing transaction, not the individual request).
- Client- and bidirectional-streaming RPCs are out of scope.

---

## §8. Versioning, stability, and backwards compatibility

### §8.1 Versioning (AIP-185)

- Every API interface **must** carry a **major version**, at the end of the protobuf package and as
  the first path segment of the REST URI.
- **Minor and patch versions must not be exposed** — `v1`, never `v1.0`, `v1.1`, or `v1.4.2`.
- A new major version **must not** depend on a previous major version of the same API. An API
  **must not** depend on other APIs except as allowed by AIP-213 and AIP-215.
- Different versions **must** be usable simultaneously in one client application for a reasonable
  transition period, and an old version **must** get a well-communicated deprecation period before
  shutdown.
- **Channel-based versioning (preferred):** at most one channel per stability level. Alpha and beta
  **must** append the stability level (`v1alpha`, `v1beta`); stable **must not** (`v1`). Beta's
  functionality **must** be a superset of stable's; alpha's **must** be a superset of beta's.
  Deprecated functionality **must not** graduate alpha → beta or beta → stable. Beta functionality
  **may** be removed after a deprecation period (recommended: 180 days); alpha-only functionality
  **may** be removed without notice.
- **Release-based versioning (legacy):** `v1beta1`, `v1alpha5`. Each release **may** take
  backwards-compatible updates in place; beta breaking changes **should** increment the release
  number. Alpha releases **may** be shut down at any time; beta **should** get ~180 days.
- Deprecate with `option deprecated = true`.
- Visibility labels are case-sensitive, **should** be UPPER case, default to an implicit `PUBLIC`,
  **may** be comma-separated (logical OR), and a single request can specify at most one.

### §8.2 Stability levels (AIP-181)

- **Alpha:** users **must** be tolerant of change and **should** be a curated, individually
  contactable group. Breaking changes **must** be expected; there is no stability expectation.
- **Beta:** **must** be complete and ready to be declared stable, subject to public testing;
  **should** be publicly available, not allowlisted. **Should** be as stable as possible but
  **must** be permitted to change. Breaking changes **must** come only after a reasonable
  deprecation period, which **must** be defined when the component is marked beta. Beta **should**
  be time-boxed (rule of thumb: 90 days) with the timeframe specified up front.
- **Stable:** **must** be fully supported for the lifetime of the major version, with **no** breaking
  changes. When one becomes necessary, the producer **should** cut the next major version and start
  a deprecation clock. Turndown **must** follow a formal process defined when the component was
  marked stable. A rare isolated breaking change **may** deprecate a component, but it **must**
  still be supported for the normal turndown period.
- **[Bosonical] An in-place breaking change to a stable API carries equal or greater gravity than
  cutting a new major version, and requires API Governance team approval.**
- In genuine emergencies (security, regulatory), any component **may** change in a breaking manner
  with no deprecation promise.

### §8.3 What is and isn't a breaking change (AIP-180)

Compatibility has three dimensions, all of which **must** hold: **source** (old code still compiles
and runs against the new client library), **wire** (serialization still matches), and **semantic**
(behavior still matches reasonable expectations). Old clients **must** work against newer servers of
the same major version.

**Allowed within a major version:**
- Adding interfaces, methods, messages, fields, enums, and enum values — provided existing surface
  keeps behaving identically.
- Adding enum values freely to request-only enums; adding to response/resource enums with caution
  and documentation.

**Breaking — must not do within a major version:**
- Removing any existing component. Renaming = remove + add: add the new one, **must not** remove the
  old; if both could be set, the behavior **must** be specified.
- Moving components between proto files (breaks generated imports even though wire-compatible).
- Moving a field into or out of a `oneof`.
- Changing the type of an existing field or message, even to a wire-compatible type.
- Adding a new **required** field to an existing request or resource. Any client-populated field
  **must** default to the pre-existing behavior. Any server-populated field **must** keep being
  populated, even when redundant.
- Changing a resource's name — this holds **even across major versions**; the same resource name must
  work in v1 and v2. The set of valid resource names **should not** change in either direction.
- Increasing the upper bound on a `string` field's size (**should** be treated as incompatible).
  Padding to a fixed size is allowed but **must** be documented.
- Changing visible behavior or semantics in ways likely to break reasonable code, even when
  undocumented.
- Changing the format or construction algorithm of an existing field's value — **including
  `OUTPUT_ONLY` fields** (e.g. an `ip_address` going from IPv4 to IPv6).
- Changing a field's static default value, or changing how a defaulted field is serialized.

Also breaking, from other sections: adding/removing `optional` (§6.12); the field-behavior changes
in §6.3; changing an LRO's `response_type`/`metadata_type` (§4.2); removing a field from a view
(§5.4); reordering or removing resource `pattern` entries (§2.5); adding a language namespace option
where defaults were previously relied on (§9.3).

### §8.4 Change validation (AIP-163)

- A method **may** offer a dry run via `bool validate_only`, returning the same status code,
  headers, and body as a real call would.
- The API **must** perform permission checks and all other validation a live request would, and the
  validated request **must** fail if the real one would.
- Fields infeasible to produce during validation (auto-generated IDs) **should** be omitted.
- **Declarative-friendly resources must include `validate_only` on every mutating method.**

### §8.5 External software dependencies (AIP-182)

- Services **should** allow creating resources on any currently-supported LTS version of the
  external software, and **may** allow non-LTS versions.
- Services **should not** indefinitely allow creation on end-of-life versions, and **may** offer a
  transition period. Removing EOL creation support is **not** considered a breaking change under
  AIP-181, even though it functionally is.
- Services **must** notify users whose resources are approaching EOL; **should** let existing
  resources remain; **may** warn about the risks; **should not** proactively remove them or restrict
  them absent critical security concerns.
- Officially supporting an EOL version **may** be done for business reasons, but the service then
  **must** take on patching and maintenance.

---

## §9. Proto files, naming, documentation, common components

### §9.1 Naming conventions (AIP-190)

- Names **should** be straightforward, intuitive, consistent, and in correct American English
  ("license", "color"). Accepted short forms **may** be used ("API").
- **Definitions must use UpperCamelCase.** (The fork cites Google Java Style as the casing
  authority — read it as the casing rule, not a brand endorsement.)
- Use familiar terminology ("delete", not "erase"). Use one name per concept and different names for
  different concepts; avoid overloading. Avoid names that are overly general within the API or the
  wider Bosonical ecosystem. Names colliding with common language keywords **may** be used but
  attract review scrutiny.
- **Interface (proto `service`) names should** be an intuitive noun (`Calendar`, `BlobStore`) and
  **should not** collide with well-established language/runtime concepts (`File`); use an `Api` or
  `Service` suffix to disambiguate. Note "interface name" ≠ "service name" (the deployed host).
- **Method names should** be `VerbNoun` in UpperCamelCase, the noun normally being the resource type.
  Standard methods follow AIP-131/132/133/134/135 and AIP-231/233/234/235; everything else is a
  custom method (AIP-136).
- **Message names should** be short and free of redundant words, and **should not** contain
  prepositions ("With", "For") — model the relationship as an optional field instead.

### §9.2 File and directory structure (AIP-191)

- APIs **must** use `proto3` syntax, and each API **must** live in a single package ending in a
  version component.
- The directory **must** match the protobuf `package` directive. File names **must** be
  `snake_case`, and **the version must not be used as a filename** (`v3.proto` is prohibited).
- APIs **should** have an obvious entry file named after the API; an API with a few discrete services
  **may** have one entry file per service. `service` definitions and their request/response messages
  **should** be in the same file.
- Within a file, higher-level definitions **should** come first, in this order (blank-line
  separated): copyright/license → `syntax` → `package` → `import`s (alphabetical) → file-level
  `option`s → `service`s → resource messages → RPC request/response messages → remaining messages →
  top-level enums.
- Within a `service`, methods **should** be grouped by resource with standard methods before custom
  ones. A parent resource **must** be defined before its children. Each request message **must**
  precede its response, and request/response pairs **should** follow the method order.

### §9.3 Language packaging options (AIP-191)

- `java_package` **must** be set; `java_multiple_files` **must** be `true`; `java_outer_classname`
  **must** be set and **should** be the filename in PascalCase + `Proto`.
- Non-Java package/namespace options **must** be set in every file of the package or in none, and if
  set **must** be identical across files.
- Compound package names **must** use PascalCase word breaks in the C#/Ruby/PHP options.
- Go: the import path **should** derive from the proto package; the API version **should** be
  prefixed with `api` (`v1` → `apiv1`); the terminal segment **should** be based on the product name
  and **must** be suffixed `pb`. The exact Go packaging value **should** be left to the team owning
  the generated code.
- All packaging annotations **should** be listed alphabetically by name.
- **Adding a namespace option to a language that previously relied on defaults is a breaking change
  in that language** — omissions must be intentional.

### §9.4 HTTP/gRPC transcoding (AIP-127)

- APIs **must** provide HTTP definitions for every RPC except bi-directional streaming ones (which
  **should not** carry a `google.api.http` annotation at all; the service **should** offer a
  non-streaming alternative).
- Each RPC **must** define its method and path via `google.api.http`. Verbs **may** be `get`,
  `post`, `patch`, or `delete`; **`put` and `custom` should not be used**. Standard methods **must**
  use their prescribed verb; custom methods **should**.
- URIs **must** use `{foo=bar/*}` syntax for request-proto variables; when extracting a resource
  name the variable **must** cover the entire name, not just the ID. `*` matches all URI-safe
  characters except `/`; `**` **may** be used only as the final segment. Nested field names **may**
  be used (and AIP-134 requires it for Update).
- `body` names the single top-level request field sent as the HTTP body; `"*"` means the request
  object itself, JSON-encoded per protobuf canonical JSON. `body` **must not** be defined for `GET`
  or `DELETE`, **must not** contain a nested field or `.`, **must not** duplicate a URI parameter,
  and **must not** be a `repeated` field. Create and Update **must** use their prescribed `body`.
- `json_name` **should not** be used to alter JSON field names except for backwards compatibility.
- `additional_bindings` **may** be defined in any number, structurally identical to the main
  annotation, but **must not** be nested inside one another, and the `body` clause **must** be
  identical across all of them.

### §9.5 Documentation in protos (AIP-192)

- **Public comments must be present on every component** — service, method, message, field, enum,
  enum value — even if terse. Services **should** explain what they are and what users can do.
- Comments **should** be grammatically correct American English, free of jargon, slang, complex
  metaphors, and pop-culture references. The first sentence **should** omit the subject and use
  third-person present tense ("Creates a book under the given publisher."). Examples involving
  people **should** use non-controversial people who are no longer alive.
- Descriptions **should** be brief but complete: what it is, how to use it, success/failure
  behavior, idempotency, units, side effects, common errors, input format and range, range
  inclusivity, length/character constraints, truncate-vs-error behavior, optionality, defaults.
- Formatting **must** be CommonMark. **Headings and tables must not be used. Raw HTML must not be
  used. ASCII-art diagrams must not be used** — link to an external page with an image. `code font`
  **should** be used for field/method names and literals.
- Cross-reference links **must** be one of: fully-qualified (`[Book][bosonical.example.v1.Book]`),
  scope-relative (`[Sci-Fi genre][Genre.GENRE_SCI_FI]`), or implied (`[Book][]`). Containing field
  names **must not** be used — reference the original definition. External links **must** be
  absolute URLs including the protocol and **should not** assume a particular docs host.
- Trademarked names: acronyms **should not** be used unless dominant in colloquial use; spelling and
  capitalization **should** follow the owner's current branding.
- **Deprecation:** the `deprecated` option **must** be `true` and the comment's first line **must**
  start with `"Deprecated: "` and give an alternative — or a reason if there is none.
- Internal content **may** be wrapped in `(--` `--)`, and non-public links plus implementation notes
  (`TODO`, `FIXME`) **must** be marked internal.
- Use only leading comments; a single component **must not** have both a leading and a trailing
  comment.

### §9.6 Common components and API-specific protos (AIP-213, AIP-215)

- All protos specific to an API **must** live in a package with a major version.
- **References to resources in other APIs must be by resource name (AIP-122), never by importing the
  other API's resource messages.** When two versions of an API need the same API-specific proto, it
  **must** be duplicated per version.
- **APIs must not create their own "API-specific common component" packages.**
- Organization-wide common component packages **must** end in `.type`, **must** be published in the
  shared repository, **must** be cleared with the API design team first, and **must** be added to
  AIP-213's list. They **must not** hold generic concepts (those belong in global common
  components) and **must not** be used by APIs outside that organization.
- Common components **must not** be moved between organization-specific and global packages; they
  **may** be copied.
- Change control on common component packages: fields/values **should not** be added to existing
  messages/enums, and **must not** be removed. Documentation **may** be clarified but **should not**
  change meaning. New messages and enums **may** be added, after consulting widely and allowing time
  for propagation before use.
- Safely importable global common components: `google.api.*` (not its subpackages),
  `google.longrunning.Operation`, `google.protobuf.*`, `google.rpc.*`, `google.type.*`.
  **[Bosonical]** `google.iam.v1.*` is also importable — it provides the IAM messages used
  throughout Bosonical. APIs **should** only rely on common-component fields released into open
  source.

---

## §10. Design patterns

### §10.1 Soft delete, undelete, expunge (AIP-164)

- Where recovery matters, `Delete` **must** mark the resource deleted rather than removing it, and
  **should** return the updated resource instead of `Empty`.
- Such resources **should** have `delete_time` and `purge_time` (AIP-148) and **should** include a
  `DELETED` state value if they have a `state` field (AIP-216).
- **`Undelete`** (`UndeleteBook`): `POST`, `:undelete`, `body: "*"`. Response **must** be the
  resource itself (fully populated where feasible), or an LRO resolving to it (`response_type` and
  `metadata_type` both specified). The request **must** have `name`, **should** be required and
  resource-typed, and **must not** carry other required fields.
- **`Expunge`** (`ExpungeBook`) **may** be provided for immediate permanent deletion of resources in
  `CREATING`, `READY`, or `SOFT_DELETED` state: `POST`, `:expunge`, `body: "*"`, response `Empty` or
  an LRO. The request refers to the resource by name and **should not** have other fields. Services
  **must** require an explicit expunge permission distinct from delete
  (`<service>.<resource>.expunge`).
- Soft-deleted resources **should not** appear in `List` unless `show_deleted` is true, and `Get`
  **should** return them rather than `NOT_FOUND`.
- Purge strategy **may** be automatic after a reasonable period (e.g. 30 days), user-set expiry
  (AIP-214), or indefinite — but **must** be documented.
- Errors: no permission → `PERMISSION_DENIED` (403), checked before existence. Permission but never
  existed / already expunged → `NOT_FOUND` (404). Delete on an already-deleted resource → success if
  `allow_missing`, else **should** be `NOT_FOUND`. Undelete on a non-deleted resource → **must** be
  `ALREADY_EXISTS` (409). Expunge on a nonexistent resource → `NOT_FOUND`; on one in the wrong state
  → `FAILED_PRECONDITION` (400).

### §10.2 Resource expiration (AIP-214)

- Expiration **must** be conveyed by a `google.protobuf.Timestamp expire_time`.
- To also accept a relative time, define a `oneof expiration` (or `{something}_expiration`)
  containing `expire_time` and a `google.protobuf.Duration ttl` marked `INPUT_ONLY`.
- On read, the API **must** always return `expire_time` and leave `ttl` blank.
- An `int64 ttl` **may** be used only where the domain demands integer TTL semantics (DNS), and
  **should** carry an `aip.dev/not-precedent` comment.

### §10.3 Resource freshness / etags (AIP-154)

- `etag` **must** be a `string` named exactly `etag`, server-provided on output, with values
  conforming to RFC 7232 (**including the quotes** — `"foo"`, not `foo`). Weak etags **must** carry
  the `W/` prefix.
- The `etag` field **on a resource should not** carry any behavior annotation; **on a request
  message** it **should** carry `REQUIRED` or `OPTIONAL`.
- A matching etag **must** be permitted (absent another failure); a mismatch **must** return
  `ABORTED` (unless a higher-precedence error like `PERMISSION_DENIED` applies).
- An absent etag **should** be permitted; services with strong consistency or parallelism
  requirements **may** require it and reject with `INVALID_ARGUMENT`.
- **Declarative-friendly resources must include `etag`** (AIP-128). It **may** also be used on custom
  methods. Strong vs. weak is the resource's choice but **should** be documented.

### §10.4 Request identification / idempotency (AIP-155)

- A `string request_id` **may** be added to request messages, including standard methods.
  **Providing a request ID must guarantee idempotency.**
- On a detected duplicate, the server **should** return the original success response. Where an
  identical response can no longer be produced (the resource changed since), the method **may**
  return current state instead.
- `request_id` **must** be on the request message and **must not** be a field on resources.
  It **should** be optional, **should** accept UUIDs (and **may** accept only UUIDs), with format
  restrictions documented. UUID request IDs **must** be annotated
  `(google.api.field_info).format = UUID4`.
- Any reasonable honoring window **may** be chosen.

### §10.5 Resource revisions (AIP-162)

- Revisions **should** be a nested `revisions` sub-collection
  (`{resource_name}/revisions/{revision_id}`) — a top-level collection is the exception, for
  revisions that outlive their parent.
- The revision message **must** be annotated as a resource (AIP-123) and **must** be named
  `{ResourceType}Revision`. It **must** contain a `snapshot` field of the parent resource's type
  holding the parent's configuration at that point in time, and a `create_time` (AIP-142). It
  **may** contain `repeated alternate_ids`.
- The revision-creation strategy is free but **must** be documented.
- Server-specified aliases (e.g. `latest`) **may** be reserved as read-only; if `latest` exists it
  **must** refer to the most recently created revision.
- User aliasing **may** be offered via an `Alias` custom method (`:alias`) whose request **must**
  have required `name` and `alias_id` fields; reusing an existing `alias_id` **must** succeed and
  repoint the alias.
- Rollback **should** be a `Rollback` custom method (`:rollback`), **must** use `POST`, **should**
  return a resource revision, and **must** take a required `name` identifying the revision to roll
  back to.
- Standard methods on revisions **must** follow AIP-131–135, with two additions: `List` **must**
  default to reverse-chronological order (overridable via `order_by`), and a `Delete` targeting an
  alias's name **must** remove the alias rather than the revision.
- Resources with revisions **may** have children, but APIs **should not** nest multiple levels of
  revisioned resources.

### §10.6 Policy preview (AIP-236)

For high-blast-radius policy resources, an `*Experiment` sub-resource lets a proposed policy be
evaluated against real traffic before promotion. Summary of the required shape:

- The experiment type **must** be named `{RegularResourceType}Experiment` under
  `.../policies/{policy}/experiments/{experiment}`, and **must** contain the standard resource
  fields, a top-level field of the live policy's type named after it, and `preview_metadata`. The
  embedded policy's `name` **must** be the live policy's name and **must not** change on update.
- `{RegularResourceType}PreviewMetadata` **must** carry `state`, `log_prefix`, `start_time`,
  `stop_time`; all fields **must** be output-only and **must** be absent until `startPreview`.
- CRUD is long-running (AIP-133/134/135) with `response_type` = the experiment; `Get` and `List`
  **must** be provided.
- `StartPreview{ResourceType}` and `StopPreview{ResourceType}` are required custom methods (`POST`,
  `:startPreview` / `:stopPreview`, `body: "*"`, LRO). Start **must** set `state = ACTIVE`,
  `start_time` = now, and the system-constant `log_prefix`; stop **must** set `state = SUSPENDED`
  and `stop_time`.
- An optional `Commit{ResourceType}` **must** atomically copy the policy into the live resource then
  delete the experiment; it **must not** succeed without a matching `etag`; a failed commit **must
  not** stop the preview or modify the live policy; a second commit **must** return `NOT_FOUND`.
- Deleting the live policy **must** cascade to all experiments. Logs **must** carry the evaluation
  result and both etags, prefixed by `log_prefix`.

Full details, including the reference proto, are in `policy-preview.md`.

---

## §11. Review checklist

This is the ordered pass `/api-style-review` runs over changed API surface. Scope it to the diff —
it is not a full-repo audit.

### §11.1 Structural checks

1. **Resource modeling** (§1, §2) — Is there a resource? Does it have `name` as the first `string`
   field with `IDENTIFIER`? Is there a `google.api.resource` annotation with correct `type`,
   `pattern`, `singular`, `plural`? One canonical parent? Acyclic references?
2. **Method shape** (§3, §4) — Correct RPC name prefix and singular/plural form? Request named
   `<Rpc>Request`? **Get/Create/Update return the bare resource, not a `<Rpc>Response` wrapper?**
   Delete returns `Empty` (or the resource for soft delete)? Correct HTTP verb, `body`, path
   variable, and `method_signature`? Is a custom method actually necessary, or does a standard
   method fit?
3. **Required-field discipline** (§3) — Does the request carry required fields the AIP doesn't
   sanction? Optional fields not defined by any AIP?
4. **List contract** (§3.2, §5.1) — `parent`, `page_size`, `page_token` present; exactly one repeated
   results field; `next_page_token` present; token opaque; page-size defaults and caps documented.
5. **Field naming and types** (§6.1, §6.2, §6.6–§6.9) — `lower_snake_case`, nouns not verbs, plural
   repeated fields, no prepositions, no unsigned ints, standard field names used for standard
   concepts, standardized codes used where one exists, timestamps as `Timestamp` with `_time`.
6. **Field behavior** (§6.3) — Is `google.api.field_behavior` on every request-path field? Is
   `IDENTIFIER` only on `name`? Are server-owned fields `OUTPUT_ONLY`?
7. **Errors** (§7) — `google.rpc.Status` with canonical codes and an `ErrorInfo`; permission checked
   before existence; correct code for each condition (`ALREADY_EXISTS` on duplicate create,
   `FAILED_PRECONDITION` on children present or a disallowed state transition, `ABORTED` on etag
   mismatch, `INVALID_ARGUMENT` on a bad `page_size` or `filter`).
8. **Documentation** (§9.5) — Comment on every component; third-person present tense; no headings,
   tables, raw HTML, or ASCII art; deprecations marked properly.
9. **File structure** (§9.2, §9.3) — proto3; versioned package matching the directory; snake_case
   filename; declaration ordering; packaging options set consistently.

### §11.2 Compatibility check

For any change to an **existing** surface, classify it against §8.3 before anything else. If the
change is breaking and the surface is beyond alpha, that is the finding — report it first,
regardless of style issues.

### §11.3 Documenting a deliberate violation (AIP-200, AIP-205)

Sometimes the right call is to break a rule. When that happens the violation **must** be marked, so
nobody copies it as precedent:

- **Permanent/accepted violation:** add an internal proto comment
  `(-- aip.dev/not-precedent: <what violates the standard and why it's necessary> --)`.
  The rationale **should** map to one of AIP-200's categories: local consistency, pre-existing
  functionality, adherence to an external spec, adherence to an existing system, expediency, or
  technical concerns.
- **Alpha-stage rough edge to be fixed before beta:** add
  `(-- aip.dev/beta-blocker: <the change that must be made for beta> --)`.
- APIs **should** only be treated as precedent-setting once they are beta or GA.
- Where an API violates a standard *throughout* (e.g. it uses `creation_time` instead of
  `create_time`), a new resource in that API **should** follow the local pattern — but the violation
  **must** still be marked non-precedent for other APIs.

### §11.4 Reporting

Report each finding with the file/line and the citation format **`AIP-NNN · <filename>`**. Don't
invent nitpicks the guide doesn't cover, and don't re-litigate code the diff didn't touch.

**A review never edits code.** It hands the developer a report and stops — no fixes, no offering
to apply one, no staged patch. This holds even for a one-line, obviously-correct fix, and even when
the reviewing agent wrote the code under review. What to do about a finding is the developer's
call; a review that rewrites what it was asked to assess destroys the developer's ability to trust
the report. If they want a finding fixed, they'll ask for it as a separate instruction.

---

# Part II — AIP index

Routing table for the 68 source documents in `docs/api-style-guides/`, grouped by
`scope.yaml` category. Open the source file when Part I isn't enough.

## Meta

### AIP-9 · Glossary — `docs/api-style-guides/glossary.md`
Defines the shared vocabulary used across the corpus so individual AIPs don't redefine terms: API
vs. API service vs. API product vs. API definition; API frontend/backend; interface, method,
request, version; client vs. user; declarative clients; Network API. Almost purely definitional —
its one normative rule is that these terms **should** be used consistently.

Consult it when a term in another AIP is ambiguous (e.g. "API service definition" vs. "API
definition"), or when writing proto comments and docs that use these words. Its declarative-client
definition matters most: such clients require services to treat client-set fields as read-only and
preserve them faithfully, which is the root of AIP-128 and AIP-129.

### AIP-200 · Precedent — `docs/api-style-guides/precedent.md`
Establishes the `aip.dev/not-precedent` internal-comment convention for marking intentional or
legacy standards violations, so later designers don't copy them or cite them as approved precedent.
Any API that violates a standard **must** carry such a comment, and it **should** explain what is
violated and why.

Enumerates the six acceptable rationale categories: local consistency, pre-existing functionality,
adherence to an external spec, adherence to an existing system, expediency (an exception granted for
time/business constraints), and technical concerns. Also states that only beta and GA APIs
**should** be considered precedent-setting. See §11.3.

## Process

### AIP-100 · API Design Review FAQ — `docs/api-style-guides/api-design-review-faq.md`
Process guidance on when formal API design review is required (any API users can code against at
beta or GA), when it isn't (internal-only, single-contracted-customer, or program-invoked APIs),
and how it works at alpha (optional but recommended; a time-sensitive alpha **may** launch without
approval if limited to a known user set, but review is mandatory to promote to beta).

Also gives practical tips: start review early, run the API linter first and explain any disabling,
comment every message/RPC/field in valid American English, and put reviewer-requested explanations
in the proto comments rather than only in the code-review thread. Covers escalation paths for
unresponsive reviewers or unresolved disagreement, and points to AIP-200 for documenting a
deliberate standards violation.

### AIP-205 · Beta-blocking changes — `docs/api-style-guides/beta-blocking-changes.md`
Defines the `aip.dev/beta-blocker` internal-comment convention: during alpha, a known usability
concern or standards violation that must be fixed before promotion to beta **must** carry a comment
linking to this AIP, and that comment **must** state what change is required for beta.

Use it when reviewers and authors agree a rough edge is tolerable now but must not ship to beta. If
instead the violation needs to persist into beta/GA, use AIP-200's `not-precedent` marker instead.

## API concepts

### AIP-111 · Planes — `docs/api-style-guides/planes.md`
Distinguishes the **management plane** (uniform, resource-oriented; configures and retrieves
resources like VMs, networks, accounts) from the **data plane** (reads and writes user data; table
rows, queue messages, blobs). Data-plane APIs **may** be heterogeneous where throughput, latency, or
an external interface spec demands it.

The load-bearing rule: data-plane resources and methods exposed through a resource-oriented
management API **must** still satisfy AIP-131 through AIP-135. Declarative clients operate on the
management plane exclusively. The distinction also drives availability, latency, and throughput
expectations, and several conditional rules elsewhere (e.g. whether `{resource}_id` is mandatory on
Create).

## Resource design

### AIP-121 · Resource-oriented design — `docs/api-style-guides/resource-oriented-design.md`
The foundational AIP. Resources (nouns) are the building blocks; a small set of standard methods
(verbs) covers most operations; the protocol is stateless; the resource graph is acyclic. Sets the
baseline every resource-oriented API must meet.

Its concrete requirements: a resource **must** support at least Get, and also List unless it's a
singleton; the resource schema **must** be identical across every method that carries it; management-
plane operations **must** be strongly consistent (after create, a get returns it; after update, a
get returns the new values; after delete, a get returns `NOT_FOUND` or a `DELETED` state); and
resource references **must** form a DAG. See §1.

### AIP-122 · Resource names — `docs/api-style-guides/resource-names.md`
The exact syntax and semantics of resource names — the canonical identifiers clients store. Covers
segment structure, the DNS-compatible character set, collection-identifier rules (plural,
`camelCase`, unique within a name), user-specified vs. system-generated ID formats, nested-collection
shortening, aliases like `users/me`, and full resource names for cross-API references.

Also the authority on declaring names in protos: `name` as the first `string` field with
`IDENTIFIER`, `parent` on List/Create requests, `google.api.resource_reference` on every reference
field, when `_id` and `_name` suffixes are appropriate, and the prohibition on embedding a resource
message where a resource name belongs. See §2.1–§2.4.

### AIP-123 · Resource types — `docs/api-style-guides/resource-types.md`
Defines the globally-unique resource *type* (`{Service Name}/{Type}`), distinct from the API-local
resource *name*, and the requirements of the `google.api.resource` annotation: `type` (PascalCase,
singular, alphanumeric), `pattern`, `singular` (lower camel case of the type), and `plural`.

Pattern variables **must** be `snake_case`, singular, `_id`-free, unique within a pattern, and
matching `[a-z][_a-z0-9]*[a-z0-9]`. Critically for compatibility: multi-pattern resources **must**
append new patterns at the end and **must not** remove or reorder existing ones, and patterns
**must** be mutually unique once ID segments are stripped. See §2.5.

### AIP-124 · Resource association — `docs/api-style-guides/resource-association.md`
Handles relationships that don't fit a clean tree. A resource **must** have at most one canonical
parent even when it relates to several types; the others become ordinary reference fields. `List`
**must not** require two distinct parents — it takes one required `parent` and **should** offer
`filter` for the rest.

For many-to-many, prefer a repeated field of resource names (AIP-144). Use a join sub-resource with
two one-to-many associations only when the relationship carries its own metadata or a repeated field
is too restrictive. See §2.6.

### AIP-126 · Enumerations — `docs/api-style-guides/enumerations.md`
When to model a fixed value set as a protobuf enum vs. a string vs. a boolean vs. an existing
standard. Enums suit sets that change infrequently (rule of thumb: at most one new value a year) and
require `UPPER_SNAKE_CASE` values with a `_UNSPECIFIED` zero value (unless a genuinely useful
`UNKNOWN` exists).

Also covers nesting (single-message enums nested and unprefixed; multi-message enums at package
level, at the bottom of the file, prefixed to avoid C++ namespace collisions), when to use a
`kebab-case` string instead, and the rule that a widely-adopted standard representation (BCP-47,
media types) beats a hand-rolled enum. See §6.4.

### AIP-128 · Declarative-friendly interfaces — `docs/api-style-guides/declarative-friendly-interfaces.md`
The requirements a resource must meet to work with infrastructure-as-code tooling: strongly-consistent
standard methods only, the `style: DECLARATIVE_FRIENDLY` annotation, and an output-only
`bool reconciling` field when changes take more than a few seconds to realize — with `GET` always
returning current state, never intended state.

Also acts as an index of the stricter cross-cutting requirements imposed elsewhere: no custom methods
(AIP-136), `Update` for repeated fields (AIP-144), standard fields (AIP-148), `etag` (AIP-154),
`validate_only` (AIP-163), soft-delete rules (AIP-164), LRO create/update, `allow_missing` on delete.
See §2.8.

### AIP-129 · Server-Modified Values and Defaults — `docs/api-style-guides/server-modified-values-and-defaults.md`
How to model fields the server generates, allocates, defaults, or normalizes, so declarative clients
can tell whether desired state matches actual state. The core rule: every field has exactly one
owner, server-owned fields are `OUTPUT_ONLY`, and the server **must not** modify client-owned fields.

Where the server decides an effective value, that **must** be two fields — the mutable user-settable
one plus an `OUTPUT_ONLY` companion named `effective_<field>`. Normalization is the narrow exception
to returning user input verbatim, and any normalized field **must** be annotated with
`google.api.field_info` (allowed formats: `uuid`, `ipv4`, `ipv6`, `email`). See §2.9.

### AIP-156 · Singleton resources — `docs/api-style-guides/singleton-resources.md`
Resources that exist exactly once per parent (config, settings, status objects). Their name is the
parent's name plus one static segment (`users/1234/config`) with no user- or system-generated ID, and
the definition **must** supply both `singular` and `plural`.

Singletons **must not** define `Create` or `Delete` (they're implicitly created and destroyed with
the parent), **should** define `Get` and `Update` (but not `Update` if every field is output-only),
and **may** define `List` per AIP-159 with the `plural` form as the trailing path segment.
Singleton children never block deletion of their parent. See §2.7.

### AIP-236 · Policy preview — `docs/api-style-guides/policy-preview.md`
A safety pattern for high-blast-radius policy rollouts: a nested `{ResourceType}Experiment`
sub-resource holds a proposed policy, `:startPreview` and `:stopPreview` drive log generation that
compares live vs. experimental evaluation against real traffic, and an optional `:commit` atomically
promotes the experiment.

Specifies the experiment resource name pattern, the required `{ResourceType}PreviewMetadata` message
(`state`, `log_prefix`, `start_time`, `stop_time`, all output-only), LRO usage on all CRUD,
etag-guarded commit (no etag → no success; double commit → `NOT_FOUND`), cascading delete of
experiments with their live policy, and the logging requirements. See §10.6.

## Operations

### AIP-130 · Methods — `docs/api-style-guides/methods.md`
The top-level taxonomy of RPC categories and a priority order for choosing among them: standard
collection/resource methods first, then batch and aggregate methods, then custom methods (resource,
collection, or stateless), then streaming — least preferred, since it's hand-written for most
clients.

Primarily a decision framework rather than a source of wire-format rules; the concrete shapes live in
AIP-131 through AIP-136. Read it first when deciding what *kind* of RPC to add, and to check whether
a proposed method will integrate with declarative clients, CLIs, and generated SDKs. See §1.5.

### AIP-131 · Standard methods: Get — `docs/api-style-guides/standard-methods-get.md`
The canonical `GetXxx` template. RPC name begins with `Get` plus the singular resource; request is
`GetXxxRequest`; **the response is the resource itself — there is no `GetXxxResponse`**; HTTP `GET`
with `name` as the only path variable, no `body`, and one `method_signature` of `"name"`.

The request carries `name` (required, resource-typed, with the pattern documented in its comment) and
**must not** carry other required fields or unsanctioned optional ones. Errors defer to AIP-193,
especially the `PERMISSION_DENIED`-before-`NOT_FOUND` ordering. See §3.1.

### AIP-132 · Standard methods: List — `docs/api-style-guides/standard-methods-list.md`
The canonical `ListXxx` template: `List` + plural resource; `ListXxxRequest`/`ListXxxResponse`; HTTP
`GET` over the collection path with `parent` as the only variable and a literal collection
identifier; no `body`; `method_signature` of `"parent"` (or empty for top-level resources).

Mandates `page_size` and `page_token` on every list request and `next_page_token` on every list
response, with exactly one repeated results field. Covers optional `filter` (AIP-160), `order_by`
syntax (comma-separated, `" desc"`, `.` for subfields), optional `total_size`, and `show_deleted` for
soft-delete APIs. See §3.2.

### AIP-133 · Standard methods: Create — `docs/api-style-guides/standard-methods-create.md`
The canonical `CreateXxx` template: HTTP `POST` on the collection, `body` mapping to the resource
field, `parent` as the only path variable, response = the resource itself (or an LRO resolving to
it), `method_signature` of `"parent,{resource},{resource}_id"`.

Its distinctive rules: `{resource}_id` **must** exist on the *request* (not the resource) for
management-plane resources and **should** for data-plane ones; the submitted resource's `name`
**must** be ignored; a duplicate ID **must** be `ALREADY_EXISTS` — unless the caller can't see the
conflicting resource, in which case `PERMISSION_DENIED`. Management-plane creates **must** be
strongly consistent. See §3.3.

### AIP-134 · Standard methods: Update — `docs/api-style-guides/standard-methods-update.md`
The canonical `UpdateXxx` template, built on the resource-plus-`update_mask` request pattern.
**Contains a Bosonical-specific override: Bosonical APIs use `PATCH` only and do not support `PUT`**,
because `PUT` turns every new field into a breaking change.

Covers `update_mask` semantics (relative to the resource, optional, an omitted mask implies all
populated fields, `*` means full replacement), the `allow_missing` upsert path and its exact
outcomes, etag-guarded optimistic concurrency (mismatch → `ABORTED`), long-running update, and two
prohibitions: no side effects beyond changing resource data, and no direct writes to state fields.
See §3.4.

### AIP-135 · Standard methods: Delete — `docs/api-style-guides/standard-methods-delete.md`
The canonical `DeleteXxx` template: HTTP `DELETE`, `name` as the only path variable, no `body`,
response `google.protobuf.Empty` (the resource itself for soft delete, an LRO for long-running
delete with both `response_type` and `metadata_type` set).

Its distinctive rules: children present **must** fail `FAILED_PRECONDITION` unless `bool force` opts
into cascade — with singleton children explicitly exempt; `etag` mismatch **must** be `ABORTED`;
`allow_missing` makes a missing-resource delete a no-op; and permission **must** be checked before
existence (`PERMISSION_DENIED` 403 then `NOT_FOUND` 404). Soft delete details live in AIP-164, bulk
delete in AIP-165. See §3.5.

### AIP-136 · Custom methods — `docs/api-style-guides/custom-methods.md`
The escape hatch for operations that aren't CRUD. Naming: verb + noun, no prepositions, no standard
verbs, never `Async` (the `LongRunning` suffix is the sanctioned way to mark a long-running variant).

HTTP: `GET` for pure retrieval, `POST` whenever there are side effects or the payload could exceed
URL limits; the URI **must** use `:customVerb` matching the RPC verb, with `body: "*"`. Three shapes
are supported — resource-based (`name` as the only path variable), collection-based (`parent`, literal
collection key), and stateless (verb and noun both after the `:`). Declarative-friendly resources
**should not** use custom methods. See §4.1.

### AIP-151 · Long-running operations — `docs/api-style-guides/long-running-operations.md`
The pattern for RPCs that can't complete synchronously (rule of thumb: 10+ seconds): return
`google.longrunning.Operation` and let the client poll. The method **must** carry a
`google.longrunning.operation_info` annotation defining **both** `response_type` and `metadata_type`,
and **must not** copy the `Operation` proto or define its own LRO interface — the shared `Operations`
service is mandatory.

Also covers validate-only response shapes, parallel-operation handling (queue, or return `ABORTED`;
declarative-friendly APIs may let a newer update preempt), operation expiration (~30 days), and error
placement — start-time errors are ordinary error responses, in-flight errors go in `Operation.error`.
Changing either type is a breaking change. See §4.2.

### AIP-152 · Jobs — `docs/api-style-guides/jobs.md`
The `Job` resource pattern for work that needs configuration separated from execution — recurring
tasks, or tasks where the permission to configure differs from the permission to run. The resource
name **must** end in "Job" and its prefix **should** read as a verb + noun.

The service **should** define all five standard methods to configure the job plus a `Run` custom
method (`POST`, URI ending `:run`, `body: "*"`, a single `name` path variable) returning an LRO that
resolves to `Run{Job}Response` carrying the result. Individual executions **may** be modeled as a
sub-collection with Get/List/Delete, in which case the operation **should** refer to the execution.
See §4.3.

### AIP-231 · Batch methods: Get — `docs/api-style-guides/batch-methods-get.md`
`BatchGetXxx` for fetching a known set of resources at a consistent point in time. HTTP `GET` with no
`body`, URI ending `:batchGet`, `repeated string names` (plus optional `parent`, which **must** match
every name), and a response whose repeated field is **in the same order as the request names**.

**The operation must be atomic** — all resources or none; if any covered location is down the whole
operation fails. Use `List` (AIP-132) when partial failure is acceptable. Batch get **should not**
paginate. A nested `repeated GetXxxRequest requests` form exists for per-resource field variance but
is discouraged. See §4.4.

### AIP-233 · Batch methods: Create — `docs/api-style-guides/batch-methods-create.md`
`BatchCreateXxx`: `POST`, URI ending `:batchCreate`, `body: "*"`, a `repeated CreateXxxRequest
requests` field with an optional hoisted `parent` that **must** match every child, and a response
with one repeated field of created resources.

Synchronous batch create **must** be atomic; asynchronous (Operation-returning) **may** choose atomic
or partial success. Partial success requires `map<int32, google.rpc.Status> failed_requests` in the
metadata, keyed by request index, mirroring what the singular Create would return, excluding
server-retryable transients — and `Operation.error` set to `Aborted` when everything fails.
Retrofitting requires `bool return_partial_success` (or a new version, if the API is synchronous).
See §4.4.

### AIP-234 · Batch methods: Update — `docs/api-style-guides/batch-methods-update.md`
`BatchUpdateXxx`, structurally identical to BatchCreate: `POST`, `:batchUpdate`, `body: "*"`,
`repeated UpdateXxxRequest requests`, a response with one repeated field of updated resources, the
same atomic-vs-partial-success rules, and the same `failed_requests` metadata pattern.

Notes `update_mask` as a good candidate for hoisting to the batch level (values **must** match if set
at both levels). Same compatibility constraint on retrofitting partial success. See §4.4.

### AIP-235 · Batch methods: Delete — `docs/api-style-guides/batch-methods-delete.md`
`BatchDeleteXxx`: **`POST` (never `DELETE`)**, URI ending `:batchDelete`, `body: "*"`,
`repeated string names`, response `google.protobuf.Empty` — or the updated resources when the
resource is soft-deleted.

Adds an explicit prohibition: **filter-based matching must not be supported**, because it risks
accidental mass deletion; AIP-165's `Purge` is the narrow escape hatch. Otherwise it shares the
atomic-vs-partial-success and `failed_requests` rules with BatchCreate/BatchUpdate, and offers the
same discouraged nested-`requests` form for per-resource variance. See §4.4.

## Fields

### AIP-140 · Field names — `docs/api-style-guides/field-names.md`
The core field-naming reference: `lower_snake_case`, no leading digits, no adjacent underscores,
plural for repeated and singular otherwise, no prepositions, adjective before noun, nouns never verbs,
booleans without an `is_` prefix, accepted abbreviations preferred (`config`, `id`, `info`, `spec`,
`stats`).

Also covers `bytes` vs. base64 `string` for binary content, `uri` vs. `url`, avoiding names that
collide with common language keywords, not repeating the enclosing message's name, and the standard
`display_name` / `title` pair. Field names are part of the public contract because they propagate
into every generated client surface. See §6.1.

### AIP-141 · Quantities — `docs/api-style-guides/quantities.md`
Naming and typing for measured quantities and counts. A quantity with a unit **must** carry that unit
as a suffix (`distance_miles`, `distance_km`), item counts **should** use `_count` (not a `num_`
prefix), and **fields must not use unsigned integer types**.

Covers compound-unit construction (underscore separation for unabbreviated components, no separation
of metric prefixes from their base, plural on the final component), the "per" convention for inverse
units (`speed_miles_per_hour`) and its exception for named derived units (hertz), and using a
specialized message like `google.type.Money` where the unit itself varies. See §6.6.

### AIP-142 · Time and duration — `docs/api-style-guides/time-and-duration.md`
Which type to use for each time concept and how to name the field: `google.protobuf.Timestamp` with a
`_time` suffix for absolute points; `google.protobuf.Duration` for spans, and with an `_offset`
suffix for a position within a stream; `google.type.Date` (`_date`), `TimeOfDay` (`_time`), and
`DateTime` (`_time`, timezone-aware) for civil time.

Notable naming rule: activity timestamps use the imperative (`publish_time`) and **should not** be
past tense (`published_time`, `created_time`, `last_updated_time`). Integer or string time fields are
permitted only for legacy/compat reasons, must carry a unit suffix where numeric, and must document
the format and the rationale. See §6.7.

### AIP-143 · Standardized codes — `docs/api-style-guides/standardized-codes.md`
When a concept already has an industry-standard code, use it — as a `string`, never as an enum, even
for a small subset of values — and state which standard the field follows.

The mandated names: `mime_type` (IANA media types), **`region_code`** for countries (Unicode CLDR;
`country_code` is prohibited), `currency_code` (ISO-4217), `language_code` (IETF BCP-47), `time_zone`
(IANA TZ), and `utc_offset` (ISO-8601). Validation **should** be case-insensitive; output **should**
use canonical case. See §6.8.

### AIP-144 · Repeated fields — `docs/api-style-guides/repeated-fields.md`
Designing list-valued fields: plural names, an enforced upper bound (rule of thumb: 100 elements)
with a sub-resource used instead when the data could grow, resource names rather than inlined
resource bodies, and message types over scalars where extra data is likely to be needed later.

Also defines the two mutation strategies: standard `Update` (whole-list replace), which
declarative-friendly resources **must** use exclusively, and `Add`/`Remove` custom methods for atomic
modification — with full naming, request/response, HTTP (`POST`, `:add*`/`:remove*`, `body: "*"`,
resource-named path variable) and error conventions (`ALREADY_EXISTS` / `NOT_FOUND`). See §6.5.

### AIP-145 · Ranges — `docs/api-style-guides/ranges.md`
Representing bounded ranges. The default is two same-typed fields prefixed `start_` and `end_`, with
**inclusive start and exclusive end** (half-open intervals).

Timestamp ranges that fit those semantics **should** use `google.type.Interval` rather than separate
fields, unless the containing message already describes an interval and the extra nesting is
unwanted. Ranges with strong colloquial inclusivity on both ends (dates, days of the week) use
`first_` / `last_` prefixes instead — for those cases only. Every range **must** document its
inclusivity. See §6.9.

### AIP-146 · Generic fields — `docs/api-style-guides/generic-fields.md`
The four mechanisms for polymorphic or free-form data — `oneof`, maps, `google.protobuf.Struct`, and
`google.protobuf.Any` — and the rule to prefer the *least generic* option that satisfies the use
case. Generic fields **should** be rare.

`oneof` is generally preferred for its type safety and semantic clarity, including for same-typed
alternatives, but is ill-suited to unbounded option sets; adding a field to an existing `oneof` is
non-breaking while moving fields in or out is breaking. Maps suit unknown or user-determined keys.
`Struct` carries arbitrary JSON (use JSONSchema if the service must reason about shape). `Any`
**should not** be used unless everything else is infeasible. See §6.10.

### AIP-147 · Sensitive fields — `docs/api-style-guides/sensitive-fields.md`
How to accept secrets, private keys, and similar values so they can be written and stored but not
read back. If the sensitive data is required for the resource to exist, it **should** be an
input-only field with no output counterpart.

If it's optional, an output-only boolean with a `_set` suffix **should** indicate presence. Where the
value must be identifiable without being fully readable, a same-typed field with an `obfuscated_`
prefix **may** replace the boolean. The obfuscation mechanism itself is left to implementers.
See §6.11.

### AIP-148 · Standard fields — `docs/api-style-guides/standard-fields.md`
The canonical field names and types to reuse across every resource rather than inventing equivalents:
`name`, `parent`, `display_name`, `title`, `given_name`/`family_name` (never `first_name`/
`last_name`), `create_time`, `update_time`, `delete_time`, `expire_time`, `purge_time`,
`annotations`, `ip_address`, and `uid`.

Specifies each field's type, output-only status, and semantics — e.g. `display_name` ≤ 63 characters
with no uniqueness requirement, `annotations` bound by Kubernetes limits with dot-namespaced keys,
`uid` a UUID4 with the `UUID4` format extension, `ip_address` requiring an explicit IP-version format.
Also flags which fields declarative-friendly and soft-delete resources should carry, and cross-refs
the standard fields owned by other AIPs (`etag`, `request_id`, `filter`, `validate_only`). See §6.2.

### AIP-149 · Unset field values — `docs/api-style-guides/unset-field-values.md`
When to use proto3's `optional` keyword on a primitive: **if and only if** distinguishing "explicitly
set to the default value" from "not set at all" is genuinely necessary. In practice that means only
integers and floats, and usually a design that avoids the distinction is better.

Clarifies that presence tracking (`optional`) and field behavior (`REQUIRED`, AIP-203) are orthogonal
and can coexist on the same field without contradiction. Adding or removing `optional` on an existing
field is backwards-incompatible. See §6.12.

### AIP-202 · Fields — `docs/api-style-guides/fields.md`
Governs the `google.api.field_info` extension, which attaches a format specifier (`UUID4`, `IPV4`,
`IPV6`, `IPV4_OR_IPV6`) to a primitive field beyond its proto type. The annotation is only required
where this or another AIP explicitly calls for it.

Each format **must** apply only to `string` fields, and services **may** normalize values (lowercase
UUIDs, strip leading zeros from IPv4 octets, RFC-5952-compress IPv6). Critically, **equivalence
comparison must not be plain text comparison** — an RFC-compliant implementation is required. Adding
or changing a format specifier on an existing field is generally not backwards compatible, and any
new format value **must** be backed by an IETF RFC or a Bosonical-approved AIP. See §6.13.

### AIP-203 · Field behavior documentation — `docs/api-style-guides/field-behavior-documentation.md`
The `google.api.field_behavior` vocabulary — `REQUIRED`, `OPTIONAL`, `OUTPUT_ONLY`, `INPUT_ONLY`,
`IMMUTABLE`, `IDENTIFIER`, `UNORDERED_LIST` — which **must** be applied to every field of every
message used in a request, with at minimum one of `REQUIRED`/`OPTIONAL`/`OUTPUT_ONLY`.

Specifies server-side handling per behavior: `OUTPUT_ONLY` inputs **must** be cleared silently and
ignored in update masks; `IMMUTABLE` changes **should** error `INVALID_ARGUMENT` while matching
values are ignored; `IDENTIFIER` belongs only on `name`; "truthy" is defined precisely for
`REQUIRED`. Ends with the definitive lists of which annotation changes are breaking and which are
safe. See §6.3.

### AIP-216 · States — `docs/api-style-guides/states.md`
How to represent lifecycle state: a nested enum called `State` (never `Status`, which is reserved for
HTTP/gRPC statuses), output-only, with `ACTIVE` for usable, `-ED` past participles for terminal
states, and `-ING` present participles for transitional ones that resolve without user action.

The load-bearing rule: **state must not be directly writable via Create or Update** — transitions go
through dedicated custom methods (verb + singular resource, `POST`, `:verb`, `body: "*"`), and a
disallowed transition **must** error `FAILED_PRECONDITION`. Also covers the `_UNSPECIFIED` zero value,
prefix conventions, the C++ top-level-enum namespace collision hazard, and the fact that adding a new
state is *not* a breaking change (so clients must be told to code defensively). See §6.14.

## Design patterns

### AIP-152 · Jobs
See **Operations** above — categorized as a design pattern in `scope.yaml`, indexed with the
method-shaped AIPs for convenience.

### AIP-153 · Import and export — `docs/api-style-guides/import-and-export.md`
The `ImportX` / `ExportX` pattern for bulk data movement, either creating many resources or
populating a single resource's data. Multi-resource operations **must** return an LRO unless
guaranteed to finish in seconds; HTTP is `POST` with `body: "*"` and a `:import` / `:export` suffix
(or `:importPages` / `:exportPages` for single-resource data).

The distinctive structural rule: source and destination configuration **must** be wrapped in a
`oneof source` / `oneof destination` even when there's only one option today, preserving room to add
more; data-level configuration common to all sources goes at the top level. Inline variants
(`InlineSource` / `InlineDestination`) **must** use the same format in both directions. Partial
failures **should** be reported in the operation metadata as `google.rpc.Status`. See §4.5.

### AIP-154 · Resource freshness validation — `docs/api-style-guides/resource-freshness-validation.md`
The `etag` mechanism for optimistic concurrency. The field **must** be a `string` named exactly
`etag`, server-provided, RFC 7232-conformant **including the surrounding quotes**, with weak etags
carrying the `W/` prefix.

A matching etag **must** be permitted; a mismatch **must** return `ABORTED`. An absent etag
**should** be permitted, though services with strong consistency requirements **may** demand it and
reject with `INVALID_ARGUMENT`. Note the annotation asymmetry: on a resource the field **should not**
carry any behavior annotation, but on a request message it **should** be `REQUIRED` or `OPTIONAL`.
Mandatory for declarative-friendly resources. See §10.3.

### AIP-155 · Request identification — `docs/api-style-guides/request-identification.md`
The optional `string request_id` field that makes a mutating RPC safely retryable. **Providing a
request ID must guarantee idempotency**; on a detected duplicate the server **should** return the
original success response.

`request_id` **must** be on the request message and **must not** appear on resources. It **should**
be optional and **should** accept UUIDs (and **may** accept only UUIDs, with format restrictions
documented); UUID request IDs **must** carry `(google.api.field_info).format = UUID4`. Where the
original response can no longer be reproduced exactly, returning current state is permitted.
See §10.4.

### AIP-157 · Partial responses — `docs/api-style-guides/partial-responses.md`
Letting callers control which fields of a large or expensive resource come back. **Contains a
Bosonical-specific rule: field masks are specified as a system parameter** (query parameter, header,
or gRPC metadata) — the `read_mask` request field is DEPRECATED and retained only for legacy and
external usage.

The parameter **must** be a `google.protobuf.FieldMask`, **must** be optional, defaults to `"*"`, and
changing that default is breaking. The alternative is a `view` enum ending in `-View` with at minimum
`BASIC` and `FULL` values, where `UNSPECIFIED` **must** be valid and documented (defaulting to
`BASIC` for List). Fields **may** be added to a view over time; **removing one is breaking**. See §5.4.

### AIP-158 · Pagination — `docs/api-style-guides/pagination.md`
The pagination contract: `int32 page_size` and `string page_token` on the request (neither required),
`string next_page_token` on the response, and the results as the first repeated field with field
number 1. **Pagination must be designed in from the outset** — adding it later is backwards-
incompatible.

Specifies default and maximum page-size behavior (unset picks a documented default without erroring,
over-max coerces down, negative errors `INVALID_ARGUMENT`), the rule that an **empty
`next_page_token` is the only end-of-collection signal**, and strict token opacity — base-64 encoding
a transparent token is explicitly called out as insufficient, and tokens **must not** convey any
authorization. Also covers the optional `skip` and `total_size` fields. See §5.1.

### AIP-159 · Reading across collections — `docs/api-style-guides/reading-across-collections.md`
Using `-` as a wildcard collection segment: on `List`, to read across multiple parents; on `Get`, to
look up a resource by globally-unique ID without knowing its parent. In both cases the URI pattern
**must** still be declared with `*` and **must not** hard-code `-`, the support **must** be
documented, and returned resources **must** use canonical names with real parent identifiers.

The safety rule: cross-collection `Get` **must not** be supported where child IDs could collide
across parents. Cross-parent requests **should not** support `order_by` (and must document
best-effort ordering if they do), and any resulting partial failures **must** follow AIP-217.
See §5.3.

### AIP-160 · Filtering — `docs/api-style-guides/filtering.md`
The string filter grammar for List/Search methods: one `string filter` field, bare literal matching,
`AND`/`OR`, `NOT` and `-`, comparison operators for strings/numerics/timestamps/durations (but not
booleans or enums), the `.` traversal operator, the `:` has operator, and API-defined `call(arg...)`
functions.

Two traps worth memorizing: **`OR` binds tighter than `AND`**, the opposite of most languages; and
`.` **must not** traverse a repeated field except in conjunction with `:`, with indexed access
(`e.0.foo`) always invalid. Also specifies literal formats (RFC-3339 timestamps, `20s` durations,
case-sensitive enum names) and that an invalid filter **should** error `INVALID_ARGUMENT`. See §5.2.

### AIP-161 · Field masks — `docs/api-style-guides/field-masks.md`
The syntax and semantics of `google.protobuf.FieldMask` on Update requests: always relative to the
resource, `.` for traversal into nested fields and (string- or integer-keyed) maps, backtick quoting
for awkward map keys, and `*` across repeated fields or maps.

The defining constraint is **read/write self-consistency**: update-then-read with the same mask
returns the same data, read-then-update with the same mask is a no-op, and any mask valid for one
direction is valid for the other. Indexed access **must** return `INVALID_ARGUMENT`, and output-only
fields named in an update mask — directly or via a wildcard — **must** have their input ignored so a
single mask works both ways. See §5.5.

### AIP-162 · Resource Revisions — `docs/api-style-guides/resource-revisions.md`
Modeling revision history as a nested `revisions` sub-collection
(`{resource_name}/revisions/{revision_id}`), with a `{ResourceType}Revision` message carrying a
`snapshot` of the parent's configuration at that point plus a `create_time`, and optional
`alternate_ids`.

Covers server-managed aliases (`latest` **must** point at the newest revision), a user-facing `Alias`
custom method (re-aliasing an existing ID **must** succeed by repointing), and a `Rollback` custom
method (`POST`, `:rollback`). Two additions to the standard methods: revision `List` **must** default
to reverse-chronological order, and `Delete` targeting an alias name **must** remove the alias rather
than the revision. Draft status. See §10.5.

### AIP-163 · Change validation — `docs/api-style-guides/change-validation.md`
The `bool validate_only` dry-run field: the method returns exactly the status code, headers, and body
a real call would, without executing. The API **must** still perform full permission checks and all
other validation, and a validated request **must** fail whenever the real one would.

Fields infeasible to produce during validation (auto-generated IDs) **should** be omitted from the
response. Optional in general — but **mandatory on every mutating method of a declarative-friendly
resource** (AIP-128). See §8.4.

### AIP-164 · Soft delete — `docs/api-style-guides/soft-delete.md`
The soft-delete / undelete / expunge triad. `Delete` marks the resource deleted rather than removing
it and **should** return the updated resource; `Undelete` (`POST`, `:undelete`, `body: "*"`) restores
it and **must** return the resource itself; an optional `Expunge` (`:expunge`) permanently removes it
and **must** require a permission distinct from delete.

Soft-delete resources **should** carry `delete_time` and `purge_time` (AIP-148) and a `DELETED` state
value (AIP-216); they **should not** appear in `List` without `show_deleted`, and `Get` **should**
return them rather than 404. Includes the full error matrix: `PERMISSION_DENIED` before existence,
`NOT_FOUND` for never-existed/expunged, `ALREADY_EXISTS` (409) for undeleting a live resource, and
`FAILED_PRECONDITION` for expunging one in the wrong state. See §10.1.

### AIP-165 · Criteria-based delete — `docs/api-style-guides/criteria-based-delete.md`
The `Purge` method for filter-based bulk deletion — explicitly discouraged. Most APIs **should** use
only `Delete` (AIP-135) or BatchDelete (AIP-235); `Purge` is for genuine thousands-of-resources
requirements where BatchDelete is insufficient.

Requires an LRO response, `POST` with `body: "*"`, a required `string filter` with List semantics
(AIP-160), and critically a `bool force` field that, when unset, **must** perform a dry run —
returning `purge_count` and a `purge_sample` (rule of thumb: 100 names, documented as a maximum and
as random or deterministic) while deleting nothing. See §4.6.

### AIP-210 · Unicode — `docs/api-style-guides/unicode.md`
Pins down what "character" means so length limits, documentation, and billing are unambiguous: in all
API documentation and every string length limit, a character **must** be a Unicode code point, and
limits **must** be measured and enforced in code points.

For identifiers: ASCII **should** be the limit (`[a-zA-Z][a-zA-Z0-9_-]*`), leading digits **should**
be avoided (reserved for Bosonical-generated IDs), and 64 characters **should** be the cap. If full
Unicode is allowed, input not in **Normalization Form C must be rejected**, identifiers **must** be
stored and compared in NFC, and two byte sequences with the same NFC form **must** be treated as
identical — the fix for a whole class of duplicate-identifier bugs. See §6.15.

### AIP-211 · Authorization checks — `docs/api-style-guides/authorization-checks.md`
When to authorize and how to phrase the denial. **Services must check authorization before validating
any request.** Any authorization failure **must** be `PERMISSION_DENIED`, phrased to avoid leaking
existence: `Permission '{p}' denied on resource '{r}' (or it might not exist).`

Where authorization can't be determined because the resource doesn't exist, the service **should**
check read-children permission on the parent and return `NOT_FOUND` if that passes. Where two
operations carry different permissions that could each reveal existence, the service **should** check
only the one for the operation being called and **should not** "help out" by checking related
permissions — those algorithms leak. See §7.3.

### AIP-214 · Resource expiration — `docs/api-style-guides/resource-expiration.md`
Expiration **must** be conveyed by a `google.protobuf.Timestamp expire_time`. To also accept relative
input, define a `oneof expiration` (or `{something}_expiration`) holding `expire_time` plus a
`google.protobuf.Duration ttl` marked `INPUT_ONLY`.

On read, the API **must** always return `expire_time` and leave `ttl` blank. An `int64 ttl` is
permitted only where the domain demands integer TTL semantics (DNS), and **should** then carry an
`aip.dev/not-precedent` comment. See §10.2.

### AIP-217 · Unreachable resources — `docs/api-style-guides/unreachable-resources.md`
How a List method reports partial failure instead of failing outright: a `repeated string unreachable`
field annotated `UNORDERED_LIST`, holding **service-relative** resource names (never full names, URIs,
or bare IDs) of what couldn't be reached or what blocked reaching it.

The response **must not** carry any other detail about the failure — the service **must** instead
provide a way to get a real error via a narrower request. Covers scoping (report the tightest
unreachable unit), the requirement to surface previously-unreachable results on later pages once
availability returns, documented maximum entry counts independent of `page_size`, and the
`bool return_partial_success` opt-in required when retrofitting onto an API that currently hard-fails.
See §5.6.

## Compatibility and versioning

### AIP-180 · Backwards compatibility — `docs/api-style-guides/backwards-compatibility.md`
The authority on what breaks clients, across three dimensions that **must** all hold: source (old
code still compiles and runs against a new client library), wire (serialization still matches), and
semantic (behavior still meets reasonable expectations).

Enumerates what's safe (adding interfaces, methods, messages, fields, enums, and enum values) and
what isn't: removing or renaming anything, moving components between files, moving a field in or out
of a `oneof`, changing a field's type, adding a required field, changing a resource's name (which
holds even *across* major versions), raising a string field's size bound, changing a value's format
or construction algorithm even on `OUTPUT_ONLY` fields, and changing a default value or its
serialization. See §8.3.

### AIP-181 · Stability levels — `docs/api-style-guides/stability-levels.md`
The three stability levels and what each promises. **Alpha:** curated, individually contactable
users; breaking changes expected; no stability guarantee. **Beta:** publicly available (not
allowlisted), complete and ready to be stable, changes permitted only after a deprecation period
defined up front, time-boxed (rule of thumb: 90 days). **Stable:** fully supported for the lifetime
of the major version, with no breaking changes.

When a stable component must break, the producer **should** cut the next major version and start a
deprecation clock; a rare isolated in-place break **may** deprecate the component but **must** still
support it for the full turndown period. **[Bosonical]** an in-place stable break requires API
Governance approval. Security and regulatory emergencies override everything, with no deprecation
promised. See §8.2.

### AIP-182 · External software dependencies — `docs/api-style-guides/external-software-dependencies.md`
For services exposing selectable third-party software versions (database engines, OS images, language
runtimes) with their own release lifecycles. Services **should** allow creation on any
currently-supported LTS version, **may** allow non-LTS, and **should not** indefinitely allow
creation on end-of-life versions.

Removing EOL creation support is explicitly **not** treated as a breaking change under AIP-181, even
though it functionally is. Services **must** notify users whose resources approach EOL, **should**
let existing resources remain, and **should not** proactively remove or restrict them absent critical
security concerns — and if they officially support an EOL version for business reasons, they **must**
take on patching it. See §8.5.

### AIP-185 · API Versioning — `docs/api-style-guides/api-versioning.md`
Bosonical's versioning strategy: a major version at the end of the protobuf package and as the first
REST path segment; **minor and patch versions must not be exposed** (`v1`, never `v1.0` or `v1.4.2`);
no dependency on a previous major version of the same API; and a well-communicated deprecation period
before any shutdown.

Details three strategies for alpha/beta. **Channel-based (preferred):** one channel per stability
level, `v1alpha` ⊃ `v1beta` ⊃ `v1`, with deprecated functionality forbidden from graduating upward
and beta removals after ~180 days. **Release-based (legacy):** `v1beta1`, `v1alpha5`, incrementing on
breaking beta changes. **Visibility-based:** UPPER-case labels, implicit `PUBLIC`, comma-separated as
logical OR, at most one per request. See §8.1.

## Polish

### AIP-190 · Naming conventions — `docs/api-style-guides/naming-conventions.md`
The vocabulary and casing rules for services, methods, and messages. Definitions **must** use
UpperCamelCase. Names **should** be correct American English, intuitive, and consistent — one name
per concept, no overloading, nothing overly general within the API or the wider Bosonical ecosystem.

Distinguishes the *interface name* (the proto `service`, an intuitive noun like `Calendar` or
`BlobStore`, disambiguated with an `Api`/`Service` suffix when needed, avoiding collisions with
language concepts like `File`) from the *service name* (the deployed host). Method names **should**
be `VerbNoun`; message names **should** be short and preposition-free. See §9.1.

### AIP-191 · File and directory structure — `docs/api-style-guides/file-and-directory-structure.md`
How proto definitions are organized on disk: `proto3` syntax, one API per package ending in a version
component, a directory path matching the package, `snake_case` filenames, and an obvious entry file
named after the API — with the version explicitly prohibited as a filename (`v3.proto`).

Prescribes the exact intra-file ordering (license → syntax → package → alphabetized imports →
file-level options → services → resource messages → request/response messages → remaining messages →
top-level enums), method grouping within a service, and parent-before-child message ordering. Also
covers the language packaging options (`java_package`, `java_multiple_files`, `java_outer_classname`
mandatory; non-Java options all-or-nothing and identical across files) and the Go import-path
conventions (`apiv1`, `…pb` suffix). See §9.2, §9.3.

### AIP-192 · Documentation — `docs/api-style-guides/documentation.md`
How proto comments — the source of generated reference docs — must be written. Comments **must**
exist on every service, method, message, field, enum, and enum value. The first sentence **should**
omit the subject and use third-person present tense ("Creates a book under the given publisher.").

Formatting **must** be CommonMark, with headings, tables, raw HTML, and ASCII art all prohibited.
Cross-reference links **must** use one of three exact forms and **must not** go through containing
field names. Deprecation requires both `option deprecated = true` and a comment starting
`"Deprecated: "` with an alternative or a reason. Internal content is wrapped in `(--` `--)`, and
only leading comments are used. See §9.5.

### AIP-193 · Errors — `docs/api-style-guides/errors.md`
The error contract. Services **must** return `google.rpc.Status` with canonical `google.rpc.Code`
values, and **every** error response **must** include an `ErrorInfo` carrying a machine-readable
`(reason, domain)` pair plus `metadata`. `reason` matches `[A-Z][A-Z0-9_]+[A-Z0-9]` (≤63 chars);
the same pair **must** mean the same error and **must not** be reused for different ones.

Any dynamic detail appearing in an error message **must** also be in `metadata`, and every metadata
key ever emitted **must** keep being emitted. Also covers `LocalizedMessage` and `Help` payload
rules, the message-stability rules for brownfield APIs, the discouragement of partial errors (use an
LRO instead), and the mandatory `PERMISSION_DENIED`-before-`NOT_FOUND` check ordering. Note: it does
not reproduce a full status-code table — it defers to the gRPC docs and AIP-194. See §7.

### AIP-194 · Automatic retry configuration — `docs/api-style-guides/automatic-retry-configuration.md`
Which gRPC status codes a client may safely retry automatically. Only requests that are unary,
non-transactional, and safe to repeat are eligible; transactional requests **should** be retried at
the application level by restarting the whole transaction.

`UNAVAILABLE` is the retryable code. Never auto-retry `OK`, `CANCELLED`, `DEADLINE_EXCEEDED`,
`INVALID_ARGUMENT`, `DATA_LOSS`, `NOT_FOUND`, `ALREADY_EXISTS`, `PERMISSION_DENIED`, `UNAUTHORIZED`,
`UNAUTHENTICATED`, `FAILED_PRECONDITION`, `OUT_OF_RANGE`, or `UNIMPLEMENTED`. Generally don't retry
`RESOURCE_EXHAUSTED` (quota recovery can take hours and has billing implications), `INTERNAL`,
`UNKNOWN`, or `ABORTED` (retry the enclosing transaction instead). Streaming RPCs are out of scope.
See §7.5.

## Protocol buffers

### AIP-127 · HTTP and gRPC Transcoding — `docs/api-style-guides/http-and-grpc-transcoding.md`
How RPCs map onto HTTP/JSON via `google.api.http`. Every RPC **must** have an HTTP definition except
bi-directional streaming ones, which **should** omit the annotation entirely and **should** be
accompanied by a non-streaming alternative.

Verbs are limited to `get`, `post`, `patch`, and `delete` — **`put` and `custom` should not be
used**. URI variables use `{foo=bar/*}` syntax and **must** capture the entire resource name, with
`*` matching everything but `/` and `**` allowed only as the final segment. The `body` clause has
strict constraints: absent for `GET`/`DELETE`, never nested, never a URI parameter, never repeated,
and identical across all `additional_bindings`. See §9.4.

### AIP-213 · Common components — `docs/api-style-guides/common-components.md`
Governs shared proto packages: organization-wide ones (**must** end in `.type`, **must** be cleared
with the API design team, **must** be published in the shared repository, **must not** hold generic
concepts) and global ones for cross-organization concepts.

Because these packages are effectively unversioned and shared, change control is strict: fields and
enum values **should not** be added to existing messages/enums and **must not** be removed;
documentation **may** be clarified but **should not** change meaning; new messages and enums **may**
be added after wide consultation and with time allowed for propagation. Lists the safely-importable
global components (`google.api.*`, `google.longrunning.Operation`, `google.protobuf.*`,
`google.rpc.*`, `google.type.*`) plus **[Bosonical]** `google.iam.v1.*`. See §9.6.

### AIP-215 · API-specific protos — `docs/api-style-guides/api-specific-protos.md`
All protos specific to an API **must** live in a package with a major version, and **must not** be
shared across APIs. References to another API's resources **must** be expressed as resource names
(AIP-122), never by importing that API's resource messages.

Two consequences: when two versions of an API need effectively the same proto, it **must** be
duplicated per version; and APIs **must not** invent their own "API-specific common component"
packages — the only sanctioned sharing mechanisms are AIP-213's organization-specific `.type`
packages (usable only within that organization) and global common components. See §9.6.

---

# Appendix A — AIP number → filename map

The source files cross-link each other by their **original** filenames (`./0133.md`,
`./0203.md#output-only`). Those links are **stale by design** — this fork renamed every file to a
readable slug. Use this table to resolve any such link, and cite findings using the slug.

| AIP | File | AIP | File |
|---|---|---|---|
| 009 | `glossary.md` | 158 | `pagination.md` |
| 100 | `api-design-review-faq.md` | 159 | `reading-across-collections.md` |
| 111 | `planes.md` | 160 | `filtering.md` |
| 121 | `resource-oriented-design.md` | 161 | `field-masks.md` |
| 122 | `resource-names.md` | 162 | `resource-revisions.md` |
| 123 | `resource-types.md` | 163 | `change-validation.md` |
| 124 | `resource-association.md` | 164 | `soft-delete.md` |
| 126 | `enumerations.md` | 165 | `criteria-based-delete.md` |
| 127 | `http-and-grpc-transcoding.md` | 180 | `backwards-compatibility.md` |
| 128 | `declarative-friendly-interfaces.md` | 181 | `stability-levels.md` |
| 129 | `server-modified-values-and-defaults.md` | 182 | `external-software-dependencies.md` |
| 130 | `methods.md` | 185 | `api-versioning.md` |
| 131 | `standard-methods-get.md` | 190 | `naming-conventions.md` |
| 132 | `standard-methods-list.md` | 191 | `file-and-directory-structure.md` |
| 133 | `standard-methods-create.md` | 192 | `documentation.md` |
| 134 | `standard-methods-update.md` | 193 | `errors.md` |
| 135 | `standard-methods-delete.md` | 194 | `automatic-retry-configuration.md` |
| 136 | `custom-methods.md` | 200 | `precedent.md` |
| 140 | `field-names.md` | 202 | `fields.md` |
| 141 | `quantities.md` | 203 | `field-behavior-documentation.md` |
| 142 | `time-and-duration.md` | 205 | `beta-blocking-changes.md` |
| 143 | `standardized-codes.md` | 210 | `unicode.md` |
| 144 | `repeated-fields.md` | 211 | `authorization-checks.md` |
| 145 | `ranges.md` | 213 | `common-components.md` |
| 146 | `generic-fields.md` | 214 | `resource-expiration.md` |
| 147 | `sensitive-fields.md` | 215 | `api-specific-protos.md` |
| 148 | `standard-fields.md` | 216 | `states.md` |
| 149 | `unset-field-values.md` | 217 | `unreachable-resources.md` |
| 151 | `long-running-operations.md` | 231 | `batch-methods-get.md` |
| 152 | `jobs.md` | 233 | `batch-methods-create.md` |
| 153 | `import-and-export.md` | 234 | `batch-methods-update.md` |
| 154 | `resource-freshness-validation.md` | 235 | `batch-methods-delete.md` |
| 155 | `request-identification.md` | 236 | `policy-preview.md` |
| 156 | `singleton-resources.md` | | |
| 157 | `partial-responses.md` | | |

**Referenced but not present in this fork:** AIP-1, AIP-2, and AIP-8 — the upstream governance and
process AIPs (how AIPs are created, numbered, and escalated). Links to them in the source text can't
be followed here. Treat escalation and exception-approval questions as matters for the developer, not
the corpus.

`scope.yaml` in the same directory defines the category taxonomy used to group Part II.
