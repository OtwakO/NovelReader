# NovelReader — Engineering Plan

`PLAN.md` describes the project as it exists now: its objective, architecture, current state, next work, and durable decisions. Historical discoveries belong in `DEVELOPMENT.md`; detailed subsystem contracts and audit evidence belong in `docs/` and `testdata/`.

The former detailed plan is preserved as `docs/archive/PLAN_HISTORY_2026-08-27.md` for historical reference only.

## Objective

Build a self-hosted, web-first novel reader with strong behavioral compatibility with Legado BookSource JSON.

NovelReader must:

- preserve imported BookSource definitions losslessly, including unknown fields;
- execute source-defined requests, rules, JavaScript, sessions, cookies, and WebView work on the backend;
- distinguish engine defects from DNS, upstream, authentication, WAF, captcha, and stale-source failures;
- provide a lightweight responsive Reader and management UI through stable typed domain APIs;
- provide an installable resilient web app shell without service-worker ownership of authenticated API or Reader Data;
- keep each reader account's books, sources, reading state, and files isolated and portable;
- remain understandable and maintainable without source-specific patches or speculative abstraction.

“Compatible” means matching documented and observed Legado behavior where a portable server implementation is practical. It does not mean bypassing site policy, captchas, authentication, or anti-automation controls.

## Architecture

### System shape

```text
Vue 3 SPA
  → typed HTTP/SSE API modules
  → authenticated Go API adapters
  → book/search/explore/candidate domain workflows
  → shared source executor and SourceSession
  → HTTP, fingerprint, typed-data, or WebView transport
  → Legado-compatible analyzer and JavaScript bridge
  → typed domain results
  → per-reader SQLite database and files
```

The frontend owns interaction and presentation. It never constructs BookSource URLs, evaluates source rules, or interprets source-specific crawling behavior. BookSource cover bytes are also backend-owned: stored books use a book-derived endpoint, while temporary Search, Explore, and candidate results use authenticated opaque signed references so HTTP-only images and source request state work under HTTPS without exposing a general image proxy.

The production frontend is an installable PWA. Its service worker pre-caches only the app shell, manifest, icons, and fingerprinted frontend assets; `/api` requests remain network-owned and authenticated books, chapters, progress, and credentials are never placed in service-worker caches. Waiting updates activate at the next launch or reload boundary rather than interrupting an active reading session.

### Repository ownership

```text
backend/internal/
  api/          HTTP/SSE adapters, authentication boundaries, static delivery
  auth/         accounts, sessions, setup, recovery, administration
  readerstore/  per-reader homes, schema validation, lifecycle, deletion
  booksource/   canonical source model, import/export, persistence
  sourceexec/   request construction, sessions, routing transports
  analyzer/     Default/CSS/XPath/JSONPath/Regex rules and JavaScript bridge
  book/         search, detail, TOC, content, shelf, progress, bookmarks
  candidate/    asynchronous readable-candidate validation and commit
  chineseconv/  display-only Chinese conversion capability and official OpenCC adapter
  explore/      source-native catalog sessions and pagination where separated
  webview/      Go-side Patchright worker client and diagnostic

webview-worker/  Python Patchright browser worker managed with uv

frontend/src/
  api/          typed backend transport and DTOs
  app/          bootstrap, Router, route policy, application shells
  stores/       narrowly shared Pinia state and durable UI preferences
  ui/           tokens and reusable interaction primitives
  features/     feature-owned pages, workflows, state, and components

testdata/booksource/
  conformance/  deterministic captured compatibility fixtures
  audits/       immutable sampled live-audit evidence
```

Feature code should stay near the feature it supports. New shared layers require more than one real caller and a clearer interface than direct use.

### Identity and storage

- `system.db` owns accounts, authentication sessions, setup/recovery state, and reader-administration jobs.
- Each immutable user ID owns one self-contained reader home containing `reader.db` and ordinary files.
- Every Reader Data request authenticates first, derives identity from request context, and acquires that reader's bounded runtime.
- `readerstore` exclusively owns reader-home paths, schema validation, database lifecycle, export/import boundaries, and deletion coordination.
- One writable `DATA_DIR` has one NovelReader server owner. Shared writable multi-process access is unsupported.
- A stopped-server copy of the complete `data/` directory is the disaster-recovery backup boundary.
- Portable reader exports exclude authentication authority and source credentials.

NovelReader remains pre-public. Development data is disposable:

- change the canonical fresh schema directly;
- recreate incompatible local databases or data roots;
- do not add migrations, backfills, compatibility adapters, or migration-only tests unless public or irreplaceable data exists or the user explicitly approves them.

See `docs/AUTHENTICATION_DESIGN.md`, `docs/USER_STORAGE_IMPLEMENTATION_TASKS.md`, and `docs/adr/0001-user-owned-data-and-local-authentication.md`.

### BookSource execution

#### Canonical model

- Each installed definition has an immutable NovelReader Source ID; `bookSourceUrl` remains imported Legado address data, may duplicate, and is never the runtime locator. Source names are display labels and may also duplicate.
- All imported fields, including unknown fields, survive import, editing, persistence, and export.
- Typed fields exist for behavior NovelReader executes; original definitions remain available for round-trip fidelity and diagnostics.
- Malformed executable fields produce explicit import or runtime diagnostics rather than silently becoming empty behavior.

#### Shared request boundary

Search, Book Info, TOC, chapter content, pagination, images, Explore, JavaScript bridge requests, and WebView requests use the same execution contract.

```text
RequestSpec
  URL, method, body, headers, charset, retry
  webView, webJs, bodyJs, type, origin, dnsIp
  source identity and SourceSession context

Response
  requested/final URL, status, headers, body/bytes
  transport kind, redirects, timing, classified failure
```

Routing selects normal HTTP, fingerprint, bounded typed `data:` execution, or WebView without exposing transport details to book-domain workflows.

#### SourceSession

A SourceSession owns the scoped state needed across one source workflow:

- cookies and cookie helpers;
- source variables, cache, and headers;
- rate and request state;
- JavaScript library/context;
- mutable book/chapter execution context;
- session-scoped fingerprint or WebView continuity.

State may persist across detail → TOC → content for one user and source. It must never leak across users or unrelated sources.

#### Analyzer

The analyzer preserves typed HTML, JSON, list, string, and JavaScript values where possible and implements Legado-compatible Default/JSoup, CSS, XPath, JSONPath, Regex, connectors, replacements, indices, variables, and script chaining.

Compatibility work must fix shared executor, analyzer, session, or workflow seams. Source-specific production branches are a last resort and require evidence that the source contract itself is exceptional.

### Book and reading model

- Normalized title plus author is the logical book identity.
- `(sourceUrl, bookUrl)` pairs are source bindings beneath that logical book.
- Adding or rediscovering the same logical book merges alternate bindings while preserving its ID, selected source, chapters, progress, bookmarks, and cache.
- Source switching validates an already persisted exact alternate binding before atomically replacing the active binding. Failed validation leaves active state unchanged.
- Reading progress is one canonical chapter/index position migrated across source switches by normalized chapter-title match, then a documented approximate index fallback.
- Bookmark source migration requires an exact normalized-title match; otherwise the bookmark remains explicitly orphaned.
- Processed chapter content may be cached server-side under bounded retention for upstream-outage fallback.
- Reader Chinese conversion is display-only and never changes canonical chapters, progress, bookmarks, shelf metadata, or backend data. Simplified conversion and Taiwan Traditional phrase conversion are provided through a narrow backend capability; release images use the pinned official BYVoid/OpenCC runtime with the Jieba-backed `tw2sp_jieba` and `s2twp_jieba` presets, while ordinary local builds may explicitly report that native conversion is unavailable.

### Search and candidate journey

Search uses deterministic batches and SSE:

- the backend selects eligible text sources in stable order;
- cursors identify the next source offset and source-list revision;
- the frontend merges cumulative results and preserves alternate bindings;
- source work remains bounded by per-search and process-wide capacity.

Opening a result creates a non-destructive Candidate Book Detail. It may validate sources but does not add the book to the shelf.

Direct Add and Candidate Book Detail share the account-scoped asynchronous candidate operation defined in `docs/CANDIDATE_BOOK_JOURNEY.md`:

- every discovered binding is queued in stable primary-first order;
- up to five source pipelines run concurrently and freed slots refill immediately;
- a candidate must produce Book Info, a usable TOC, a readable chapter URL, and credible chapter content;
- progress and per-source stages are exposed through reconnectable SSE snapshots;
- the first verified source wins; untouched and winner-cancelled attempts are marked skipped, not failed;
- server-held verified Book Info and TOC commit idempotently without recrawling;
- direct card Add commits automatically, while Candidate Book Detail requires explicit Add;
- cancellation is acknowledged immediately while active source work drains privately before runtime release;
- operations expire, evict, cancel, commit, and shut down without leaking reader-runtime leases.

The active private API uses `/api/candidate-resolutions`. Superseded synchronous preview, readable, enrich, and shelf-ingestion endpoints are removed.

### Explore

Explore is strictly one selected BookSource at a time.

- Eligibility follows the source's Explore configuration independently of text-search enablement.
- The backend evaluates source-native categories, controls, requests, rules, and pagination.
- Opaque bounded sessions retain source state and enforce sequential server-authoritative pages.
- The frontend renders typed catalogs, controls, results, pagination, and diagnostics without seeing raw rules or executable expressions.
- NovelReader does not create a cross-source recommendation feed, rankings, or editorial aggregation.

### Frontend

The production frontend is a Vue 3 SPA using Vue Router, Pinia, Vue I18n, Vite, and the Options API convention.

- Hash routing preserves static-file and direct-link deployment compatibility.
- Pinia is reserved for genuinely cross-route state, not every API response.
- Feature-owned English, Simplified Chinese, and Traditional Chinese modules must have identical key coverage.
- Browser language selects the initial locale with English fallback; preference is device-local and does not alter URLs.
- Shelf restoration, filters, disclosure state, and similar convenience state may remain tab/device-local when they are not Reader Data.
- Responsive mobile behavior, accessibility, visible failure states, and no horizontal overflow are required UI gates.
- Reader interactions, typography, source recovery, TOC, bookmarks, wake lock, and keyboard controls stay feature-local and guarded against accidental input conflicts.
- Route-level code splitting and immutable caching of fingerprinted assets are the frontend delivery policy; correctness must not depend on eager loading unrelated routes.

### Deployment

- `docker-compose.yml` is the standalone operator-facing production stack.
- It runs `ghcr.io/otwako/novelreader:latest` and the private-network WebView worker, persists `./data:/data`, and requires no `.env` file.
- The app entrypoint prepares bind-mount ownership and drops to configured numeric `PUID:PGID`.
- `compose.e2e.yaml` is the deterministic local-build and deployment-test contract.
- GitHub Actions runs backend, frontend, and changed WebView verification in parallel, builds each required container image once with Buildx, runs Compose E2E against those exact images, and publishes only after they pass.
- Every successful container run records a full-commit `sha-*` app/worker pair for exact rollback. Main moves `latest` and `edge` only for changed images. Unchanged WebView code reuses the published `edge` image for main E2E; `v*` releases publish normalized semver image tags and reuse the preceding immutable release worker digest after verifying it with the new app.
- `run-local.bat` remains the development workflow.

WebView is observable but not continuously monitored. Settings performs an authenticated backend-owned synthetic execution through the same Go → worker → Chromium path used by sources and reports `not configured`, `unavailable`, or `verified`.

## Current State

### Source Collections

The `feat/source-collections` branch adds reader-owned Source Collections without changing the flat BookSource execution model:

- a complete uploaded JSON document can be imported and atomically replaced as one renameable collection;
- a URL collection can be manually synchronized, with failed downloads or malformed documents leaving the last good sources unchanged;
- collection replacement is authoritative: existing definitions and user edits are overwritten and missing entries are removed;
- collection membership stores an internal document position separate from imported Legado `customOrder`; synchronization preserves Source IDs by exact imported definition first, then by the prior persisted document order, and writes the incoming document order for the next synchronization;
- an immutable NovelReader Source ID identifies each installed definition, while duplicate `bookSourceUrl` values are allowed within and across collections and remain independent through Search, Explore, candidate resolution, stored bindings, and source sessions;
- automatic synchronization is off by default, with only manual, daily, and weekly schedules;
- standalone sources and existing individual source editing/deletion remain supported;
- Search, Explore, candidate, and reading workflows continue consuming the existing flat effective source list and do not depend on collection concepts.

The implementation includes transactional collection storage, authenticated upload/URL/rename/replace/sync/delete endpoints, bounded public URL retrieval, collection-aware management UI, immutable Source IDs across runtime workflows, and a small reader-runtime-owned daily/weekly scheduler that reuses manual synchronization semantics.

### Complete foundations

- Per-reader authentication, ownership, isolated storage, administration, recovery, registration policy, password change, and durable reader deletion.
- Lossless BookSource import/export and backend-owned request/rule execution.
- Complete search → Book Info → TOC → content domain pipeline with structured failures.
- HTTP, fingerprint, typed-data, and optional Patchright WebView transport boundaries.
- Bounded workflow sessions, browser admission, process capacity controls, and deterministic load coverage.
- Batched streaming Search with cumulative alternate-source discovery.
- Asynchronous readable-candidate resolution and no-recrawl shelf commit.
- Stored Book Detail, source discovery, source switching, Shelf, Reader, progress, bookmarks, offline cache, and display-only Chinese conversion.
- Strict single-source Explore.
- BookSource management, Settings, WebView diagnostic, reader account, and administrator reader management.
- Production Vue frontend replacement with no active Svelte runtime.
- Standalone Docker Compose deployment and successful GHCR publication workflow.
- Removal of superseded candidate-ingestion and unbatched Search API surfaces.

### Current compatibility position

The major crawl and Explore vertical slices are operational, but full Legado compatibility is not complete. Remaining work is concentrated in shared rule and JavaScript semantics rather than missing end-to-end product journeys.

Known incomplete areas include:

- remaining Default/JSoup/CSS/XPath/JSONPath/Regex edge semantics;
- complete Java bridge surface and mutable entity behavior;
- cookie, source-variable, and login-state parity for advanced sources;
- JavaScript timeout, recursion, and runtime-isolation closure;
- advanced source login/browser interactions;
- media-oriented and Android-only source behaviors.

Detailed compatibility backlog: `docs/LEGADO_COMPATIBILITY_TASKS.md`.

### Work in progress

The current `feat/reader-backup-restore` branch adds credential-free, cross-account Reader Data backup and replacement restore:

- exports stream a versioned `.tar.gz` physical snapshot with a source-username and system-timezone timestamp in the download filename;
- the payload contains `reader.db` and ordinary Reader Data files, while account credentials, Application Sessions, API tokens, source credentials, locks, and SQLite sidecars remain owned by the target account;
- same-schema archives remain manually restorable by copying the documented payload into a stopped target Reader Directory;
- web restore validates into same-root staging, briefly quiesces only the target reader, atomically publishes with rollback, and leaves other readers online;
- archive format compatibility stays separate from Reader schema compatibility so future staged `N → N+1` migration steps can be added without changing HTTP, UI, or cutover behavior;
- reader-owned hash-only API tokens use separate `backup:export` and `backup:restore` scopes, with export-only recommended for scheduled automation;
- a dedicated Backup & Restore page owns download, upload/validation/confirmation, operation progress, and token management.

Completion requires safe bounded archive handling, consistent SQLite snapshots, target-identity rewriting, current-schema validation, rollback-protected runtime cutover, scoped token authentication, frontend integration in all locales, and focused backend/frontend verification.

The previously documented `perf/reader-startup` slice remains integrated separately:

- fingerprinted static assets receive immutable long-term caching while the app shell remains revalidated;
- Vue feature routes load lazily;
- Reader metadata and TOC requests start concurrently;
- custom-font inventory loads only when required;
- registration policy and account bootstrap requests run concurrently after setup closes;
- startup loads only the selected locale.

Focused backend tests, frontend typecheck/lint, all 112 frontend tests, production build, layout checks, and desktop/mobile Playwright verification are green. The next action is to integrate this isolated performance change without mixing unrelated work.

## Roadmap

### 1. Integrate the current performance slice

Integration criteria:

- preserve all Reader failure and source-recovery behavior;
- preserve session/setup routing semantics;
- retain reproducible builds without dependency drift;
- pass focused backend tests, full frontend tests/build, and real desktop/mobile browser verification;
- preserve its isolated commit and integrate cleanly.

### 2. Continue shared Legado compatibility convergence

Use deterministic audits and real sources to select the next shared compatibility seam. Prioritize behavior that unlocks multiple sources or a core text-reading workflow.

Current priority families:

1. remaining rule-engine semantics and typed intermediate behavior;
2. JavaScript bridge/session parity used by text BookSources;
3. mutable book/source context and durable variables;
4. request/cookie/login integration required during normal source execution;
5. regression closure for observed shared failure categories.

Do not implement a source-specific adapter when a reusable Legado behavior explains the source.

### 3. User-visible source login

Interactive per-BookSource login remains deferred until its architecture is explicitly approved. A complete design must account for heterogeneous `loginUi`, script-based `loginUrl`, `loginCheckJs` during normal requests, credentials, cookies, WebView, captcha/user interaction, and source-scoped durable login state. It is not merely a standalone credential form.

### 4. Operational and diagnostic refinement

Add diagnostics only for demonstrated needs. Likely future work includes redacted request/rule tracing and clearer source-execution evidence. Do not introduce continuous BookSource health monitoring; failures should surface when a user-triggered operation actually fails.

## Durable Decisions

| Area | Decision |
|---|---|
| Compatibility | Follow documented and observed Legado behavior; fix shared seams before source-specific behavior. |
| Frontend boundary | Vue consumes typed domain APIs and never executes BookSource rules. |
| Work ownership | Go owns fetching, parsing, aggregation, persistence, source sessions, and other resource-intensive work. |
| Frontend framework | Vue 3 SPA, Vue Router, narrow Pinia, Vue I18n, Vite, Options API convention. |
| Routing | Hash history until deployment requirements justify server route handling. |
| Source identity | Immutable NovelReader Source ID; `bookSourceUrl` is imported Legado address data and may duplicate. |
| Logical book identity | Normalized title plus author, with source bindings beneath it. |
| Candidate validation | Backend-owned asynchronous operation with five work-conserving pipelines and first fully readable winner. |
| Candidate persistence | Server-held verified result; idempotent no-recrawl commit. |
| Explore | One selected BookSource and its native catalog at a time. |
| Request architecture | One shared executor and transport-neutral request/response contract. |
| Browser runtime | Private Python Patchright worker over a small versioned HTTP boundary. |
| Runtime state | Explicit per-user/per-source SourceSession; no cross-user/source leakage. |
| Storage | `system.db` plus one self-contained reader home per immutable user ID. |
| Schema policy | Recreate disposable pre-public data; no migration machinery by default. |
| Unknown source fields | Preserve losslessly. |
| Errors | Structured and explicit; never collapse failures into silent empty results. |
| Reader conversion | Frontend display-only; canonical text and reading state remain unchanged. |
| Source health | User-triggered diagnostics only; no continuous background monitoring. |
| Go toolchain | Go 1.27.0 is authoritative in `backend/go.mod`, and CI follows that module directive; Docker retains its existing unpinned Go Alpine builder image. |
| Deployment | Standalone production Compose plus separate deterministic E2E Compose contract. |

## Constraints and Out of Scope

Unless separately approved:

- no captcha solving or automatic WAF bypass;
- no source-specific production patches merely to make one fixture pass;
- no frontend crawling or source-rule interpretation;
- no public-data migration framework during disposable pre-public development;
- no Kubernetes, distributed writable data root, or multi-instance session coordination;
- no cross-source Explore recommendation feed;
- no audio/TTS/RSS/media-domain implementation;
- no Android-only UI extension emulation;
- no broad health polling of imported BookSources;
- no automatic interactive source-login implementation;
- no speculative repository/store abstraction or global frontend state for feature-local workflows.

## Verification Policy

Match verification effort to the change, but compatibility and cross-stack behavior must be proven at the correct boundaries.

For BookSource execution changes:

1. identify the upstream Legado behavior and shared seam;
2. add a deterministic failing regression using captured or synthetic evidence;
3. implement the smallest shared fix;
4. run focused package tests, then broader tests only when coupling or risk warrants them;
5. reproduce relevant live behavior when the change concerns upstream execution;
6. classify failures by transport, site/WAF/DNS, session, analyzer, workflow, storage, or frontend;
7. record non-obvious findings in `DEVELOPMENT.md` or the relevant audit document.

For frontend behavior:

- run typecheck, ESLint, focused tests, and production build;
- use real Playwright interactions against the real backend for meaningful user journeys;
- inspect desktop and mobile layouts, visible states, overflow, and browser console output;
- do not substitute direct API calls for a required visible UI verification.

For container/deployment changes:

- validate both Compose contracts;
- build the affected images;
- run deterministic Compose E2E when Docker access permits;
- report environment blocks explicitly rather than implying runtime verification passed.

## Documentation Ownership

- `PLAN.md` — current architecture, state, priorities, decisions, and constraints.
- `DEVELOPMENT.md` — append-only history of non-obvious changes and discoveries.
- `docs/CANDIDATE_BOOK_JOURNEY.md` — candidate operation and user-journey contract.
- `docs/AUTHENTICATION_DESIGN.md` — authentication and reader ownership design.
- `docs/USER_STORAGE_IMPLEMENTATION_TASKS.md` — storage implementation checklist and legacy-removal evidence.
- `docs/LEGADO_COMPATIBILITY_TASKS.md` — detailed compatibility backlog.
- `docs/CODEBASE_CLEANUP_AUDIT.md` — maintainability cleanup findings and completed removals.
- `testdata/booksource/audits/` — immutable deterministic live-audit evidence.
- `docs/archive/PLAN_HISTORY_2026-08-27.md` — historical snapshot of the former detailed plan; not a current source of truth.

Update this plan only when the objective, architecture, current phase, next action, durable decisions, or real constraints change. Do not turn it back into a bug diary or audit log.
