# NovelReader — Engineering Plan

## Objective

Build a web-first novel reader whose core booksource engine is behaviorally compatible with Legado BookSource JSON. Compatibility means preserving source data, constructing every request according to Legado URL rules, evaluating Default/CSS/XPath/JSONPath/Regex/JavaScript rules with Legado semantics, maintaining cookies and source variables, crawling detail/TOC/content consistently, and exposing a transport seam for HTTP and WebView execution. The frontend consumes stable domain APIs and must not contain source-specific crawling logic.

“Near-perfect compatibility” is the target for the documented Legado contract. Sites can still fail because of DNS loss, WAF policy, authentication, captchas, or changed HTML; those must be reported as transport/site failures rather than misclassified as parser failures.

## Architecture

### Directory layout

The existing feature-oriented layout remains canonical:

```text
backend/internal/
  api/          HTTP/SSE transport only
  booksource/   canonical source model, import/export, persistence
  sourceexec/   unified URL/request/session/transport execution (new)
  analyzer/     Legado rule engine and JS bridge
  fetcher/      HTTP transport implementation and transport-neutral HTTP client contract
  fingerprint/  optional TLS/HTTP fingerprint transport adapter
  webview/      Go client and session-scoped WebView transport
  book/         search, enrichment, TOC, content domain workflows
webview-worker/  Python Patchright headless browser worker
  processor/    content safety and paragraph formatting
  database/     SQLite setup/migrations
frontend/src/
  api/          typed backend client
  lib/          user-facing features and reusable reader components
reference/legado/                upstream behavioral reference
backend/internal/**/*_test.go    conformance tests beside modules
testdata/booksource/              small fixtures extracted from real sources
```

New directories require a PLAN update before creation. `sourceexec` and `webview` are planned boundaries, not permission to duplicate logic.

### Core contracts

#### Canonical BookSource

- Preserve every imported JSON field, including unknown/future Legado fields.
- Keep typed fields for fields NovelReader executes.
- Retain original JSON for lossless export and debugging.
- Use `bookSourceUrl` as the stable source identity; names are display labels and may duplicate.
- Do not silently coerce malformed rules into empty strings. Import must report field-level warnings/errors.

#### Request execution

All search, detail, TOC, content, JS `java.ajax/get/post/connect`, pagination, image, and future explore requests use one request contract:

```text
RequestSpec
  URL, method, body, headers, charset, retry
  webView, webJs, bodyJs, type, origin, dnsIp
  source identity and session context

Response
  requested URL, final URL, status, headers, decoded body/bytes
  transport kind, redirect chain, timing, error classification
```

`HTTPTransport`, `FingerprintTransport`, and `WebViewTransport` implement the same interface. Fingerprint transport is an injected adapter, not a dependency of `book` or `sourceexec`. The selector is a policy decision in the executor, not a branch duplicated inside each book workflow. The initial policy is fingerprint-first with normal HTTP fallback; the browser profile is configurable and defaults to the newest pinned Chrome profile.

#### SourceSession

A source session owns:

- cookie jar and cookie helper operations;
- source variables and memory cache;
- source headers and rate limit state;
- JS library/context;
- request-scoped book/chapter context.

WebView sessions remain scoped to the same SourceSession. The Go `webview` client sends a small JSON request to the Patchright worker; the worker owns Chromium lifecycle and returns a transport-neutral response plus cookies. Book workflows do not import or know about Patchright.

Sessions are isolated by user/source identity. Search must not share cookies across sources or users. Detail → TOC → content for one source may share that source’s session.

#### RuleEngine values

The analyzer must preserve typed intermediate values where possible: HTML element selections, JSON objects/lists, strings, and JS values. Re-serializing every element to inner/outer HTML is a compatibility fallback, not the primary model.

Supported documented modes and semantics:

- Legado Default/JSoup syntax;
- explicit CSS, XPath, JSONPath, Regex;
- JavaScript and `<js>` blocks anywhere allowed by Legado;
- `&&`, `||`, and `%%`;
- indices, ranges, negative indices, exclusions, and reverse ranges;
- `##regex##replacement` and replace-first `###`;
- URL-context resolution and `isUrl`;
- `@put`/`@get` rule variables;
- JS `java.getString`, `getStringList`, `getElements`, `setContent`.

### Data flow

```text
Frontend search/detail/reader
  → API domain handler
  → book workflow
  → SourceExecutor.Execute(RequestSpec)
  → HTTPTransport or WebViewTransport
  → Analyzer with SourceSession/context
  → typed domain result
  → API DTO
```

No frontend feature may construct source URLs or interpret source rules. Explore, bookmarks, offline reading, and source debugging will reuse domain/API contracts.

### Error model

Every stage returns structured errors with:

- source URL and stable source identity;
- workflow stage (`search`, `bookInfo`, `toc`, `content`, `js`, `transport`, `rule`);
- request/final URL and status when available;
- rule field and mode when parsing fails;
- retry count and transport type.

A non-empty HTTP body with a non-200 status is not automatically “dead”; it is retained for classification and optional parsing policy. Empty extraction is not automatically “outdated.”

## Compatibility reference

The implementation is audited against:

- `reference/legado/app/src/main/java/io/legado/app/model/analyzeRule/AnalyzeUrl.kt`;
- `AnalyzeRule.kt`, `AnalyzeByJSoup.kt`, `AnalyzeByXPath.kt`, `AnalyzeByJSonPath.kt`, `AnalyzeByRegex.kt`;
- `JsExtensions.kt`;
- [Legado source-rule documentation](https://mgz0227.github.io/The-tutorial-of-Legado/Rule/source.html).

When behavior is ambiguous, add a fixture and cite the upstream function/documentation section in the test comment.

## Phases

### Phase 0 — Compatibility baseline and harness

**Goal:** make failures reproducible before changing behavior.

Tasks:

- [x] Create fixture corpus for raw search, detail, TOC, content, JSON, XPath, Regex, JS, POST/GBK, cookie, pagination, and WebView-option sources.
- [x] Build a source identity tool keyed by raw `bookSourceUrl` plus JSON index/hash, never name alone.
- [x] Add a conformance runner that records raw source JSON, expanded request, method/body/headers, response status/final URL/body sample, rule field, extracted values, and classification.
- [x] Add golden tests for the core regression classifications and request contracts.
- [x] Define expected categories: transport failure, HTTP/WAF, legitimate zero results, rule mismatch, JS failure, unsupported WebView, and successful extraction.
- [x] Verify the server remains alive during the test; abort a run on process crash instead of continuing with contaminated results.

Completion gate: deterministic tests can distinguish a broken request from a broken selector and reproduce a raw Legado request independently.

### Phase 1 — Lossless source model and unified request executor

**Goal:** every workflow uses Legado-equivalent URL execution.

Tasks:

- [x] Preserve unknown BookSource fields and losslessly export imported JSON.
- [x] Implement `RequestSpec`, structured `Response`, and `SourceSession`.
- [ ] Move URL template expansion, JSON options, `<,>` page syntax, relative URL resolution, URL-level JS, body JS, charset, headers, retry, redirect metadata, and method/body construction into `sourceexec`.
- [ ] Apply URL options to search, book info, TOC, content, `nextTocUrl`, `nextContentUrl`, and JS bridge requests.
- [x] Execute `dnsIp` for normal HTTP requests while preserving the original request host; fingerprint transport delegates this option to normal HTTP fallback.
- [x] Execute URL-option `HEAD` requests through normal and fingerprint transports.
- [x] Use RFC-compatible URL resolution against the actual response/found page URL.
- [x] Preserve response bodies for non-2xx responses and classify rather than discard them.
- [x] Add request-parity diagnostics for enrichment: record exact expanded URL, method, headers, redirect chain, status, and body when NovelReader differs from curl/browser.
- [ ] Reproduce and resolve the `m.22biqu.net` enrichment discrepancy where curl returned 200 but NovelReader observed 404.
- [ ] Implement per-source/per-user cookie policy and source-variable persistence.
- [ ] Ensure `Referer`, `Origin`, content type, custom headers, and cookies are forwarded exactly as the source requests them.
- [x] Implement `bodyJs`, `webJs` dispatch metadata, and response-body transformation hooks.

Completion gate: one fixture request produces the same method, URL, body, headers, charset, cookies, redirects, and final body behavior as Legado. Search/detail/TOC/content all call the same executor.

### Phase 2 — Legado rule-engine conformance

**Goal:** eliminate heuristic behavior differences in rule evaluation.

Tasks:

- [ ] Implement explicit Regex mode for `##...##...` rules.
- [ ] Implement exact Default/JSoup grammar: selectors, getters, direct children, `@attr`, numeric selectors such as `.directoryArea.1`, CSS-compatible `:eq(n)` selectors such as `.directoryArea:eq(1)`, indexes, negative indexes, exclusions, arrays, ranges, steps, and reverse ranges.
- [ ] Implement exact `&&`, `||`, and `%%` semantics for strings, elements, and lists.
- [ ] Implement `<js>` chain semantics and result propagation at every documented position.
- [ ] Preserve typed HTML/JSON intermediates and correct `outerHtml`, `html`, `text`, `textNodes`, `ownText`, `all`, `href`, and `src` behavior.
- [ ] Handle plain JSON property access and JSONPath according to Legado’s object/list model.
- [ ] Implement `@put`/`@get` and Java variable operations with source/chapter scope.
- [ ] Ensure replacement rules distinguish replace-all from replace-first.
- [ ] Replace `mustString` silent recovery with typed errors and field-level diagnostics.

Completion gate: conformance fixtures pass against expected Legado outputs for every supported mode and connector; no parser error is reported as an empty value.

### Phase 3 — JavaScript bridge and session semantics

**Goal:** execute real JavaScript sources instead of only simple URL expressions.

Tasks:

- [ ] Implement `java.get`, `post`, `ajax`, `ajaxAll`, `connect`, `getCookie`, `base64`, MD5, HMAC, URI encoding, time formatting, and response accessors with Legado-compatible return shapes.
- [ ] Implement `java.setContent`, `getString`, `getStringList`, and `getElements` against the current analyzer/session.
- [ ] Implement `source.get/put`, `getVariable/putVariable`, `cache`, `book`, `chapter`, `title`, `baseUrl`, `src`, and `result` scopes.
- [ ] Make JS runtime state safe under pooling: no cross-source leakage, deterministic bindings, persistent state only in SourceSession.
- [ ] Implement cookie operations through the actual session jar.
- [ ] Add JS timeout, cancellation, and bounded network recursion.
- [ ] Add tests for token extraction, multi-stage `java.ajax().match()`, source variables, cookies, and JS-defined helper libraries.

Completion gate: all previously observed `Object has no member`, `ReferenceError`, empty-cookie, and empty-`java.getString` categories have regression tests and pass.

### Phase 4 — Complete crawl pipeline

**Goal:** search → book info → TOC → content behaves consistently.

Tasks:

- [x] Audit every Phase 4 checkbox against current code, deterministic tests, and Legado behavior; mark only evidenced behavior complete and turn confirmed gaps into focused TDD slices.
- [x] Enrich book info with `bookInfoInit` behavior.
  - [x] Preserve selector collections plus structured JSON/JavaScript initialization content before detail-field rules, including `&&` merge, `||` alternatives, null failure, table context, and uncapped XPath selections.
  - [x] Execute shared Legado `@put:{...}` / `@get:{...}` variables in parsed-rule order with session persistence, lenient raw maps, dynamic replacements, JavaScript substitution, and all-in-one regex initialization.
- [x] Parse TOC with correct documented reversal semantics, volumes, VIP flags, timestamps, relative URLs, and duplicate handling.
- [x] Implement TOC pagination with cycle detection, request options, retries, and explicit partial-failure reporting.
- [x] Implement `nextContentUrl` pagination and content concatenation.
- [x] Bind complete book/chapter context for every rule evaluation.
- [x] Preserve source/final URLs for subsequent relative content requests.
- [x] Make content extraction use the declared rule first; keep SPA/script fallback as an explicit diagnostic fallback, never as a replacement for rule correctness.
- [x] Preserve typed crawl diagnostics at API boundaries; distinguish upstream crawl failures, storage failures, and missing resources with stable status/code responses.
- [x] Add full E2E tests using raw source entries: search actual book, add to shelf, enrich, load TOC, open first/middle/last chapter, and verify non-empty content.

Audit checkpoint (2026-07-15, updated 2026-07-16): final/source URL continuity, `bookInfoInit`, complete typed book/current/next-chapter context, TOC semantics, fail-closed TOC pagination, content pagination, declared-rule-first extraction, API failure propagation, the raw-source first/middle/last workflow, and the source-class completion matrix are complete. Dead wrappers and heuristic TOC/content replacements are removed; Phase 4 is closed.

Completion gate: **passed** — deterministic end-to-end workflows cover normal HTML, JSONPath, XPath/Regex, GBK POST, and multi-page TOC/content source shapes.

### Phase 5 — WebView transport with minimal refactor

**Goal:** support browser-backed sources in headless Docker without coupling book workflows to a browser library.

Tasks:

- [x] Define `WebViewTransport` behind the same `RequestSpec`/`Response` interface.
- [x] Add the Python Patchright worker over a localhost HTTP boundary; pin the package and browser install.
- [x] Add a Go `webview` client that owns the protocol, timeout, cancellation, and SourceSession cookie synchronization.
- [x] Route `webView:true` requests through WebView while normal requests retain fingerprint/HTTP policy.
- [x] Support page JavaScript, `webJs`, delayed execution, cookies, redirects, headers, and final DOM/response capture.
- [x] Add a configurable browser lifecycle, timeout, concurrency limit, and cancellation policy.
- [x] Make WebView optional at build/deploy time; HTTP-only deployments report a precise unsupported capability.
- [x] Add a fake WebView transport for deterministic tests.

Completion gate: a source marked `webView:true` can run through the same search/detail/TOC/content workflow without book-domain changes.

### Phase 5.5 — Capacity and lifecycle hardening

**Goal:** keep hundreds of sources efficient without unbounded memory, browser work, or connection churn.

Decisions: browser overload waits within the existing request deadline; workflow sessions use TTL plus a bounded LRU. The browser worker remains bounded by pages, pending requests, and browser recycling. Normal HTTP pools are shared; ephemeral search fingerprint pools close when the source finishes, while staged workflow fingerprint clients remain session-scoped until eviction.

Tasks:

- [x] Add bounded-wait/retry for transient browser-worker overload.
- [x] Add TTL/LRU eviction to the workflow session registry.
- [x] Reuse stateless HTTP connection pools across source sessions.
- [x] Cache fingerprint clients within SourceSession lifetime.
- [x] Add per-request and process-wide capacity metrics.
- [x] Verify hundreds-source search and concurrent-user behavior with deterministic load tests.

Completion gate: load tests show bounded memory and no permanent busy failures under configured capacity; session and browser lifetimes are observable and reclaimable.

### Phase 5.6 — Batched streaming search

**Goal:** make search coverage predictable for any enabled-source count while preserving bounded execution and cumulative results.

Decisions: the browser owns cumulative search state; the backend remains stateless between batches. A versioned cursor identifies the next offset and hashes the deterministically ordered eligible source IDs. Source-list changes require restart. Batch size controls coverage; requested concurrency controls only per-search parallelism and is clamped by deployment limits.

Tasks:

- [x] Preserve legacy all-source `Search`/`SearchStream` behavior behind shared fan-out code.
- [x] Add deterministic source ordering, versioned cursors, source-list revisions, and exact batch partitions.
- [x] Add requested per-batch concurrency without weakening global, JS, or browser capacity limits.
- [x] Extend search SSE with start, progress, completion, retry, and stale-source events.
- [x] Add cumulative frontend result merging that deduplicates retries and preserves alternate sources.
- [x] Add persistent batch-size and search-intensity controls, cumulative progress, Stop, Retry, Search more, and Restart behavior.
- [x] Verify alternate sources survive shelving from merged search results.

Defaults: 50 sources per batch, user range 1–500, and Balanced concurrency 8. Gentle/Balanced/Fast map to 4/8/16; an advanced positive value is allowed but the server reports and enforces the effective deployment cap.

Completion gate: deterministic tests prove batches have no gaps or overlaps, stale cursors execute no source work, cancellation retries safely, cumulative merging loses no alternate sources, existing callers remain compatible, and browser verification passes the complete search-control lifecycle.

### Phase 6 — Diagnostics, frontend extensibility, and operational hardening

**Completed slice: production container delivery.** One rootless frontend/backend image and one private-network Patchright image target `linux/amd64`. A single Compose contract supports GHCR pulls and local builds, persists `/data`, treats WebView as an optional profile, and proves frontend, SQLite readiness, graceful shutdown, persistence, and app-to-worker execution without live sites. GitHub Actions publishes lowercase `ghcr.io/otwako/novelreader` and `ghcr.io/otwako/novelreader-webview` on `main`, `v*`, and manual dispatch. Packages are intended to be public after their first publication; visibility remains a manual GitHub package setting.

Container-delivery tasks:

- [x] Add SQLite-backed `/api/healthz` readiness and graceful SIGTERM/SIGINT shutdown.
- [x] Add rootless app and worker images from latest official base tags with runtime health checks.
- [x] Add one Compose contract for GHCR deployment, local builds, persistent data, and optional private WebView.
- [x] Add deterministic clean-checkout Compose verification for frontend, readiness, persistence, shutdown, and WebView routing.
- [x] Add SHA-pinned GitHub Actions publishing for both lowercase GHCR images on `main`, `v*`, and manual dispatch.
- [x] Document public-package setup, image tags, data ownership/backup, health, and HTTP-only/WebView deployment.

Out of scope for this slice: Kubernetes, reverse proxy configuration, database replacement, image signing/SBOM, release automation, and non-amd64 platforms.

Tasks:

- [ ] Add source debug API: preview request, headers/body (redacted), response metadata, rule-by-rule extraction, and JS logs.
- [ ] Add source health history without labeling failures as permanently outdated.
- [x] Add explore/discovery using the same executor and rule engine.
- [ ] Add source editor/import validation and raw JSON round-trip preview.
- [ ] Add frontend source-debug, source health, and crawl-progress components using typed API contracts.
- [x] Add reading progress, bookmarks, offline cache, and alternate-source switching without coupling them to source parsing.
- [x] Add Docker multi-stage build and clean-checkout E2E verification.

Completion gate: frontend features consume stable domain APIs; no frontend code knows CSS/Default/JS source syntax.

#### Reading-state execution order (2026-07-18)

Priority is fixed as: (1) reliable chapter/scroll resume, (2) safe alternate-source switching, (3) bookmarks, and (4) server-side offline chapter cache. NovelReader remains single-user while authentication/profiles are out of scope.

The resume slice reuses existing `books.dur_chapter_index`/`dur_chapter_pos` rather than adding schema: validate progress writes and missing books, expose both values through the typed client, continue from shelf/detail, restore normalized in-chapter position after content renders, debounce persistence, avoid stale chapter fetches, skip volume-only TOC rows, and keep progress display accurate. Completion requires store/API regressions plus desktop/mobile reader verification across reload and direct deep links.

Source switching keeps one canonical reading position and migrates it on every switch: normalized chapter-title match first, then clamped raw chapter index as a deliberately approximate fallback while preserving in-chapter position. This replaces the earlier per-source-state plan because imported sources are frequently modified or removed, and avoids stale state semantics rather than optimizing negligible storage. The switch must prove the target source/TOC before committing, remain reversible, and expose whether fallback mapping was used. Offline means processed chapter content persisted server-side under bounded retention so reading survives upstream outages; browser-disconnected PWA behavior remains out of scope.

### Phase 7 — Strict per-source Explore/Discover

**Status:** complete on 2026-07-18; Phase 4's crawl-pipeline completion gate passed on 2026-07-16.

**Goal:** provide a complete, cleanly separated Explore experience that executes each imported BookSource's native Legado discovery contract without inventing a NovelReader-only source format.

Decisions: Explore is per-source only. `enabledExplore`, `exploreUrl`, `exploreScreen`, and `ruleExplore` are the compatibility boundary. Explore reuses the shared source executor, session, transport, and analyzer contracts; frontend and API consumers receive typed domain results and never evaluate source syntax.

Execution order is gated: (1) upstream/raw-source audit, (2) typed contracts and data flow, (3) deterministic fixtures, (4) domain service, (5) API, (6) frontend, and (7) live verification. Do not start a later gate while an earlier contract is unresolved. Reorder only when audit or test evidence exposes a prerequisite; record the change and rationale here before implementation. Add Phase 6 diagnostics only when Explore demonstrates a concrete observability gap rather than building the whole diagnostics phase first.

Tasks:

- [x] Audit Legado's Explore URL/category grammar, screen/layout metadata, pagination, rule context, result model, failure semantics, and source enablement behavior against raw sources and the local upstream reference.
- [x] Define typed per-source Explore contracts and data flow before implementation, including category navigation, paging state, source identity, and diagnostics.
- [x] Add deterministic raw-source fixtures covering normal HTML, JSON, XPath/Regex, POST/charset, JavaScript, and WebView exploration where those capabilities occur in real sources.
  - [x] Extract and hash-guard exact raw entries for indices 1, 3, 7, 12, 160, 184, and 916.
  - [x] Make lenient array and legacy category parsing executable against the pinned index-1 fixture, rejecting executable values rather than evaluating them.
  - [x] Execute the pinned index-916 JavaScript catalog plus local session-cache/header/AJAX and tagged-script fixtures.
  - [x] Add captured/local response fixtures with executable request, paging, and result expectations for raw indices 1, 3, 7, 12, 160, and 184, including emitted and selected WebView options.
- [x] Implement the smallest Explore domain service on the existing executor/analyzer/session boundary; do not route it through search fan-out or duplicate request/rule logic.
  - [x] Add independently eligible source listing, typed literal/legacy catalogs, bounded opaque sessions, and sequential URL-category pages through the shared executor/result parser.
  - [x] Execute top-level `@js:`/`<js>` category generators with source headers, cookies, cache, `infoMap`, JSLib, timeout, and the same retained session used by pages.
  - [x] Execute validated `text`/`button`/`toggle`/`select` actions with retained session bindings, explicit refresh, and generation-scoped entry IDs.
- [x] Add stable API endpoints for enabled Explore sources, source-native screens/categories, paged results, and explicit partial/failure diagnostics.
- [x] Add an accessible per-source frontend flow that preserves source-native navigation and paging without exposing rule syntax.
- [x] Verify raw-source round trips and live per-source Explore behavior through Playwright, recording exact source identity and request/rule evidence.

#### Explore audit and contracts (2026-07-16)

Audit identity: local Legado `44e07fea541287804cc58d0168940a756cd11cfd`; raw `test_booksource4.json` SHA-256 `23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c`, 939 entries. Of these, 722 independently Explore-enabled sources have nonblank `exploreUrl`; observed shapes include 373 array-like values, 285 legacy `title::url` values, 77 top-level JavaScript generators, 612 literal page templates, 67 angle-bracket page selectors, POST, JSONPath, XPath/Regex, and WebView rules. Stable fixture candidates are raw indices 1, 3, 7, 12, 160, 184, and 916, always guarded by the full-file hash plus source URL.

Compatibility requirements:

- Eligibility is `enabledExplore && exploreUrl != blank`, independent of normal search `enabled`; an omitted imported `enabledExplore` retains Legado's default `true`.
- Parse array-shaped categories with Legado-compatible leniency (matching Gson's accepted raw forms); an array-shaped parse failure is a typed category diagnostic, never a fallback to legacy splitting. Non-array values use legacy newline/`&&` entries split as `title::url`. Evaluate whole-field `@js:`/`<js>` first with source, cookie, cache, base URL, and per-session `infoMap` bindings.
- Support `url`, `text`, `button`, `toggle`, and `select` entries. Delivery is staged: URL categories form the first vertical slice, but typed controls are frozen now and all controls must work before Phase 7 closes.
- Preserve `exploreScreen`, category style, `ruleExplore.updateTime`, and unknown imported fields losslessly, but do not invent runtime semantics for currently unused Android-only layout metadata.
- Selected URLs use the existing executor contract: page/key JavaScript, `<...>` page selectors, URL options, headers/charset/body, retry, bodyJs, DNS/fingerprint, and WebView routing.
- If `ruleExplore.bookList` is blank, use the complete `ruleSearch` object. Otherwise use `ruleExplore`; do not merge fields individually. Preserve reverse-prefix behavior, relative resolution against the final response URL, blank-name filtering, URL deduplication, and Legado's single-book fallback when list extraction is empty and no `bookUrlPattern` exists.
- Page 1 is initial. Paging is server-authoritative and serialized per session/category: accept only the expected next page, cache/replay the last successful page idempotently, reject skips/stale pages with the expected page, and never advance on transport/rule failure. Empty or duplicate-only pages set `exhausted=true`.
- Category-generation errors become typed diagnostics rather than synthetic `ERROR:` categories. Raw Explore/category URLs, URL options, rule syntax, scripts, stack traces, and secrets never cross the domain API; result `bookUrl`/`coverUrl` and the existing source ID remain available because shelving and the established detail/TOC workflow require them.

Typed domain/API boundary:

- `ExploreSource { id, name, group }` exposes only independently eligible sources.
- `ExploreCatalog { source, sessionId, entries, diagnostics }` contains opaque entry IDs. Entries are a tagged union: URL entries expose display title and selectability; controls expose type, display title, current value, and allowed options, never raw URL/action/viewName expressions.
- `ExplorePageRequest { sessionId, categoryId, page }` and `ExplorePage { sourceId, sessionId, categoryId, page, nextPage, books, exhausted, diagnostics }` reuse `book.SearchResult` for books. `page` is an optimistic sequencing value: the service serializes category execution, replays the cached last success, and returns `page_conflict` plus `nextPage` for skips/stale requests.
- `ExploreDiagnostic { code, stage, severity, retryable, message }` distinguishes invalid source/session/category/value, unsupported capability, category script/parse, request build, transport/WebView, HTTP status, and result-rule failures.
- An opaque random session ID owns one bounded-TTL source session, parsed categories, `infoMap`, per-category paging/dedup state, cookies, variables, and transport state. Category/control IDs are stable only within that session. This is process-local like existing workflow sessions; multi-instance/user routing remains out of scope until the application gains authentication or shared session storage.
- Additive endpoints are `GET /api/explore/sources`, `POST /api/explore/catalog`, `POST /api/explore/control`, and `POST /api/explore/page`. Control updates execute server-side actions and return a refreshed typed catalog. API handlers perform validation/mapping only; execution stays in the book-domain Explore service.

Data flow: source store → eligibility/catalog service → bounded Explore session → existing `sourceexec.Executor`/routing transport → analyzer with Explore context → shared book-list parser → typed `SearchResult` page → thin API → frontend. Search fan-out, raw `/api/sources`, and frontend rule evaluation are not part of this flow.

Out of scope: unified cross-source feeds, recommendation ranking, cross-source deduplication, editorial metadata, NovelReader-only source schema extensions, Android flexbox styling, cross-instance Explore sessions, and authenticated multi-user isolation.

Completion gate: representative raw sources execute source selection → native category/screen → pagination → book result → existing detail/TOC/content workflow with deterministic and live evidence; unsupported source behavior fails explicitly rather than silently returning an empty feed.

#### Live compatibility audit (2026-07-18)

Manual testing found broad Explore failures despite the deterministic Phase 7 gate: parse errors, result-rule failures, empty pages, and list extraction collapsing multiple books into one. Before changing behavior, audit a deterministic stratified sample of 50 sources from hash-pinned `test_booksource4.json` (`23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c`) in a fresh isolated database. Include the reported `中文看书（优）` raw index 89, both `夜伴书屋` identities at indices 9 and 752, and duplicate-name `笔趣阁` sources by stable index/URL rather than display name.

For each sample, record raw index/URL, Explore grammar and result-rule class, first selectable category, exact expanded request, engine status/diagnostic, result count, and whether distinct book URLs remain distinct. Reproduce every apparent engine failure against the upstream page with Playwright/direct HTTP and classify it as request construction, transport/DNS/WAF, category parsing, rule/parser, single-book fallback, frontend state, legitimate empty result, or stale source rule. Sampling is one page/category per source to avoid unnecessary upstream load; use a fixed seed and stratify across literal/legacy/JavaScript catalogs, page selectors, GET/POST/charset, Default/CSS/JSONPath/XPath/Regex result rules, and WebView metadata. Report the evidence and ranked shared gaps to the user before writing fixes. Any accepted fix then follows the mandatory captured-fixture TDD loop and reruns affected strata plus the full deterministic suite.

#### Second live compatibility audit (2026-07-19)

Run a disjoint deterministic 50-source sample with seed `NovelReader-explore-audit-v2` against the same hash-pinned 939-source corpus and the engine containing the first approved shared fixes. Exclude all first-audit indices and malformed top-level catalogs before selection; deduplicate by source URL and stratify the remaining candidates across strict/lenient/legacy/JavaScript catalogs, GET/POST/page-selector/charset/WebView requests, and Default/CSS/JSONPath/XPath/Regex/JavaScript result rules. Use the sorted raw indices `3, 10, 40, 46, 61, 76, 82, 94, 97, 123, 134, 146, 165, 173, 175, 181, 183, 201, 204, 209, 231, 232, 282, 299, 342, 358, 391, 403, 410, 413, 414, 426, 429, 437, 439, 451, 470, 471, 516, 537, 614, 629, 658, 661, 742, 780, 787, 807, 816, 920` and identify every result by raw index plus URL.

Execute one first selectable category/page per source in a fresh isolated database, preserve exact engine diagnostics/counts, and reproduce apparent failures against direct browser evidence. Classify independently from the first batch and compare gap recurrence only after evidence is captured. This cycle is audit-only: do not modify production parsing or raw BookSource rules before reporting findings and receiving explicit approval.

#### Approved shared compatibility fixes (2026-07-19)

The user approved generic fixes for the recurring v2 gaps. Implement in four green, atomic phases using reduced captured fixtures and raw-rule shapes before production edits:

1. Complete Default traversal for `@tag.*` and `@.class`/`@#id`, with per-parent indexing and ordered `!index` exclusion while preserving CSS attribute getters.
2. Correct connector and mixed-mode semantics: incompatible/empty `||` branches fall through, blank `&&` branches are ignored, single-brace JSON interpolation is evaluated against the current JSON object, and an optional intermediate JSONPath may feed an empty value to a following JavaScript default without hiding terminal/required-field errors.
3. Add focused Legado JavaScript compatibility: `java.toNumChapter`, non-blocking structured `java.toast`, and shared URL-option parsing so `java.ajax` can execute method/body/header options with cancellation, cookies, redirects, and relative URL resolution intact.
4. Rerun affected v1/v2 identities, immutable fixtures, full backend/race/frontend/build/E2E suites, and desktop/mobile Playwright before reporting.

All behavior must be parser-generic and preserve public APIs. No raw BookSource rewrites, source names/URLs/indices in production logic, new dependencies, or swallowed required-field failures. Authenticated Pixiv/browser/cache/login support required by raw 134 is explicitly out of scope.

## Mandatory implementation → verification loop

Every significant booksource-engine change must follow this loop:

1. **Audit and design** — compare the intended behavior with the Legado source implementation and the source-rule documentation; identify the shared contract and avoid source-specific patches first.
2. **Write a failing deterministic test** — use a captured raw response and the exact raw BookSource rule. The test must name the expected URL, method, headers/body, rule mode, and extracted result.
3. **Implement the smallest shared fix** — change the common executor, session, analyzer, or workflow boundary; do not patch one source unless the raw source itself is demonstrably exceptional.
4. **Run the deterministic suite** — execute the focused test, then `go test ./...`. Stop on any regression.
5. **Run live Playwright verification** — on a fresh server, import the raw compilation, search a real book, add it through the UI, open detail, load TOC, and read content. Record source URL, source index/hash, rule field, request status, and visible result.
6. **Debug failures by layer** — collect the exact expanded request and compare NovelReader with curl and Playwright. Classify the failure as transport/request construction, HTTP/WAF/DNS, session/cookie, rule/parser, workflow state, frontend, or legitimate zero results. Never infer parser failure from a timeout or source failure from an empty selector alone.
7. **Cross-check another raw source** — if the selected source fails, test at least one other source using the same rule feature. If the second source passes, investigate source-specific request/DOM differences; if both fail similarly, treat it as an engine gap until disproven.
8. **Fix and repeat** — return to step 3, add the regression test for the discovered bug, rerun deterministic tests, then repeat Playwright verification. Do not mark the change complete while the live gate is blocked by an unresolved failure in the changed path.
9. **Synchronize documentation** — update `Current State`, append an `Issues & Fixes` entry, record the live verification evidence/limitations, and commit the PLAN update with the code/test change.

**Last-resort rule:** an outdated or incorrect raw BookSource rule may be concluded only after the exact raw URL is reachable, the exact raw response is inspected, the exact rule is reproduced independently, and another source/engine path confirms the behavior is not a shared NovelReader gap.

## Testing strategy

- Unit tests live beside analyzer, sourceexec, fetcher, and book modules.
- Conformance tests are deterministic and use recorded responses; no live site is required for CI.
- Live verification is a separate report and always records raw source identity, exact request, response metadata, and timestamp.
- Every non-trivial parser/request change follows failing test → implementation → passing test.
- Test the same raw source through: direct HTTP reproduction, NovelReader executor, and browser/WebView when applicable.
- Never infer source death from an empty selector result or browser timeout alone.

## Out of scope for the compatibility milestone

- Captcha solving, bypassing WAF policy, and authenticated account automation.
- Audio/TTS/RSS/EPUB features.
- Legado mobile-only UI extensions such as custom buttons and event listeners; preserve them losslessly but do not execute them in the backend.
- Perfect parity with undocumented private Java/Android APIs; unsupported calls must be explicit and observable.

## Current State

**Phase:** Phase 7 Explore/Discover audit and contract design; Phase 4 crawl compatibility, Phase 0 baseline, and the production-container slice are complete.

**Last completed:** Enabled the conformance runner's `-webview-endpoint` path and verified raw index 213 reached Patchright, returned `transport:webview` with HTTP 200, and produced a rule mismatch from rendered DOM rather than an unsupported-transport error. Added and passed the deterministic 2×200-source concurrent search load test, including process-wide admission and zero leaked active work; Searcher capacity snapshots and Patchright `/healthz` counters, plus a 256-concurrent-session registry stress test are also covered. Added process-wide search admission on top of per-search fan-out. Implemented the first Phase 5.5 capacity controls: transient browser busy retry, bounded TTL/LRU workflow sessions, shared stateless HTTP connection pools, and SourceSession-scoped JS/fingerprint client reuse. The current worker also has bounded admission, whole-request deadlines, cancellation-safe context cleanup, graceful shutdown, browser crash recovery, and configurable browser-process recycling. Verified the Go client and a local HTML fixture, including forced browser recycling. Go workflows route `webView:true` requests through a session-scoped Patchright worker client, while HTTP-only deployments fail explicitly. The worker supports isolated browser contexts, page JavaScript, `webJs`, delays, cookies, headers, redirects, and final DOM capture. Preserved structured URL-option JSON bodies and prepared raw-vs-form POST payloads consistently across normal and fingerprint transports. Added URL-option `HEAD` execution through normal and fingerprint transports. Added URL-option `dnsIp` execution for normal HTTP requests while preserving the original host for HTTP Host/TLS behavior. Applied case-insensitive URL/source header overlays across search, book info, TOC, content, pagination, and conformance execution. Source-level headers now also reach JavaScript bridge requests through SourceSession and both normal/fingerprint clients. Preserved JS bridge POST content types through the cookie-session adapter. Added a cookie-session adapter for JS HTTP clients without native `ForSource` support, so stateless `java.get/post/ajax` calls now inject and persist source cookies. Made HTTP transport header merging case-insensitive so explicit lowercase `Origin` and `Content-Type` values are not overridden or duplicated. Completed conformance request diagnostics across normal and fingerprint transports: exact request metadata, redacted response headers, redirect chains, status, and bounded body samples. Updated the conformance parser to resolve search links against each response's final URL, verified live sources 1 and 84 still return 13 and 2 results with absolute book URLs, and classified source 89 as a transport timeout. Threaded source-session state into per-result search analyzers so field-level JS rules can read cookies and variables. Resolved search, cover, and TOC links against the actual final response page URL instead of the source root/book input. Preserved redirect-origin cookies across normal and fingerprint transports by syncing both requested and final URL scopes. Added shared `bodyJs` response transformation after transport success and threaded it through search, book info, TOC, and content pagination. Moved request-body charset encoding into the shared source transports so normal and fingerprint requests use the same `RequestSpec` contract. Preserved imported BookSource JSON (including unknown fields and rule value shapes) through JSON round-trip and SQLite persistence. Completed the deterministic Phase 0 fixture corpus with executable rule expectations, fixed JSONPath object decoding and long-list truncation, and added the production-mode raw conformance runner, golden classification tests, and optional health checks that abort after a server failure. Routed search, book-info, TOC, and chapter content through `sourceexec.Executor`;  content supports URL options, `nextContentUrl`, and Legado’s next-TOC-chapter stop condition; TOC pagination reports failures; explicit mode prefixes, standalone Regex, `###`, `&&`, `%%`, Analyzer-backed Java helpers, scoped sessions, and Default indexed selectors have conformance tests. Added Legado-compatible HTTP retry on unsuccessful responses, explicit response-charset decoding, multi-class Default selectors, chainable Jsoup selections, JavaScript-returned URL-option parsing, declaration scoping for pooled runtimes, redirect-preserving `java.get().header()`, source-scoped fingerprint jars, staged-response final-URL tracking, segmented URL `@js`, POST-body page selectors, `<js>...</js>` wrapper execution, typed JavaScript TOC objects, context cancellation, and stable API diagnostics for upstream crawl, storage, and not-found failures. Full Go tests pass. Fresh raw-compilation Playwright verified `八叉书库` after regular transport integration: search result, book detail, 1-chapter TOC, and 352 rendered readable paragraphs. Fresh raw `趣书网吧` verification passes search → add → 438-page TOC → first-page content with 135 rendered paragraphs. A second fresh post-review E2E pass reproduced the same 438-page TOC and 135 rendered paragraphs. Direct book deep links now load the same 438-chapter detail page without frontend effect-loop errors. Raw `中文看书（优）` independently returned 15 exact POST results; the fixed engine also produced a live result before that upstream began timing out again. Raw `神话之后（优+）` produced 2 results and a 2271-chapter TOC through the UI. The production-mode conformance runner now records raw indices 1, 84, and 89 with request/response/classification output. It also supports a bounded detail → TOC → first/middle/last-chapter workflow; raw index 788 completed the earlier first-chapter workflow with an 828-chapter TOC, raw index 779 completed a later-stage WebView workflow with browser-rendered content, and raw index 778 passed the expanded live matrix with 179 chapters plus non-empty Chinese content at indices 0, 89, and 178. A deterministic source-779 replay now covers its real rule shapes, single-quoted WebView options, HTTP-to-browser cookies, and paginated browser content without network access. Capacity defaults now target a 2-vCPU/4-GB container and remain environment-configurable for the likely 4-vCPU/8-GB deployment.

**In progress:** Shared compatibility phases 1–3 and live-discovered generic follow-ons are complete. The focused bridge supports exact `toNumChapter`, non-blocking `toast`, URL-option AJAX, JSON-list newline extraction, `@@` forced Default mode, and reusable leading-JavaScript transformations across element fallbacks. JSoup selections now expose array-like numeric enumeration with non-enumerable helper methods while preserving `__html` for downstream analysis. Targeted live reruns produced valid distinct books for raw 76, 82, 204, 209, 342, 410, 429, 471, 516, 614, 816, 897, and 920 across attempts. Raw 742 parses without error but remains empty because its raw `bookUrlPattern` requires obsolete `qudushu.com` while live links redirect to `qudushu.la`; no engine/source patch was added. Full backend and analyzer/book/sourceexec race suites pass. Authenticated Pixiv/browser/cache/login work for raw 134 remains explicitly deferred. Unrelated manual-testing changes in `frontend/dist` and `frontend/package-lock.json` remain untouched.

**Next action:** Commit the live-discovered generic semantics, preserve targeted rerun evidence, then run both fixed audit manifests and full frontend/build/E2E/Playwright verification.

**Environment notes:** `reference/legado` is the local upstream reference. `test_booksource4.json` is raw test input and must be sampled by stable URL/index identity, never source name alone. Existing server processes must be stopped before live E2E tests.

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Compatibility boundary | Documented Legado behavior first | Prevents source-specific patches and accidental semantic drift. |
| Request architecture | One SourceExecutor for every workflow | Search-only URL support caused confirmed failures. |
| Transport | HTTP and WebView behind one interface | Enables plug-in WebView without rewriting book workflows. |
| Browser runtime | Python Patchright worker over localhost HTTP | Keeps Chromium/Patchright lifecycle out of the Go process while remaining headless-Docker compatible. |
| Browser protocol | Versioned JSON `/execute` and `/healthz` endpoints | Small explicit contract; easy fake transport and independent worker upgrades. |
| Capacity policy | Bounded browser wait, TTL/LRU sessions, shared stateless HTTP pools | Protects memory and remote sites while preserving staged source continuity. |
| Batch search state | Stateless cursor/revision; cumulative browser state | Avoids backend TTL/session and multi-instance coupling; source-list changes restart safely. |
| Container registry | Two public lowercase GHCR images, `linux/amd64` | Matches verified Chromium support; owner marks package visibility public after first publish. |
| Compose deployment | One file with app default and optional private WebView profile | Supports both GHCR pulls and local builds without duplicate deployment contracts. |
| Search controls | 50 sources per batch; Balanced concurrency 8 | Coverage and parallelism remain separate, persistent, and server-bounded. |
| Source identity | `bookSourceUrl`, not name | Names are duplicated in compilations. |
| Source-switch progress | One canonical position migrated on every switch | Source imports change or disappear; independent source histories become stale and add confusing semantics. |
| Bookmark content | Location plus optional note, managed in a Reader panel | Keeps capture at the reading context while allowing lightweight annotation. |
| Bookmark source switching | Exact normalized-title migration or explicit orphan | Approximate index fallback could silently point a saved annotation at the wrong passage. |
| Offline chapter cache | Automatic network-first processed copies; LRU 100/book and 500 global | Keeps content fresh while providing bounded upstream-outage fallback without a batch-download workflow. |
| State | SourceSession with explicit scope | Cookies and variables must persist within a source flow but never leak across users/sources. |
| Rule values | Typed intermediates | Re-parsing HTML strings loses Legado element/JSON behavior. |
| Error handling | Structured, no silent empty fallback | Empty output currently hides transport and parser defects. |
| Unknown fields | Lossless preservation | Import/export must not destroy future Legado data. |
| Frontend boundary | Domain/API contracts only | Future reader/explore/debug features remain independent of scraping syntax. |

## Deferred Work

| Item | Reason | Revisit |
|---|---|---|
| Captcha/WAF bypass | Security, legal, and site-policy boundary | Never as automatic bypass; support WebView where permitted. |
| Login UI automation | Requires credential/captcha UX | After WebView and explicit user sessions. |
| Audio/image source workflows | Separate domain models | After text compatibility milestone. |
| Advanced Android-only Java APIs | Not part of the portable backend contract | Add only when a real source and safe equivalent require it. |
| Backend-persisted search sessions | Stateless cursor and tab-scoped browser state cover current needs | Revisit only if searches must roam across devices or backend instances. |

## Synchronization rules

- Update `Current State` in the same change as implementation.
- Add an append-only `Issues & Fixes` entry for every resolved compatibility bug.
- Do not mark a phase complete from a live-site sample alone; the phase gate and deterministic tests must pass.
- After each major redesign, run a Playwright CLI E2E verification when the server and source environment are available: search a real book, add it to the shelf, open the detail page, load the TOC, and read content. Record the exact source/query, observed request/result, and any environment limitation in PLAN.md.
- If live E2E is unavailable, run the deterministic conformance suite and explicitly record why Playwright could not run; do not describe the redesign as live-verified.
- Any architecture deviation requires updating the Architecture and Decisions sections before code changes.
- Every commit that changes behavior includes the relevant PLAN update.
- Review raw source identity and exact request before changing a source-specific rule.

## Issues & Fixes

### [2026-07-03] Search fetcher had shared cookie jar — cross-user leak
- **Problem**: `searchFetcher` used `NewWithTimeout` which creates a cookie jar. Cookies accumulated from one user's search could leak to another.
- **Fix**: Created `NewStateless(timeout)` constructor with `Jar: nil`. Search uses stateless client. Content/intro/TOC fetching still uses jar (correct — per-host cookies benefit both users).
- **Affected**: `backend/internal/fetcher/fetcher.go`, `backend/internal/book/search.go`
- **Watch out**: Content fetching (GetChapterContent, GetBookInfo) still shares cookies. Correct for now but revisit if per-user source logins are added.

### [2026-07-03] JSVM per-eval re-init wasted CPU + blocked pool
- **Problem**: `Eval` re-ran `initCode` on every call via `RunString`. With 4 runtimes and 50 goroutines, each eval held its runtime longer than needed, inflating contention.
- **Fix**: Removed per-eval `RunString`. `initCode` is loaded once by `LoadLib` into each runtime when it enters the pool. Per-eval only sets `result`/`baseUrl`/`java` bindings.
- **Affected**: `backend/internal/analyzer/js.go`
- **Watch out**: `LoadLib` must be called before any eval. Currently called once at startup.

### [2026-07-03] CacheManager was unbounded
- **Problem**: `CacheManager` used a plain `map[string]string` with no size limit. Memory grows without bound under multi-user load.
- **Fix**: Rewrote with `container/list` LRU eviction, `maxEntries = 4096`.
- **Affected**: `backend/internal/analyzer/cache.go`
- **Watch out**: Eviction is LRU by access order. Tunable via `maxEntries` const.

### [2026-07-03] EventSource auto-reconnect restarted search on transient errors
- **Problem**: EventSource's default behavior on any network error is to reconnect to the same URL, restarting the entire 256-source fan-out.
- **Fix**: Track `finished` flag. On `onerror`, close EventSource if not finished. Trade auto-reconnect for clean stop.
- **Affected**: `frontend/src/api/client.ts`
- **Watch out**: User must manually re-submit search after error. Acceptable for a one-shot search.

### [2026-07-03] Live search counter only counted errors
- **Problem**: `sourcesDone` was only incremented in `onError` callback, not in `onResult`. Status always showed "from 0 sources" during search.
- **Fix**: Increment `sourcesDone` in both callbacks.
- **Affected**: `frontend/src/lib/SearchPage.svelte`

### [2026-07-03] EventSource not closed on route-away
- **Problem**: Navigating away mid-search left EventSource open. Backend kept searching 30s doing wasted work.
- **Fix**: Added `$effect` cleanup that closes EventSource on component unmount.
- **Affected**: `frontend/src/lib/SearchPage.svelte`

### [2026-07-02] XSS via `{@html}` rendering unsanitized source HTML
- **Problem**: Reader used `{@html p}` to render paragraphs. Source HTML (from `@html` attr rules) was injected directly into DOM — `<img src=x onerror=…>` or `<script>` could execute.
- **Fix**: Processor strips all HTML tags, converts `<br>`/`</p>` to newlines, unescapes entities. Reader uses `{p}` (plain text).
- **Affected**: `backend/internal/processor/processor.go`, `frontend/src/lib/Reader.svelte`

### [2026-07-02] Chapter URLs resolved against wrong base
- **Problem**: Chapter URLs were resolved against source root URL instead of the page they were found on. A relative href `456.html` on `https://site.com/book/123/` would resolve to `https://site.com/456.html` instead of `https://site.com/book/123/456.html`.
- **Fix**: `parsePage` now resolves chapter URLs against the TOC page URL (where the selector found them). Added `resolveURL` helper.
- **Affected**: `backend/internal/book/chapterlist.go`

### [2026-07-02] `total_chapter_num` never populated → progress bar dead
- **Problem**: After fetching chapters, `books.total_chapter_num` was never updated. Progress bar always showed "Ch.1 / ?".
- **Fix**: Added `UpdateTotalChapters()` called after TOC fetch. Bookshelf progress bar now renders.
- **Affected**: `backend/internal/book/store.go`, `backend/internal/api/server.go`

### [2026-07-02] Volume entries became broken chapters
- **Problem**: Chapters with empty URL were stored as regular chapters. Opening one resolved the empty URL against source base → garbage URL → 500.
- **Fix**: Empty URL now infers `isVolume=true`. Volume chapters are stored but won't be fetched for content.
- **Affected**: `backend/internal/book/chapterlist.go`

### [2026-07-02] Source routes broken — URLs in path segments
- **Problem**: DELETE/PUT routes used path segments `{url}`. URLs contain slashes — impossible to match.
- **Fix**: Changed to query params. `DELETE /api/sources?url=...`, `PUT /api/sources?url=...`.
- **Affected**: `backend/internal/api/server.go`, `frontend/src/api/client.ts`

### [2026-07-02] Partial Chinese conversion removed
- **Problem**: 50-character s2t/t2s map produced mixed-script garbled text.
- **Fix**: Removed maps, `convertChinese` is a no-op. Awaiting opencc.
- **Affected**: `backend/internal/processor/processor.go`

### [2026-07-03] TOC fetch ignored source headers (BLOCKER)
- **Problem**: `GetChapterList` passed `nil` for headers. Any source needing `Cookie`, `Referer`, or custom `User-Agent` to serve the TOC page got empty results.
- **Fix**: Now passes `parseHeaderJSON(src.Header)` instead of `nil`.
- **Affected**: `backend/internal/book/search.go` (line ~376)
- **Watch out**: Same fix applied to pagination fetches within ChapterListParser (both go through the same closure).

### [2026-07-03] JSLib never loaded into JSVM (MAJOR)
- **Problem**: `src.JSLib` was stored in DB but never evaluated. Sources relying on JSLib-defined helper functions got undefined-reference errors on every rule eval.
- **Fix**: Analyzer now prepends `src.JSLib` to every `jsEval`/`jsEvalList`/`jsEvalElements` call via `SetJSLib()`.
- **Affected**: `backend/internal/analyzer/analyzer.go`, `backend/internal/book/search.go`, `backend/internal/book/chapterlist.go`
- **Watch out**: URL template eval (`@js:` in BuildURL) doesn't get JSLib prepended — only rule eval paths through Analyzer do. Fix if @js: URL sources also use JSLib.

### [2026-07-03] Search used shared cookie jar — cross-source contamination
- **Problem**: `searchFetcher` used `NewInsecure()` which creates a cookie jar. With 50 concurrent source searches, cookies set by one source's redirect domain leaked into another's request.
- **Fix**: Added `NewInsecureStateless()` constructor with `Jar: nil`. Search uses it.
- **Affected**: `backend/internal/fetcher/fetcher.go`, `backend/internal/book/search.go`
- **Watch out**: Content fetcher (`s.fetcher`) still has a cookie jar. Correct for per-host cookie reuse during chapter reading.

### [2026-07-03] Search included audio/image sources (MAJOR)
- **Problem**: `searchCandidates` only checked `SearchURL != "" && RuleSearch != ""`, letting `BookSourceType=1` (audio) and `2` (image) sources through. These produce garbled results when searched with text parsing.
- **Fix**: Added `src.BookSourceType == 0` filter.
- **Affected**: `backend/internal/book/search.go`
- **Watch out**: 30 removed sources from search. None were producing valid text results anyway.

### [2026-07-03] bookUrlPattern stored but never validated
- **Problem**: `src.BookURLPattern` was persisted but never used. Search results with over-matching `bookList` selectors (ad/promo links) were accepted as valid books.
- **Fix**: Pre-compile pattern regex before parsing loop; skip results where `bookUrl` doesn't match.
- **Affected**: `backend/internal/book/search.go`
- **Watch out**: Invalid regex patterns are silently ignored (not all sources have valid patterns).

### [2026-07-03] Divergent defaults between NewFromJSON and ImportSources
- **Problem**: `NewFromJSON` set `CreatedAt`/`UpdatedAt`/`LastUpdateTime` defaults that were already handled by `UnmarshalJSON` and the store's `Upsert`/`ImportBatch`. Redundant addition+removal for single-source imports.
- **Fix**: Removed redundant defaults from `NewFromJSON`. All path go through `UnmarshalJSON` for field defaults, then `Upsert`/`ImportBatch` for timestamp initialization.
- **Affected**: `backend/internal/booksource/entity.go`

### [2026-07-03] Intentionally unmapped legado fields undocumented
- **Problem**: `customButton` (30%), `eventListener` (30%), `enabledReview` (1%), `phonehttp` (0.1%), `userid` (0%) were silently dropped on import. No documentation explaining why.
- **Fix**: Added ponytail comment on `BookSource` struct listing intentionally-omitted fields and why.
- **Affected**: `backend/internal/booksource/entity.go`
- **Watch out**: These fields are LOST on re-export. Acceptable — they're legado-reader UI features, not fetch logic.

### [2026-07-03] concurrentRate not enforced — documented
- **Problem**: `src.ConcurrentRate` exists in DB but no code enforces per-source rate limiting.
- **Fix**: Added ponytail comment on `searchCandidates` describing the ceiling and upgrade path.
- **Affected**: `backend/internal/book/search.go`

### [2026-07-03] URL option `js` parameter discarded (MAJOR)
- **Problem**: `{"js":"java.url=java.url+'yyyy'"}` was parsed but thrown away. Sources using per-request URL rewriting or header injection via the URL option `js` hook got no effect.
- **Fix**: After full URL construction, eval `opt.Js` with bindings `java.url` (settable), `java.headerMap` (map, mutated in place), `key`, `page`, `baseUrl`. The eval result replaces the URL.
- **Affected**: `backend/internal/analyzer/urlbuilder.go`
- **Watch out**: The `java.url` binding is a string, not a mutable ref — JS must return the new URL string.

### [2026-07-03] Request charset encoding ignored for POST body (MAJOR)
- **Problem**: Sources like `全本同人` with `charset: "gb2312"` in URL options had their POST body sent as raw UTF-8. The server expected gb2312-encoded form data.
- **Fix**: Added `EncodeParamValue` + `encodeWithCharset` using `golang.org/x/text/encoding`. POST body key=value pairs are re-encoded in the source's charset before sending. Added `encodeBody()` in search.go.
- **Affected**: `backend/internal/analyzer/urlbuilder.go`, `backend/internal/book/search.go`
- **Watch out**: Only handles gbk/gb2312 and big5 charsets. Other charsets fall back to UTF-8 encoding.

### [2026-07-03] Missing JS bindings: `src`, `book`, `chapter` (MAJOR)
- **Problem**: Legado's JS context has `src` (source content alias), `book` (object with name, author, bookUrl, etc.), and `chapter` (object with url, title, index, etc.). None were bound, so rules using `{{book.name}}` or `{{chapter.title+chapter.index}}` produced undefined.
- **Fix**: Added `src` as an alias for `result` in JSVM.Eval. Added `book` and `chapter` objects as optional extra bindings via `Analyzer.SetBookData()`/`SetChapterData()`. Threaded through `GetBookInfo` and `GetChapterContent`.
- **Affected**: `backend/internal/analyzer/analyzer.go`, `backend/internal/analyzer/js.go`, `backend/internal/book/search.go`
- **Watch out**: Chapter list parser still doesn't bind per-chapter data — most chapter-list JS doesn't reference `chapter`.

### [2026-07-03] URL option `type` and `origin` fields missing
- **Problem**: Legado's `UrlOption` has `type` and `origin` fields. These were not in our `urlOption` struct, silently dropped on import.
- **Fix**: Added `Type` and `Origin` fields to `urlOption`. Added `Type` to `URLMeta`.
- **Affected**: `backend/internal/analyzer/urlbuilder.go`
- **Watch out**: `serverID` and `webViewDelayTime` are still omitted (no WebView backend).

### [2026-07-03] Per-source concurrentRate now enforced
- **Problem**: `src.ConcurrentRate` was stored but never used. Legado uses it for per-source rate limiting.
- **Fix**: Added `rateLimitWait()` in search.go. Parses `concurrentRate` as milliseconds between requests. Uses a mutex-protected `lastAccess` map keyed by `BookSourceURL`. Sources without a rate use system default (no limit).
- **Affected**: `backend/internal/book/search.go`
- **Watch out**: Simple time-since-last-access throttle, not a token bucket. Fine for the 11 sources (1%) that set a rate.

### [2026-07-03] Default-rule parser missing — all unprefixed rules silently failed (BLOCKER)
- **Problem**: Legado's primary rule format (`class.odd.0@tag.a.0@text`) was routed to goquery as CSS, producing no matches. Every bare Default-rule source returned empty results silently.
- **Fix**: Implemented `ModeDefault` parser in `modes_default.go`. Detects Default format by prefix heuristics (`class.`, `id.`, `tag.`, numeric indices, multiple `@`). Supports position indices (positive, negative, exclusion), `@text`/`@href`/`@src`/`@html` getters, CSS fallback for combinators.
- **Affected**: `backend/internal/analyzer/modes_default.go`, `backend/internal/analyzer/ruleparser.go`, `backend/internal/analyzer/analyzer.go`
- **Watch out**: Array index syntax `[0:10:2]`, `[0,2,4]` not yet supported. Most sources use simple `.N` indices which work.

### [2026-07-03] Duplicate `alternate_sources` column crashed server on every query (BLOCKER)
- **Problem**: `Init()` created `alternate_sources` in `CREATE TABLE`, then immediately ran `ALTER TABLE books ADD COLUMN alternate_sources` — duplicating the column at position 19. `SELECT *` returned two `alternate_sources` columns, shifting `created_at` (int64) onto the TEXT duplicate → type conversion panic → server crash.
- **Fix**: Removed redundant ALTER TABLE. Changed all book queries to use explicit column list (`bookColumns`) instead of `SELECT *` for deterministic scan order.
- **Affected**: `backend/internal/book/store.go`
- **Watch out**: Old databases with the duplicate column must be deleted (`rm backend/data/novelreader.db`).

### [2026-07-03] `findJSONOption` missed `,\n{` patterns (7 sources) (BLOCKER)
- **Problem**: Scanned for adjacent `,{` only. Newlines between comma and brace (from multi-line URL formatting) made `findJSONOption` miss the option entirely. Also used `json.Valid` which rejected trailing junk (e.g. `@js:` after the option).
- **Fix**: Skip whitespace between comma and brace; brace-count to extract just the JSON object; `json.Valid` on the extracted portion only.
- **Affected**: `backend/internal/analyzer/urlbuilder.go`

### [2026-07-03] `&&` rule connector unhandled (376 sources) (MAJOR)
- **Problem**: Only `||` was treated as a rule chain separator. `&&` (merge/concatenate results) was passed verbatim as part of the CSS/Default selector, failing to match anything.
- **Fix**: Added `splitTopLevel()` helper that handles `&&`, `||`, `%%` at top level while respecting `<js>`/`{{}}` depth. `GetString` and `GetElements` now split on `&&` and concatenate results.
- **Affected**: `backend/internal/analyzer/analyzer.go`
- **Watch out**: `%%` zip connector not yet implemented (2 sources only).

### [2026-07-03] Mid-chain `<js>` tags never executed (150 segments) (MAJOR)
- **Problem**: Rules like `$.bid<js>java.put('bid',result);'http://...'</js>` were parsed as a single segment with mode detection at start. The `<js>` block inside was passed to the CSS/Default parser and silently ignored.
- **Fix**: `nextSegment` now treats `<js>` at depth 0 as a segment boundary, splitting the rule chain into separate CSS/Default and JS rule segments.
- **Affected**: `backend/internal/analyzer/ruleparser.go`

### [2026-07-03] `java.connect()` response chain incomplete (MAJOR)
- **Problem**: Only `raw.request.url()` chain was implemented. Legado also uses `.body()`, `.code()`, `.headers()` on the Connect response. Failed requests returned a map without `raw` at all → `Object has no member 'raw'`.
- **Fix**: Always return `body`, `code`, `headers`, and `raw` (with nested `request.url`, `request.headers`). Even on error, the chain returns available data instead of erroring.
- **Affected**: `backend/internal/analyzer/js.go`

### [2026-07-03] `<,{{page}}>` page-selection syntax unsupported (11 search sources) (MINOR)
- **Problem**: `<,/page/{{page}}>` on page 1 should produce empty string. Was passed literally as part of the URL.
- **Fix**: Regex-based detection and replacement: for page=1, `<...>` segments produce empty string; for page>1, the page-indexed element is selected.
- **Affected**: `backend/internal/analyzer/urlbuilder.go`

### [2026-07-03] `@js:` eval failure leaked raw JS to HTTP client (MINOR)
- **Problem**: When `@js:` JS evaluation failed, `urlStr` stayed as the raw `@js:...` string. The fetcher then tried to parse `@js:
var su=...` as a URL → `net/url: invalid control character`.
- **Fix**: Return error from `BuildURL` when `@js:` eval fails. The source is properly skipped instead of sending garbage to the fetcher.
- **Affected**: `backend/internal/analyzer/urlbuilder.go`

### [2026-07-03] Newlines in URL templates broke parsing (MINOR)
- **Problem**: Many sources have literal `\n` in `searchUrl` for multi-line formatting. `url.Parse` rejects URLs with control characters.
- **Fix**: Strip `\n`/`\r` from non-`@js:` URL templates before parsing (remove, not space-replace).
- **Affected**: `backend/internal/analyzer/urlbuilder.go`
- **Watch out**: `@js:` URLs are NOT stripped (they need newlines for JS code).

### [2026-07-03] `{{...}}` regex failed on nested braces (2 sources) (MINOR)
- **Problem**: Template regex `{{([^}]+)}}` couldn't match `}` inside expressions like `{{var x={a:1}; x}}`.
- **Fix**: Replaced regex with brace-counting scanner that correctly handles nested braces.
- **Affected**: `backend/internal/analyzer/urlbuilder.go`


### [2026-07-12] Legado compatibility audit exposed incomplete core pipeline
- **Problem**: URL options were handled only for search; detail, TOC, content, JS bridge, cookies, state, and several rule semantics diverged from Legado.
- **Fix**: Planned a unified SourceExecutor/SourceSession/transport architecture and conformance-first implementation sequence.
- **Affected**: `backend/internal/analyzer`, `backend/internal/book`, `backend/internal/fetcher`, `backend/internal/booksource`, future `sourceexec` and `webview` modules.
- **Watch out**: Do not patch individual sources before unified request execution and deterministic conformance tests exist.

### [2026-07-12] URL option metadata and relative resolution were incomplete
- **Problem**: URL options for `webJs`, `bodyJs`, `dnsIp`, and `origin` were discarded; POST body JavaScript lacked `key/page` bindings; root-relative URLs were joined against the base path instead of the host.
- **Fix**: Added metadata to `URLMeta`, passed template bindings into body evaluation, and switched relative resolution to `net/url.ResolveReference`; added conformance tests.
- **Affected**: `backend/internal/analyzer/urlbuilder.go`, `backend/internal/analyzer/urlbuilder_conformance_test.go`.
- **Watch out**: These metadata fields are preserved but not yet executed by the unified SourceExecutor; do not mark URL execution complete until all workflows use them.

### [2026-07-12] Transport contract was absent
- **Problem**: Search, detail, TOC, and content had no shared request/response interface, making it impossible to add WebView or consistent diagnostics without changing each workflow independently.
- **Fix**: Added transport-neutral `RequestSpec`, `Response`, and `Transport` contracts plus a metadata-preserving adapter from `analyzer.URLMeta`.
- **Affected**: `backend/internal/sourceexec/request.go`, `backend/internal/sourceexec/request_test.go`.
- **Watch out**: Production workflows still use the old fetcher directly; the next step is an HTTP adapter with conformance tests before wiring callers.

### [2026-07-12] HTTP transport adapter added
- **Problem**: The new request contract had no executable transport, so status/body preservation and request metadata could not be verified at the transport boundary.
- **Fix**: Added `HTTPTransport` with GET/POST execution, header/origin forwarding, context cancellation, retry forwarding, final URL capture, and non-2xx body retention; added an `httptest` conformance fixture.
- **Affected**: `backend/internal/sourceexec/http_transport.go`, `backend/internal/sourceexec/http_transport_test.go`.
- **Watch out**: Charset encoding, status retry policy, cookies/session state, and production workflow wiring remain intentionally incomplete.

### [2026-07-12] Unified executor boundary added
- **Problem**: URL expansion and transport invocation still had no single orchestration point, so future HTTP/WebView selection could be duplicated across workflows.
- **Fix**: Added `sourceexec.Executor` with explicit JSVM and Transport dependencies, `Build`, and `Execute`; added a fixture proving method/body/header propagation.
- **Affected**: `backend/internal/sourceexec/executor.go`, `backend/internal/sourceexec/executor_test.go`.
- **Watch out**: Existing search/detail/TOC/content paths still bypass the executor until session and URL-option conformance tests are complete.

### [2026-07-12] Source session state was missing
- **Problem**: Legado cookie, source-variable, and memory state had no isolated backend owner; the JS bridge used no-op cookies and ephemeral source data.
- **Fix**: Added `SourceSession` with isolated cookie jar, cookie header/value helpers, persistent variables, and request-flow memory, plus isolation tests.
- **Affected**: `backend/internal/sourceexec/session.go`, `backend/internal/sourceexec/session_test.go`.
- **Watch out**: The session is not yet wired into JSVM; transport/client ownership must remain per session to prevent cookie leakage.

### [2026-07-12] HTTP transport did not share source session cookies
- **Problem**: A session could hold cookies, but the HTTP transport had no synchronization boundary, so server-set cookies could not reliably reach JS/source state or subsequent workflow requests.
- **Fix**: Added a session-aware HTTP transport constructor and synchronized cookies before and after each request; added a two-request `httptest` conformance flow.
- **Affected**: `backend/internal/sourceexec/http_transport.go`, `backend/internal/sourceexec/session.go`, `backend/internal/sourceexec/session_transport_test.go`.
- **Watch out**: A session-aware transport owns a dedicated fetcher client; sharing that client across sessions would reintroduce cookie leakage.

### [2026-07-12] JavaScript bindings ignored source session state
- **Problem**: The JSVM always exposed no-op cookie functions and VM-global source/cache state, so Legado JS could not share cookies, variables, or memory with HTTP requests.
- **Fix**: Added analyzer-level `SourceState`, session-backed `source`, `cookie`, and `cache` objects, and a conformance test for cookie lookup and state writes.
- **Affected**: `backend/internal/analyzer/js.go`, `backend/internal/analyzer/js_session_test.go`, `backend/internal/sourceexec/session.go`.
- **Watch out**: Analyzer and URL-template callers still need to pass `SourceState`; until then only direct JS evaluations using the binding are session-aware.

### [2026-07-12] URL-template JavaScript bypassed source session state
- **Problem**: `{{cookie...}}`, `{{source...}}`, and URL-option JavaScript were evaluated without the active source session, so request construction could not use cookies or persistent variables.
- **Fix**: Added `BuildURLWithState` and `NewExecutorWithSession`; passed `SourceState` through URL templates and option-JS bindings with a regression test.
- **Affected**: `backend/internal/analyzer/urlbuilder.go`, `backend/internal/sourceexec/executor.go`, `backend/internal/sourceexec/executor_session_test.go`.
- **Watch out**: Existing book workflows still construct URLs outside the executor; wiring them is required before real sources benefit.

### [2026-07-12] Analyzer session propagation was incomplete
- **Problem**: Direct JS bindings supported sessions, but ordinary Analyzer rule evaluation did not pass the source session into the JSVM.
- **Fix**: Added `Analyzer.SetSourceState`, included the state in all JS binding maps, and added a rule-evaluation regression test.
- **Affected**: `backend/internal/analyzer/analyzer.go`, `backend/internal/analyzer/analyzer_session_test.go`.
- **Watch out**: The book workflows still need to create and pass one session through request, analyzer, and pagination lifecycles.

### [2026-07-13] Search bypassed the unified executor
- **Problem**: Search expanded URLs and fetched them through a separate shared stateless client, so URL/session JavaScript behavior could not match Legado and source cookies could not persist across search stages.
- **Fix**: Search now builds and executes requests through `sourceexec`, uses one isolated session/client per source, merges source and URL headers, and passes the session into result-rule analysis; added a real `httptest` search fixture.
- **Affected**: `backend/internal/book/search.go`, `backend/internal/book/search_executor_test.go`.
- **Watch out**: `SetSearchFetcher` remains a legacy injection seam and is not yet integrated with the per-source transport factory; resolve this before relying on custom search clients in tests or deployments.

### [2026-07-13] Book-info bypassed the unified executor
- **Problem**: Detail-page URLs containing Legado POST/body/options were fetched as literal GET URLs, so valid book sources returned empty or incorrect metadata.
- **Fix**: `GetBookInfo` now builds and executes the detail request through an isolated session-aware executor, merges source headers, applies charset body encoding, and passes the session to Analyzer; added a POST detail integration fixture.
- **Affected**: `backend/internal/book/search.go`, `backend/internal/book/bookinfo_executor_test.go`.
- **Watch out**: Detail sessions are not yet persisted into TOC/content workflows; the next session-lifecycle design must preserve cookies without cross-user leakage.

### [2026-07-13] TOC bypassed URL execution and reversed documented order
- **Problem**: TOC fetched URLs as literal GET requests, ignored POST/body/options, did not share session state with rule evaluation, and reversed lists unless `-` was present, contrary to the documented source rule contract.
- **Fix**: `GetChapterList` now uses a session-aware executor for initial and paginated pages; parser analyzers receive the session; relative next-page URLs are normalized before cycle detection; reversal occurs only for a leading `-`; added a POST/order integration fixture.
- **Affected**: `backend/internal/book/search.go`, `backend/internal/book/chapterlist.go`, `backend/internal/book/toc_executor_test.go`.
- **Watch out**: Auto-detected TOC links and content requests still use legacy fetch paths; pagination retry/partial-failure semantics remain to be tested.

### [2026-07-13] Explicit analyzer mode prefixes were not stripped
- **Problem**: `@css:`, `@xpath:`, `@json:`, and `@js:` rules were classified but their prefixes were passed into the underlying parser, causing valid explicit rules to return empty results or JS errors.
- **Fix**: Added shared mode-prefix normalization before CSS/XPath/JSON/JS dispatch; the TOC integration fixture now exercises explicit CSS.
- **Affected**: `backend/internal/analyzer/analyzer.go`.
- **Watch out**: Mode detection and Default/Regex/connector semantics still need dedicated conformance coverage.

### [2026-07-13] Content bypassed chapter URL execution
- **Problem**: Chapter URLs containing Legado POST/body/options were fetched as literal GET URLs, so valid sources returned empty content despite correct rules.
- **Fix**: `GetChapterContent` now builds and executes chapter requests through an isolated session-aware executor, applies source headers/charset, binds the final URL to Analyzer, and retains SPA JSON fallback; added a POST content integration fixture.
- **Affected**: `backend/internal/book/search.go`, `backend/internal/book/content_executor_test.go`.
- **Watch out**: `nextContentUrl` aggregation and detail→TOC→content session continuity remain incomplete.

### [2026-07-13] Content pagination was silently omitted
- **Problem**: `nextContentUrl` was only exposed by a getter and never followed, so multi-page chapters returned only the first page.
- **Fix**: `GetChapterContent` now evaluates and follows next-content URLs through the same executor/session, concatenates page content, binds each final URL, and stops on empty/repeated URLs; added a two-page integration fixture.
- **Affected**: `backend/internal/book/search.go`, `backend/internal/book/content_pagination_test.go`.
- **Watch out**: Multi-page partial failures currently return an error; retry/status policy and URL-array semantics need explicit conformance tests.

### [2026-07-13] TOC pagination silently returned partial success
- **Problem**: A failed `nextTocUrl` request broke the loop and returned collected chapters as if the TOC were complete.
- **Fix**: Pagination now returns a contextual `toc: next page` error; added success and failure two-page fixtures.
- **Affected**: `backend/internal/book/chapterlist.go`, `backend/internal/book/toc_pagination_test.go`.
- **Watch out**: Decide and document whether future source policy permits partial TOCs; default behavior remains fail-loudly.

### [2026-07-13] Rule engine omitted standalone Regex and list connectors
- **Problem**: Leading `##` rules were classified as CSS and `GetStringList` ignored documented `&&`/`%%` semantics, causing valid multi-rule extraction to return empty or incorrectly ordered results.
- **Fix**: Added Regex mode detection, `###` first-match marker handling, and deterministic `&&` concatenation/`%%` interleaving with conformance tests.
- **Affected**: `backend/internal/analyzer/ruleparser.go`, `backend/internal/analyzer/modes_regex.go`, `backend/internal/analyzer/analyzer.go`, analyzer conformance tests.
- **Watch out**: Default indices/ranges, Regex edge cases, element connector semantics, and JS helper parity remain incomplete.

### [2026-07-13] Java rule helpers were stubs
- **Problem**: `java.getString`, `java.getElements`, and `java.setContent` returned empty/no-op values, preventing JavaScript sources from re-entering the rule engine or switching content.
- **Fix**: Bound the active Analyzer into JS evaluation, implemented helper delegation and mutable content, and added an end-to-end JS helper conformance test.
- **Affected**: `backend/internal/analyzer/js.go`, `backend/internal/analyzer/analyzer.go`, `backend/internal/analyzer/js_helpers_conformance_test.go`.
- **Watch out**: Java helper return shapes and HTTP methods still need broader Legado fixture coverage.

### [2026-07-13] Live E2E verification was not a documented redesign gate
- **Problem**: Major backend redesigns were validated by Go tests but the required real-user browser path was not consistently rerun after each redesign.
- **Fix**: Added a synchronization rule requiring Playwright CLI verification after each major redesign when the server/source environment is available, with explicit recording of limitations when it is not.
- **Affected**: `PLAN.md`.
- **Watch out**: A live browser pass cannot replace deterministic conformance tests or prove all sources compatible.

### [2026-07-13] Playwright gate found a live-source content timeout
- **Problem**: Fresh-server E2E reached search, shelf, detail, and TOC, but opening the selected chapter returned HTTP 500 because `www.shenhuazhihou.com` timed out during content fetching; image hosts also produced unrelated browser resource errors.
- **Fix**: Recorded the run as incomplete rather than claiming full-pipeline success; stopped the temporary 8890 server/browser and retained the deterministic test pass.
- **Affected**: Live verification environment; `/tmp/novelreader_e2e.log`; no source-specific code changed.
- **Watch out**: Repeat the gate with a confirmed multi-chapter, reachable source before marking the redesign live-verified; investigate timeout/retry policy separately from parser correctness.

### [2026-07-13] Detail, TOC, and content sessions were disconnected
- **Problem**: Each workflow created a new source session, so cookies set during book detail could not reach TOC or chapter requests.
- **Fix**: Added `SessionRegistry` with source/book scope and chapter association; wired all three workflows to reuse the session; added a cookie-required end-to-end fixture.
- **Affected**: `backend/internal/sourceexec/session_registry.go`, `backend/internal/book/search.go`, `backend/internal/book/session_continuity_test.go`.
- **Watch out**: Registry scope is currently one Searcher/server; introduce authenticated user scoping and eviction before multi-user deployment.

### [2026-07-13] Playwright rerun still hit an unreachable selected chapter source
- **Problem**: After session continuity changes, fresh E2E again reached search, shelf, detail, and TOC, but the selected `shenhuazhihou.com` chapter returned HTTP 500 after a backend timeout; no frontend rendering result could be claimed.
- **Fix**: Recorded the gate as incomplete, stopped the temporary server/browser, and kept the deterministic continuity test as the valid verification for this slice.
- **Affected**: Live verification environment; `/tmp/novelreader_e2e2.log`; no source-specific code changed.
- **Watch out**: Next live gate must deliberately select a confirmed reachable multi-chapter source rather than the first search result.

### [2026-07-13] Multi-source Playwright verification passed on reachable sources
- **Problem**: The first selected source repeatedly timed out, leaving the redesign’s live content result ambiguous.
- **Fix**: Imported `test_booksource4.json`, selected different UI result cards, and verified raw source identities: `https://www.bsxiaoshuo.com` returned 167 chapters and rendered 38 paragraphs; `http://wap.wangshugu.info` returned one chapter and rendered 46 paragraphs. Direct API inspection confirmed the expected source URLs and raw rules.
- **Affected**: Live verification only; `/tmp/novelreader_multi_source.log`; no source-specific code changed.
- **Watch out**: `望书阁网` did not extract a separate `tocUrl`, but its detail-page chapter list and content still worked; this is a source-rule coverage gap to investigate separately.

### [2026-07-13] Timed-out source was reachable but its raw content selector matched no paragraphs
- **Problem**: Direct curl and Playwright loaded `https://www.shenhuazhihou.com/book/20438/729342.html` with HTTP 200 and visible chapter text, while the backend request timed out. Independent DOM inspection found `#chaptercontent` contained text but `#chaptercontent p` matched zero nodes, whereas the raw rule is `id.chaptercontent@p@html`.
- **Fix**: Classified the prior failure as two separate investigations—HTTP transport timeout and rule/DOM mismatch—rather than calling it a frontend or parser regression; no source-specific rule was changed.
- **Affected**: Live source verification; raw source `神话之后（优+）`; no production code changed.
- **Watch out**: Improve transport retry/timeout diagnostics first, then test whether Legado’s Default selector semantics or a compatibility fallback should handle this DOM shape.

### [2026-07-13] Live 22笔趣阁 TOC exposed unsupported Default index syntax
- **Problem**: UI search selected `https://m.22biqu.net`, but NovelReader showed `Chapters (0)`. Playwright loaded the raw book page successfully and found two `.directoryArea` sections with 55 chapter links; the raw TOC rule uses `.directoryArea:eq(1)@p@a`.
- **Fix**: Recorded this as an engine/parser compatibility failure, not a dead source; no source-specific rule was changed.
- **Affected**: Live verification; raw source `22笔趣阁`; no production code changed.
- **Watch out**: Implement and test Legado Default selector indices/`:eq()` semantics before judging similar sources outdated.

### [2026-07-13] Default indexed selectors were not implemented
- **Problem**: The Default parser treated `.directoryArea:eq(1)` as an unsupported CSS pseudo-selector and `.directoryArea.1` as an invalid segment; chained child selectors consequently returned no chapters.
- **Fix**: Added `:eq(n)`/negative-eq handling, `.class.N` shorthand, correct CSS-segment no-index behavior, and captured HTML conformance tests.
- **Affected**: `backend/internal/analyzer/modes_default.go`, `backend/internal/analyzer/ruleparser.go`, `backend/internal/analyzer/default_index_conformance_test.go`.
- **Watch out**: The fix is not live-verified yet; Playwright must retest `m.22biqu.net` and another indexed source before marking this compatibility slice complete.

### [2026-07-13] Indexed TOC live gate was blocked by enrichment failure
- **Problem**: After the selector fix, the UI test selected the `m.22biqu.net` result but book enrichment returned HTTP 404, so the app showed `Chapters (0)`. Direct Playwright inspection of the raw book page showed two `.directoryArea` sections and 50 links in the indexed section; curl returned HTTP 200.
- **Fix**: Classified the UI result as a separate enrichment/transport discrepancy, not evidence that the selector fix failed; stopped the temporary server/browser and retained the passing captured-HTML test.
- **Affected**: Live verification; `/tmp/novelreader_index_e2e.log`; no source-specific rule changed.
- **Watch out**: Re-test indexed TOC through a source whose detail enrichment succeeds; investigate why the backend saw 404 while curl saw 200 before declaring live compatibility.

### [2026-07-13] Enrichment request parity investigation was not explicit in the plan
- **Problem**: The plan covered generic status/retry work but did not explicitly require comparing NovelReader’s exact enrichment request against curl/browser when statuses differ.
- **Fix**: Added request-parity diagnostics and the `m.22biqu.net` 404-vs-200 discrepancy as a Phase 1 task and current gate.
- **Affected**: `PLAN.md`.
- **Watch out**: Do not mark indexed-selector compatibility complete until the UI reaches TOC parsing successfully.

### [2026-07-13] Content pagination followed the next chapter as another page
- **Problem**: `nextContentUrl` for `m.22biqu.net` correctly returned internal page URLs, but after the last internal page it returned the next TOC chapter. NovelReader fetched subsequent chapters and eventually returned HTTP 500 instead of stopping.
- **Fix**: Added a session-registry chapter URL guard matching Legado’s `nextChapterUrl` stop condition; added a regression fixture proving the next chapter is not fetched as content pagination.
- **Affected**: `backend/internal/book/search.go`, `backend/internal/sourceexec/session_registry.go`, `backend/internal/book/content_next_chapter_test.go`.
- **Watch out**: Live Playwright must verify the first chapter now renders without traversing into later chapters.

### [2026-07-13] Next-chapter guard passed multi-source Playwright verification
- **Problem**: The guard needed live confirmation because the prior failure only appeared on a real multi-page source.
- **Fix**: Fresh UI E2E imported the raw compilation, selected `m.22biqu.net`, loaded 50 chapters, and rendered the first chapter after fetching only its internal `_2` page; a separate `bsxiaoshuo.com` source loaded 167 chapters and rendered 38 paragraphs. Only unrelated image-host console errors remained.
- **Affected**: Live verification; `/tmp/novelreader_m22_fixed.log`; no additional production code changed.
- **Watch out**: Continue testing indexed/JS/charset sources; live success on two sources does not establish global compatibility.

### [2026-07-13] Shared transport ignored Legado retry and response charset options
- **Problem**: The fetcher returned the first non-2xx response even when `retry` was configured, and response decoding always relied on auto-detection instead of the source-declared URL-option charset.
- **Fix**: Retry unsuccessful HTTP responses up to the configured count while retaining the final response body; added explicit response charset decoding through `RequestSpec` and the HTTP transport.
- **Affected**: `backend/internal/fetcher/fetcher.go`, `backend/internal/sourceexec/http_transport.go`, `backend/internal/fetcher/fetcher_retry_test.go`, `backend/internal/sourceexec/http_transport_charset_test.go`.
- **Watch out**: Live E2E still needs a raw POST/charset source; deterministic tests cover GBK response decoding and retry status transitions.

### [2026-07-13] Cross-source transport verification required a fresh search
- **Problem**: The first second-source Playwright attempt navigated directly to `#/search`, which discarded in-memory results; selecting the expected card timed out.
- **Fix**: Re-ran the full search flow from the application entry point before selecting the raw `童话雨邪`/笔尚小说 result; the source loaded and rendered 38 paragraphs.
- **Affected**: Live verification workflow; no production code change.
- **Watch out**: Every E2E sample must perform a fresh search or explicitly prove result state persists across navigation.

### [2026-07-13] Default multi-class selector failed on a live POST source
- **Problem**: Raw `露露书` returned matching search HTML for its POST UTF-8 request, but `class.ptm-list-view-cell ptm-img ptm-col-xs-4` was classified as CSS and matched no elements.
- **Fix**: Implemented Legado Default handling for space-separated class names and single-segment explicit Default selectors; added a focused conformance test.
- **Affected**: `backend/internal/analyzer/ruleparser.go`, `backend/internal/analyzer/modes_default.go`, `backend/internal/analyzer/default_multiclass_test.go`.
- **Watch out**: Continue testing class combinations with indexes, chained selectors, and CSS-prefixed equivalents.

### [2026-07-13] Multi-class fix passed raw POST/charset full-pipeline verification
- **Problem**: The parser fix needed live confirmation on the exact source that exposed it.
- **Fix**: Fresh UI E2E imported `test_booksource4.json`, searched `凡人修仙传`, selected raw source `露露书`, loaded its POST UTF-8 search result, displayed 2 TOC chapters, and rendered 1,229 paragraphs from the first chapter.
- **Affected**: Live verification; `/tmp/novelreader_lulu_fixed.log`.
- **Watch out**: This verifies one Default/POST/UTF-8 path; JavaScript and GBK paths still require independent coverage.

### [2026-07-13] JavaScript source bridge gaps found during raw verification
- **Problem**: Raw `java.ajax` sources failed because `org.jsoup` selections were arrays without chained `attr`/`select`, `@js`-returned `url,{options}` metadata was encoded into the URL path, and pooled runtimes retained top-level `let`/`const` declarations.
- **Fix**: Added chainable Jsoup selection methods, reparsed URL options returned by JavaScript, and block-scoped declaration-bearing scripts; added conformance tests for each behavior.
- **Affected**: `backend/internal/analyzer/js.go`, `backend/internal/analyzer/urlbuilder.go`, `backend/internal/analyzer/jsoup_conformance_test.go`, `backend/internal/analyzer/urlbuilder_js_option_test.go`, `backend/internal/analyzer/js_scope_test.go`.
- **Watch out**: Live sampled JS sources still need a confirmed non-empty two-step result before full TOC/content verification.

### [2026-07-13] JavaScript redirect source exposed header-dependent transport behavior
- **Problem**: Raw `八叉书库` returns HTTP 302 with `Location: result/?searchid=...` under a reduced request, but the source’s exact browser-style header set receives HTTP 400 from the Go client before the redirect. The JS bridge could not extract a search ID because no redirect response existed.
- **Fix**: Added redirect-preserving `java.get()` responses and `.header()` access, with a deterministic redirect conformance test. The remaining 400-vs-302 request parity is not classified as source failure.
- **Affected**: `backend/internal/fetcher/fetcher.go`, `backend/internal/fetcher/fetcher_redirect_test.go`, `backend/internal/analyzer/js.go`.
- **Watch out**: Add request diagnostics and a conservative header compatibility fallback before declaring this JS source unsupported.

### [2026-07-13] Fingerprint transport integration boundary selected
- **Problem**: Solving TLS/HTTP fingerprint mismatch by coupling `tls-client` directly into `book` or `sourceexec` would make the request engine difficult to test, replace, and extend to WebView.
- **Fix**: Chose an injected `fetcher.HTTPClient` contract and a separate `internal/fingerprint` adapter. The initial policy is fingerprint-first with normal HTTP fallback and configurable Chrome profile; the existing `sourceexec.Transport` contract remains stable.
- **Affected**: Architecture and Phase 1 plan; implementation pending.
- **Watch out**: Pin the dependency, test fallback/status/cookies/charset, and do not classify the raw source as outdated until the adapter is verified.

### [2026-07-13] Fingerprint adapter passed raw JavaScript full-pipeline verification
- **Problem**: The adapter needed to prove it solved the exact request mismatch without leaking `tls-client` types into analyzer or book workflows.
- **Fix**: Added `fetcher.HTTPClient`, isolated `internal/fingerprint`, fingerprint-first/normal-fallback behavior, Unicode query normalization, and `StrResponse.body()` compatibility. The adapter uses the latest pinned Chrome profile by default and remains configurable through `TLS_CLIENT_PROFILE`.
- **Affected**: `backend/internal/fetcher/fetcher.go`, `backend/internal/fingerprint/`, `backend/internal/analyzer/js.go`, `backend/cmd/server/main.go`, `backend/go.mod`, `backend/go.sum`.
- **Watch out**: This first integration covers JavaScript bridge requests; regular `sourceexec` requests still use their existing transport and need a separate injected seam.

### [2026-07-13] Fingerprint adapter live verification passed
- **Problem**: The first E2E attempt used a stale server binary and could not validate the new adapter.
- **Fix**: Restarted the actual child server with the writable Go module cache, imported the raw 939-source compilation, and reran the fresh UI flow. `八叉书库` produced a result, detail page, 1-chapter TOC, and 352 readable paragraphs.
- **Affected**: Live verification; `/tmp/novelreader_tlsclient_e2e6.log`.
- **Watch out**: Use `GOMODCACHE=/tmp/go-mod GOPATH=/tmp/go` in this environment; verify a second JS source before expanding the adapter to regular requests.

### [2026-07-13] Regular source transport required staged cookie and URL-rule compatibility
- **Problem**: The second raw JavaScript source used a URL followed by `@js:`, `<, ... >` page syntax inside a POST body, and a CSRF cookie created by the JavaScript preflight. The regular POST path initially lacked all three compatibility behaviors.
- **Fix**: Added segmented URL JavaScript evaluation, applied page selectors to expanded POST bodies, synchronized JavaScript response cookies into the source session, added default User-Agent parity, and injected fingerprint-first transport into all `sourceexec` workflows through `book.TransportFactory`.
- **Affected**: `backend/internal/analyzer/js.go`, `backend/internal/analyzer/urlbuilder.go`, `backend/internal/book/search.go`, `backend/internal/fingerprint/`, `backend/cmd/server/main.go`.
- **Watch out**: Do not share fingerprint cookie jars across sources; the factory creates source-scoped transports and the normal transport remains the fallback.

### [2026-07-13] Second JavaScript source verification was split by engine and upstream state
- **Problem**: Raw `笔趣小说` independently returned 15 HTML results for `剑来`, and NovelReader later returned 12 results after the shared fixes; subsequent full-flow retries encountered `m.bqgcn.net` DNS/503 failures and could not complete TOC verification.
- **Fix**: Confirmed the raw response and request contract with curl, diagnosed the initial failures as shared engine issues, fixed those issues, and recorded the later outage separately rather than classifying the source as outdated.
- **Affected**: Raw verification; `/tmp/bqres`; `/tmp/novelreader_bq_diag.log`; `/tmp/novelreader_bq_final.log`.
- **Watch out**: Re-run the full `剑来` search → add → TOC → content path when the endpoint returns HTTP 200; the remaining gate is upstream availability, not a proven parser defect.

### [2026-07-13] Verification-debug-fix loop formalized
- **Problem**: Source failures could be prematurely classified as outdated because implementation, deterministic tests, live E2E, and cross-source diagnosis were not always performed as one repeatable loop.
- **Fix**: Added a mandatory audit → failing test → shared implementation → deterministic suite → Playwright → layered diagnosis → second raw source → fix/retest → PLAN synchronization workflow.
- **Affected**: `PLAN.md`.
- **Watch out**: Do not mark a changed path complete while its live gate is blocked by an unresolved failure; do not call a raw rule outdated before independent reproduction.

### [2026-07-13] Fingerprint transport audit found contract gaps
- **Problem**: Review found incorrect page-selector indexing, discarded fingerprint response charsets, shared JavaScript cookie jars, lost redirect cookies, incomplete retry delegation, reversed fallback redirect behavior, uncancelable JavaScript HTTP calls, and unbounded fingerprint response reads.
- **Fix**: Corrected Legado page-selector semantics, response decoding, source-scoped fingerprint clients, redirect-cookie import, retry fallback, redirect flag handling, context-aware JS helpers, and bounded reads; added focused regression tests.
- **Affected**: `backend/internal/analyzer/`, `backend/internal/fetcher/`, `backend/internal/fingerprint/`, `backend/internal/sourceexec/`, `backend/internal/book/search.go`, `backend/cmd/server/main.go`.
- **Watch out**: Keep JavaScript fallbacks stateless; all source-scoped cookies must enter requests through `SourceSession`.

### [2026-07-13] Raw 趣书网吧 TOC exposed missing typed JS-object handling
- **Problem**: Raw source index 778 uses `<js>...</js>` to return 438 `{text, href}` objects. The evaluator passed the wrapper through to goja and the chapter parser stringified objects, producing `Chapters (0)` despite a reachable page and valid rules.
- **Fix**: Strip `<js>` wrappers, preserve object elements, apply `chapterName`/`chapterUrl` fields directly, and propagate the workflow context into analyzer JavaScript.
- **Affected**: `backend/internal/analyzer/analyzer.go`, `backend/internal/book/chapterlist.go`, `backend/internal/book/chapterlist_object_test.go`, `backend/internal/analyzer/js_toc_test.go`.
- **Watch out**: Continue testing JS-returned objects with nested fields and non-TOC rule contexts; do not regress HTML element extraction fallback.

### [2026-07-13] Raw 趣书网吧 full pipeline passed after shared fixes
- **Problem**: The first live attempt stopped at TOC with `analyzer: no elements matched`; the raw page and exact rule were independently reachable and reproducible.
- **Fix**: Restarted a fresh server with raw `test_booksource4.json`, re-ran UI search/add, loaded 438 TOC pages, opened the first chapter, and verified 135 visible content paragraphs containing actual chapter text.
- **Affected**: Live E2E; raw source index 778, `https://www.qubook.org##旅途`, `/tmp/novelreader_qubook_fixed2.log`.
- **Watch out**: Keep direct deep-link verification separate from the search-user-flow gate.

### [2026-07-13] Direct book deep link triggered a frontend effect loop
- **Problem**: Opening `#/book?id=...` directly caused Svelte `effect_update_depth_exceeded` and remained on `Loading...`, while navigation from search worked.
- **Fix**: Moved App hash-listener setup from reactive `$effect` to `onMount`, guarded BookDetail loading by book ID, and surfaced load errors instead of swallowing them.
- **Affected**: `frontend/src/App.svelte`, `frontend/src/lib/BookDetail.svelte`.
- **Watch out**: Existing unrelated Svelte accessibility warnings remain in `App.svelte` and `Reader.svelte`; do not treat those as this routing fix.

### [2026-07-13] Reviewer found incomplete context and fallback contracts
- **Problem**: Search/book/content JavaScript paths did not all inherit cancellation; fingerprint fallback omitted source-session cookies; typed TOC objects skipped explicit volume flags; BookDetail accepted stale async responses and could hang on empty IDs.
- **Fix**: Threaded contexts through executor and analyzers, passed effective session headers to fallback, evaluated `isVolume` for typed objects, and added generation/empty-ID guards to BookDetail.
- **Affected**: `backend/internal/book/search.go`, `backend/internal/fingerprint/client.go`, `backend/internal/book/chapterlist.go`, `frontend/src/lib/BookDetail.svelte`.
- **Watch out**: Keep adding cancellation and source-session tests when new rule-evaluation paths are introduced.

### [2026-07-13] Post-review raw 趣书网吧 E2E passed
- **Problem**: The transport and parser corrections needed verification after the reviewer fixes, not just deterministic tests.
- **Fix**: Restarted a fresh server with raw 939-source data and reran UI search → add → 438-page TOC → first chapter; Playwright observed 135 rendered paragraphs and actual Chinese content.
- **Affected**: `/tmp/novelreader_final_e2e.log`; raw source index 778.
- **Watch out**: The remaining Phase 1 gate is a repeatable multi-source harness, not another one-off manual sample.

### [2026-07-13] Pivoted away from unreachable 笔趣小说
- **Problem**: `m.bqgcn.net` repeatedly returned DNS/503 failures independently through both base-URL Playwright checks and direct HTTP checks, preventing meaningful parser verification.
- **Fix**: Classified it as an upstream connectivity failure and removed it from the active verification target; future validation will use another reachable raw source.
- **Affected**: Verification workflow and `PLAN.md` next action.
- **Watch out**: Do not spend implementation time on a source until its base URL is independently reachable; retain the failure as transport evidence only.

### [2026-07-13] Redirected raw source required final-URL resolution
- **Problem**: Raw `中文看书（优）` redirects from `wap.zwkan.com` to `wap.zwkan.cc`; its JavaScript rule returned relative `/search`, which NovelReader resolved against the stale `.com` origin and timed out.
- **Fix**: Source sessions now retain the latest staged response URL; URL JavaScript results resolve relative to that final URL. Added a redirect regression test.
- **Affected**: `backend/internal/sourceexec/session.go`, `backend/internal/analyzer/js.go`, `backend/internal/analyzer/urlbuilder.go`.
- **Watch out**: Repeated upstream timeout after one successful fixed run is transport instability, not evidence against final-URL resolution.

### [2026-07-13] Alternate raw source reached TOC but exposed stale content selector
- **Problem**: Raw `神话之后（优+）` search and TOC succeeded with 2271 chapters, but its first chapter returned only the title under `id.chaptercontent@p@html`.
- **Fix**: Inspected the exact raw chapter HTML: `#chaptercontent` contains text and `<br>` nodes but no `<p>` elements, so the empty content is a source-rule/site markup mismatch rather than a transport failure; no speculative parser workaround was added.
- **Affected**: Raw source index 84; `/tmp/shen_chap`; `/tmp/novelreader_shen.log`.
- **Watch out**: Preserve `趣书网吧`/`八叉书库` as content-positive gates and use this source for search/TOC coverage unless its raw rule is updated.

### [2026-07-13] Conformance runner exposed non-callable JS response methods
- **Problem**: Production-mode raw index 1 failed during URL JavaScript because `java.get(...).header("location")` was exposed as a Go map member that goja could not call.
- **Fix**: Added real goja response objects with callable `header`, `headers().get`, `body()`, `code()`, and `raw().request().url()` methods; added deterministic regression coverage.
- **Affected**: `backend/internal/analyzer/js.go`, `backend/internal/analyzer/js_response_method_test.go`, `backend/internal/conformance/`.
- **Watch out**: Keep the conformance CLI on production fingerprint transport; deterministic fixtures use normal local HTTP.

### [2026-07-13] Phase 0 conformance runner implemented
- **Problem**: Live verification depended on ad hoc commands and could not persist raw identity, exact requests, response samples, and classification in one reproducible record.
- **Fix**: Added `internal/conformance` and `cmd/conformance`; records raw index/hash, expanded request, redacted headers, response status/final URL/body sample, rule field, extracted results, and failure category. Production-mode run covered raw indices 1, 84, and 89.
- **Affected**: `backend/internal/conformance/`, `backend/cmd/conformance/main.go`, `PLAN.md`.
- **Watch out**: The broad fixture corpus remains before Phase 0 can close.

### [2026-07-13] Conformance runner gained crash-abort and golden taxonomy checks
- **Problem**: A runner could continue after its server disappeared, and category labels lacked deterministic coverage.
- **Fix**: Added optional `-health-url` checks before/after runs and a final health gate; added golden success, legitimate-zero, rule-mismatch, and WebView classification tests.
- **Affected**: `backend/internal/conformance/runner.go`, `backend/internal/conformance/health_test.go`, `backend/internal/conformance/golden_test.go`, `backend/cmd/conformance/main.go`.
- **Watch out**: Use `-health-url` whenever the runner targets a live server; the runner itself uses direct source transport by design.

### [2026-07-13] Executable fixture corpus exposed broken JSONPath input handling
- **Problem**: JSONPath evaluation passed raw `[]byte` to the selector library instead of a decoded object, and JSON element extraction silently truncated results at 50.
- **Fix**: Decode JSON once before selection, centralize selector fallback, remove the element cap, and add fixture plus 75-element regression coverage.
- **Affected**: `backend/internal/analyzer/modes_json.go`, `backend/internal/analyzer/json_conformance_test.go`, `testdata/booksource/`.
- **Watch out**: Keep large JSON TOCs covered; do not reintroduce arbitrary parser caps for workflow data.

### [2026-07-13] Phase 0 fixture corpus completed
- **Problem**: Compatibility tests had isolated inline fixtures but no auditable corpus covering each transport and rule category declared in the architecture.
- **Fix**: Added a manifest-backed deterministic corpus with executable expectations for search, detail, TOC, content, JSON, XPath, Regex, JavaScript POST, charset, cookie, pagination, and WebView classification; transport-facing fixtures now run through the shared request/session paths.
- **Affected**: `testdata/booksource/`, `backend/internal/conformance/fixture_manifest_test.go`, `backend/internal/conformance/fixture_workflow_test.go`, `.gitignore`.
- **Watch out**: Keep fixtures small and offline; live-site samples remain a separate verification gate.

### [2026-07-13] BookSource import/export dropped future fields
- **Problem**: `BookSource.UnmarshalJSON` mapped only known typed fields, serialized rule objects as strings, and SQLite persistence had no place for unknown imported fields.
- **Fix**: Retained the original source JSON, restored it exactly when unchanged, merged preserved fields after typed updates, and persisted the raw JSON through a backward-compatible `source_json` column migration.
- **Affected**: `backend/internal/booksource/entity.go`, `backend/internal/booksource/json.go`, `backend/internal/booksource/store.go`, `backend/internal/booksource/json_test.go`.
- **Watch out**: Backend does not execute UI-only future fields; they remain available for lossless re-export.

### [2026-07-13] Shared transports sent UTF-8 POST bodies for non-UTF-8 sources
- **Problem**: Search and book workflows encoded POST values ad hoc, while direct `RequestSpec` transports sent raw UTF-8 bodies; fingerprint and normal transports could therefore disagree.
- **Fix**: Added `sourceexec.EncodeRequestBody` and applied it in both HTTP transports; removed workflow-level pre-encoding and added a GBK request-body regression test.
- **Affected**: `backend/internal/sourceexec/request.go`, `backend/internal/sourceexec/http_transport.go`, `backend/internal/fingerprint/transport.go`, `backend/internal/book/search.go`, `backend/internal/sourceexec/http_transport_charset_test.go`.
- **Watch out**: Form-body encoding still intentionally handles flat `key=value` pairs; nested form structures need a real source before expanding it.

### [2026-07-13] Parsed bodyJs was never applied to responses
- **Problem**: URL options preserved `bodyJs`, but workflows passed the raw response directly to analyzers, so sources relying on body transformation parsed the wrong document.
- **Fix**: Added `Executor.TransformResponse` and applied it after transport success for search, book info, TOC pagination, chapter content, and next content pages; added executor regression coverage.
- **Affected**: `backend/internal/sourceexec/executor.go`, `backend/internal/sourceexec/executor_test.go`, `backend/internal/book/search.go`.
- **Watch out**: `webJs` still requires a real WebView transport; these paths fail explicitly instead of silently parsing raw HTML.

### [2026-07-13] Redirect-origin cookies disappeared from source sessions
- **Problem**: Transports synchronized cookies only for the final redirect URL, losing cookies set by an origin host when redirects crossed hosts.
- **Fix**: Sync both the original request URL and final response URL in HTTP and fingerprint clients; added cross-host redirect regression tests.
- **Affected**: `backend/internal/sourceexec/http_transport.go`, `backend/internal/sourceexec/session_transport_test.go`, `backend/internal/fingerprint/client.go`, `backend/internal/fingerprint/transport_cookie_test.go`.
- **Watch out**: Cookie scope remains governed by the standard cookie jar; do not flatten host/path restrictions into one global map.

### [2026-07-13] Enrichment links resolved against the source root
- **Problem**: Search `bookUrl`/`coverUrl` and book-info cover/TOC links were resolved against `bookSourceUrl`, so redirects or nested detail paths produced incorrect absolute URLs.
- **Fix**: Resolve extracted enrichment links against the final response URL; added redirect fixtures for search and book info.
- **Affected**: `backend/internal/book/search.go`, `backend/internal/book/search_executor_test.go`, `backend/internal/book/bookinfo_executor_test.go`.
- **Watch out**: TOC chapter links already use each fetched page's final URL; preserve that behavior for pagination.

### [2026-07-13] Search field analyzers lost source session state
- **Problem**: Search requests used a source session, but per-result analyzers did not receive it, so field JS rules could not read source cookies or variables.
- **Fix**: Propagated the existing `SourceState` into each result analyzer and added a cookie-backed field-rule regression test.
- **Affected**: `backend/internal/book/search.go`, `backend/internal/book/search_executor_test.go`.
- **Watch out**: Keep session scope isolated per source search; do not reuse a result analyzer across sources.

### [2026-07-13] Conformance parser emitted relative search links
- **Problem**: The production searcher resolved links after transport, but the conformance runner parsed against the source root and reported raw relative `bookUrl` values.
- **Fix**: Added final-response-base parsing and used it in the runner; live verification produced absolute URLs for source indices 1 and 84 while index 89 remained a transport timeout.
- **Affected**: `backend/internal/book/search.go`, `backend/internal/conformance/runner.go`.
- **Watch out**: Keep the response final URL as the parser base; source root is only the fallback when transports omit it.

### [2026-07-13] Conformance records lacked response diagnostics
- **Problem**: Records showed request metadata and a body sample but omitted response headers and redirect destinations, making browser/curl comparisons incomplete.
- **Fix**: Added bounded redirect-chain capture to the normal fetcher, response-header capture with sensitive-value redaction, and corresponding sourceexec/conformance fields and tests.
- **Affected**: `backend/internal/fetcher/fetcher.go`, `backend/internal/sourceexec/request.go`, `backend/internal/sourceexec/http_transport.go`, `backend/internal/conformance/runner.go`.
- **Watch out**: Fingerprint transport currently exposes final URL and headers but not a complete redirect chain; tls-client redirect-hook integration still needs isolated validation before relying on it.

### [2026-07-13] Fingerprint diagnostics omitted redirect targets
- **Problem**: The fingerprint client shared a long-lived tls-client instance whose public interface did not expose a request-scoped redirect callback, so conformance records had empty chains for fingerprint requests.
- **Fix**: Added opt-in `CaptureRedirects` configuration that creates a traced fingerprint client only for diagnostics, preserving normal production connection reuse; added cross-host redirect coverage.
- **Affected**: `backend/internal/fingerprint/client.go`, `backend/internal/fingerprint/transport_cookie_test.go`, `backend/internal/conformance/runner.go`.
- **Watch out**: Keep capture opt-in; per-request traced client creation is diagnostic overhead, not a default production path.

### [2026-07-13] Header option casing caused duplicate defaults
- **Problem**: HTTP transport checked `Origin` and `Content-Type` map keys case-sensitively, so lowercase source headers could be overridden by option/default values.
- **Fix**: Added case-insensitive header detection and regression coverage for explicit lowercase values.
- **Affected**: `backend/internal/sourceexec/http_transport.go`, `backend/internal/sourceexec/http_transport_test.go`.
- **Watch out**: Preserve explicit source headers; URL-option defaults should fill gaps, not overwrite them.

### [2026-07-13] Stateless JS clients lost source cookies
- **Problem**: JSVM only scoped clients exposing `ForSource`; stateless normal HTTP clients could sync response cookies into SourceState but never send them on the next `java.ajax/get/post` request.
- **Fix**: Added `fetcher.SessionHTTPClient` and used it as the fallback source scope; added a stateless-cookie regression test.
- **Affected**: `backend/internal/fetcher/session_client.go`, `backend/internal/fetcher/session_client_test.go`, `backend/internal/analyzer/js.go`, `backend/internal/analyzer/js_cookie_sync_test.go`.
- **Watch out**: Keep the adapter scoped to one SourceState; native fingerprint `ForSource` remains preferred.

### [2026-07-13] JS bridge adapter overwrote explicit POST content types
- **Problem**: The session adapter's `Post` path discarded the caller's content type and always defaulted to form encoding.
- **Fix**: Preserve an explicit content type before delegating to the context-aware POST path; added JSON content-type coverage.
- **Affected**: `backend/internal/fetcher/session_client.go`, `backend/internal/fetcher/session_client_test.go`.
- **Watch out**: Do not mutate caller-owned header maps while adding defaults.

### [2026-07-13] Source headers were absent from URL-JS HTTP calls
- **Problem**: Source-level headers were merged after URL construction, so `java.ajax/get/post` calls made by URL JavaScript did not receive source defaults; exact-key merges also allowed duplicate HTTP names across workflows.
- **Fix**: Added case-insensitive `sourceexec.MergeHeaders`, applied it to all executor-backed book workflows and conformance, and stored source headers in SourceSession for normal and fingerprint JS clients.
- **Affected**: `backend/internal/sourceexec/request.go`, `backend/internal/sourceexec/session.go`, `backend/internal/book/search.go`, `backend/internal/analyzer/js.go`, `backend/internal/fetcher/session_client.go`, `backend/internal/fingerprint/client.go`.
- **Watch out**: Explicit per-request headers remain higher priority than source defaults; keep source headers isolated with the session.

### [2026-07-13] dnsIp metadata was ignored by normal HTTP transport
- **Problem**: URL options preserved `dnsIp` in RequestSpec but normal HTTP always resolved the hostname through system DNS.
- **Fix**: Added per-request dialer override that connects to the configured IP without changing the original URL host; fingerprint transport delegates dnsIp requests to its normal fallback.
- **Affected**: `backend/internal/fetcher/fetcher.go`, `backend/internal/sourceexec/http_transport.go`, `backend/internal/fingerprint/transport.go`.
- **Watch out**: The override accepts the first valid comma-separated IP and is intentionally request-scoped; add multi-IP failover only when sources require it.

### [2026-07-13] URL method HEAD was rejected by shared transport
- **Problem**: URL options mapped `HEAD` into RequestSpec but the shared transport only dispatched GET and POST.
- **Fix**: Added context-aware normal HTTP HEAD execution and fingerprint dispatch while preserving URL-option charset, retry, headers, and dnsIp behavior.
- **Affected**: `backend/internal/fetcher/fetcher.go`, `backend/internal/sourceexec/http_transport.go`, `backend/internal/fingerprint/transport.go`.
- **Watch out**: RequestSpec still intentionally rejects methods outside Legado's GET/POST/HEAD contract.

### [2026-07-13] Structured POST bodies were parsed as strings or sent as forms
- **Problem**: URL options with object/array `body` values failed JSON option parsing, and JSON strings were labeled form data or charset-encoded.
- **Fix**: Preserve structured bodies as compact JSON and centralize raw JSON versus form POST preparation for normal and fingerprint transports.
- **Affected**: `backend/internal/analyzer/urlbuilder.go`, `backend/internal/sourceexec/request.go`, `backend/internal/sourceexec/http_transport.go`, `backend/internal/fingerprint/transport.go`.
- **Watch out**: Explicit `Content-Type` remains authoritative; only URL-encoded form bodies receive charset encoding.

### [2026-07-14] WebView sources were rejected before transport selection
- **Problem**: Book workflows returned `unsupported_webview` as soon as `webView:true` appeared, so browser-backed execution could not participate in search, detail, TOC, or content flows.
- **Fix**: Added a routing transport and optional Python Patchright worker client; workflows now select browser transport by request metadata while retaining normal fingerprint/HTTP behavior.
- **Affected**: `backend/internal/sourceexec/router.go`, `backend/internal/webview/`, `webview-worker/`, `backend/internal/book/search.go`, `backend/cmd/server/main.go`, `backend/internal/config/config.go`.
- **Watch out**: HTTP-only deployments intentionally return an explicit WebView capability error; install and keep the worker private before enabling `WEBVIEW_ENDPOINT`.

### [2026-07-14] WebView execution had no headless runtime boundary
- **Problem**: The request contract carried WebView metadata, but no deployable browser runtime could execute page JavaScript in a GUI-less environment.
- **Fix**: Added a versioned localhost JSON protocol, Go client, session cookie synchronization, optional routing, and a pinned Patchright Python worker with isolated browser contexts and bounded concurrency. A local fixture verified DOM JS, delay, final DOM, and cookies.
- **Affected**: `backend/internal/webview/`, `backend/internal/sourceexec/router.go`, `webview-worker/`, `README.md`, `.gitignore`.
- **Watch out**: A real source gate is still pending; Android-only Java/WebView APIs and `dnsIp` remain explicit limitations of the browser transport.

### [2026-07-14] Browser worker could accumulate queued work and stale browser state
- **Problem**: Per-request contexts were closed, but the HTTP server could create unbounded pending tasks and the long-lived browser process had no crash recovery, recycling, or cancellation-safe shutdown path.
- **Fix**: Added a bounded request queue, fixed consumer count, whole-request deadlines, cancellation-safe context cleanup, graceful worker shutdown, browser reconnect/restart handling, and configurable context-count recycling. Fixture checks pass with recycling forced after every request.
- **Affected**: `webview-worker/browser.py`, `webview-worker/worker.py`, `webview-worker/README.md`, `webview-worker/Dockerfile`.
- **Watch out**: Tune `WEBVIEW_MAX_PAGES`, `WEBVIEW_MAX_PENDING`, and `WEBVIEW_MAX_CONTEXTS_PER_BROWSER` from production memory measurements; never expose the worker publicly.

### [2026-07-14] Workflow sessions had unbounded lifetime
- **Problem**: The singleton Searcher registry retained every book and chapter session indefinitely, including browser/fingerprint clients stored in session memory.
- **Fix**: Added idle TTL and bounded oldest-first eviction with alias removal for book/chapter mappings; default limits are 4,096 sessions and one hour idle.
- **Affected**: `backend/internal/sourceexec/session_registry.go`, `backend/internal/sourceexec/session_registry_test.go`.
- **Watch out**: User-scoped registries are still required when authentication/multi-user state is introduced.

### [2026-07-14] Source fan-out recreated connection pools
- **Problem**: Search, detail, TOC, and content created a new HTTP client per workflow, discarding reusable keep-alive connections; shared clients could not safely carry source cookies.
- **Fix**: Added stateless client clones that share transports but no jars, injected SourceSession cookies explicitly for stateless HTTP, reused the pool across book workflows, and cached scoped JS/fingerprint clients in SourceSession memory.
- **Affected**: `backend/internal/fetcher/fetcher.go`, `backend/internal/fetcher/fetcher_redirect_test.go`, `backend/internal/sourceexec/http_transport.go`, `backend/internal/book/search.go`, `backend/internal/analyzer/js.go`.
- **Watch out**: Cookie state belongs to SourceSession; do not reintroduce a shared cookie jar.

### [2026-07-14] Browser capacity failures were permanent
- **Problem**: A full browser queue returned 503 and the Go client surfaced it immediately, causing avoidable source failures under normal fan-out.
- **Fix**: Added bounded exponential retry within the caller's context deadline; browser admission remains bounded and returns a clear error after capacity cannot free.
- **Affected**: `backend/internal/webview/worker_http.go`, `backend/internal/webview/client.go`, `backend/internal/webview/client_test.go`.
- **Watch out**: Metrics/load tests are still needed to tune retry delays and queue/page limits.

### [2026-07-14] Concurrent searches had no process-wide admission
- **Problem**: Each SearchStream bounded itself, but concurrent API searches could multiply source goroutines and remote requests without a process-wide cap.
- **Fix**: Added a process-wide source-fetch semaphore while preserving the per-search fan-out limit and cancellation-aware admission.
- **Affected**: `backend/internal/book/search.go`.
- **Watch out**: Tune the global limit with load tests; a single Searcher registry is not a substitute for per-user quotas.

### [2026-07-14] Capacity pressure was not observable
- **Problem**: Limits existed but active work, failures, queue pressure, and browser recycling were not exposed for tuning.
- **Fix**: Added Searcher capacity snapshots, Patchright worker health counters, and concurrent registry stress coverage.
- **Affected**: `backend/internal/book/search.go`, `webview-worker/browser.py`, `webview-worker/worker.py`, `backend/internal/sourceexec/session_registry_test.go`.
- **Watch out**: Metrics are process-local; add aggregation and user-level quotas when deployment topology/authentication is defined.

### [2026-07-14] Capacity limits lacked an end-to-end load gate
- **Problem**: Unit tests covered individual limiters, but no deterministic test proved that concurrent searches across hundreds of sources stayed bounded and reclaimed all active work.
- **Fix**: Added a two-concurrent-search fixture with 400 total source requests, peak-concurrency assertion, completion/failure accounting, and zero-active-work verification; race tests pass.
- **Affected**: `backend/internal/book/search_capacity_test.go`, `backend/internal/book/search.go`.
- **Watch out**: Fixture limits validate invariants, not ideal production values; tune from real latency, memory, and upstream rate-limit measurements.

### [2026-07-14] Conformance could not exercise browser-backed requests
- **Problem**: The raw conformance runner always classified `webView:true` requests as unsupported, so production browser routing had no reproducible diagnostic gate.
- **Fix**: Added an optional Patchright endpoint to the runner and CLI, retained explicit unsupported behavior when omitted, and verified raw index 213 reaches the browser transport and returns a rendered response.
- **Affected**: `backend/internal/conformance/runner.go`, `backend/cmd/conformance/main.go`, `backend/internal/conformance/fixture_workflow_test.go`, `README.md`.
- **Watch out**: Index 213 currently has a rule mismatch against its rendered DOM; a source with browser flags on detail/TOC/content URLs remains pending.
### [2026-07-14] Headless WebView POST compatibility was incomplete
- **Problem**: Patchright's APIResponse does not expose Playwright's `all_headers()`, POST response text was always decoded as UTF-8, and `page.set_content` left the final URL as `about:blank`; relative links also percent-escaped WebView option suffixes.
- **Fix**: Added Patchright-compatible header access, source/response charset decoding, final-URL fallback, charset propagation in the protocol, and option-preserving relative URL resolution. Added a reusable conformance workflow and verified raw index 788 through search, detail, 828-chapter TOC, and first-chapter content.
- **Affected**: `webview-worker/browser.py`, `backend/internal/webview/`, `backend/internal/book/search.go`, `backend/internal/conformance/`, `backend/cmd/conformance/main.go`, `README.md`.
- **Watch out**: The verified index 788 uses WebView for search; index 779 now covers later browser stages, but upstream source rules and availability can change.
### [2026-07-14] Imported WebView options used Legado single-quote syntax
- **Problem**: Relative chapter and content URLs containing `,{'webView': true}` were percent-escaped or rejected as invalid JSON, so later browser stages silently fell back to normal HTTP.
- **Fix**: Preserved request-option suffixes during URL resolution and normalized the single-quoted option form before parsing; the workflow fixture and real index 779 now prove browser routing.
- **Affected**: `backend/internal/analyzer/urlbuilder.go`, `backend/internal/book/search.go`, `backend/internal/conformance/workflow_test.go`.
- **Watch out**: Normalization intentionally targets Legado option objects; arbitrary malformed JavaScript fragments remain unsupported.
### [2026-07-14] Later-stage browser workflow exposed cookie and deadline incompatibilities
- **Problem**: Patchright rejected URL cookies carrying both `url` and `path`, and the fixed ten-second workflow deadline expired while a browser chapter followed paginated content.
- **Fix**: Emit either URL-based or domain/path-based browser cookies, normalize empty cookie paths, add Patchright-compatible response handling, and make the Searcher workflow timeout configurable while retaining the ten-second default.
- **Affected**: `webview-worker/browser.py`, `backend/internal/book/search.go`, `backend/internal/conformance/workflow.go`.
- **Watch out**: The conformance gate used a bounded 60-second timeout; production defaults should be tuned from observed source latency rather than increased globally by default.

### [2026-07-14] Real WebView regression depended on an upstream site
- **Problem**: Source 779 proved later-stage browser routing live, but its availability and HTML could change independently of NovelReader.
- **Fix**: Added a local replay using source 779's detail, TOC, and content rule shapes with canned pages and a fake worker; it asserts single-quoted option routing, HTTP-to-browser cookie transfer, browser cookie persistence, and content pagination.
- **Affected**: `backend/internal/conformance/workflow_test.go`.
- **Watch out**: Keep the live smoke gate separate; the replay proves engine behavior, not current upstream compatibility.

### [2026-07-14] Capacity defaults assumed a large unbounded host
- **Problem**: Search fan-out, JavaScript runtimes, retained sessions, and browser admission were sized independently and several production limits were hard-coded, making a small container prone to avoidable memory and CPU pressure.
- **Fix**: Added a 2-vCPU/4-GB baseline of 16 per-search and 32 global source requests, 4 JavaScript runtimes, 1,024 sessions with 30-minute TTL, 2 browser pages, 8 pending browser requests, and recycling after 100 contexts; backend limits are environment-configurable, tested, and a medium-server starting profile is documented.
- **Affected**: `backend/internal/config/`, `backend/internal/book/search.go`, `backend/internal/book/search_capacity_test.go`, `backend/internal/analyzer/js.go`, `backend/internal/analyzer/js_pool_test.go`, `backend/cmd/server/main.go`, `webview-worker/worker.py`, `webview-worker/test_worker.py`, `README.md`.
- **Watch out**: These are conservative starting values, not benchmark-derived maxima; preserve Docker resource limits and tune from active/queue/failure metrics.
### [2026-07-14] Search fan-out buffered completed sources
- **Problem**: Fan-out filled semaphore slots and waited to launch every source before consuming completion events, delaying SSE results until most or all of a large batch had already run.
- **Fix**: Replaced launch-then-drain behavior with a fixed worker pool whose completion channel is consumed immediately; a blocking fixture proves the first callback arrives before the full batch starts.
- **Affected**: `backend/internal/book/search_stream.go`, `backend/internal/book/search_batch_test.go`.
- **Watch out**: Keep completion consumption concurrent with job admission; otherwise cancellation again loses useful partial progress.
### [2026-07-14] Batch preparation hid source-store failures
- **Problem**: The API treated every non-stale batch preparation failure as invalid client input, so database failures returned 400 responses with internal error text.
- **Fix**: Added typed batch/cursor validation errors, return sanitized 500 responses for internal preparation failures, and covered the failure boundary.
- **Affected**: `backend/internal/book/search_batch.go`, `backend/internal/api/search.go`, `backend/internal/api/search_test.go`.
- **Watch out**: Keep trust-boundary validation errors distinct from storage and execution failures.
### [2026-07-14] Shelving discarded alternate search sources
- **Problem**: The enrichment request ignored `alternateSources`, so a cumulatively merged result lost every fallback source when added to the shelf.
- **Fix**: Decode alternatives into the new book before enrichment or fallback persistence and cover the source-missing fallback.
- **Affected**: `backend/internal/api/server.go`, `backend/internal/api/enrich_test.go`.
- **Watch out**: Every frontend shelf fallback and future book-creation endpoint must preserve the same alternate-source contract.
### [2026-07-14] Interrupted search state could restore without Retry
- **Problem**: Partial progress was persisted without marking an active attempt, and a continuation had no retry cursor until the server start event arrived; reload or an early disconnect could leave no valid action.
- **Fix**: Capture the attempt cursor before opening SSE, persist an in-flight marker and attempt controls, normalize restored attempts to Retry, and reject malformed saved state.
- **Affected**: `frontend/src/lib/SearchPage.svelte`.
- **Watch out**: Any future search-state schema change must preserve the active attempt's cursor, batch size, and concurrency together.
### [2026-07-14] Scoped fingerprint transports retained idle sockets
- **Problem**: Every search source created an isolated fingerprint client whose idle pools were never closed; a Docker gate retained about 11,400 established connections, reached 838 MiB backend memory, and eventually caused fixture connection refusals.
- **Fix**: Added explicit scoped-client idle-pool closure, forwarded it through fingerprint and routing transports, and defer closure when a search-source workflow ends. Post-fix Docker runs completed 48,000 requests with no failures or retained established connections and 27.32 MiB observed peak backend memory.
- **Affected**: `backend/internal/fingerprint/client.go`, `backend/internal/fingerprint/transport.go`, `backend/internal/fingerprint/transport_test.go`, `backend/internal/sourceexec/router.go`, `backend/internal/book/search.go`, `README.md`.
- **Watch out**: Long-lived detail/TOC/content sessions intentionally retain their scoped client until session eviction; only ephemeral search sessions close immediately.
### [2026-07-14] WebView image bound only to container loopback
- **Problem**: The worker image published port 8787 but the process listened on `127.0.0.1`, so Docker port forwarding and peer containers received connection resets.
- **Fix**: Set the image-only `WEBVIEW_WORKER_HOST=0.0.0.0` default while preserving loopback for direct local runs; rebuilt-image health verification passes through the published port.
- **Affected**: `webview-worker/Dockerfile`, `README.md`.
- **Watch out**: Keep the worker on a private container network; its execute endpoint can navigate arbitrary HTTP(S) URLs.
### [2026-07-14] Browser recycle reported false worker failure
- **Problem**: Recycling intentionally closed Chromium for lazy relaunch, but health required a connected browser and returned `ok:false`, which would trigger orchestrator restart loops after every 100 contexts.
- **Fix**: Health now reports readiness from the live bounded consumer pool; a deterministic async test covers healthy lazy-recycle state and failed consumers, and a 110-request Docker gate remained healthy after recycle.
- **Affected**: `webview-worker/browser.py`, `webview-worker/test_worker.py`.
- **Watch out**: Consumer task failure must continue to make health fail even when Chromium can be relaunched.
### [2026-07-14] Alpine fixture lacked the expected HTTP applet
- **Problem**: The E2E fixture initially reused the app image with `busybox httpd`, but the latest Alpine BusyBox omits that applet and the fixture exited 127.
- **Fix**: The inactive `e2e` profile reuses the already-built worker image and Python standard-library HTTP server, adding no image or production dependency.
- **Affected**: `compose.yaml`, `docker-e2e.sh`, `testdata/docker/`.
- **Watch out**: Keep the fixture profile test-only and unexposed; production startup must not enable `e2e`.
### [2026-07-15] Phase 4 checklist overstated crawl compatibility
- **Problem**: Recorded live successes obscured missing `bookInfoInit` and first/middle/last matrix coverage plus partial TOC fields/pagination, analyzer context, content pagination/fallback, and API failure semantics.
- **Fix**: Audited every Phase 4 task against NovelReader code/tests and the Legado reference, closed only evidenced URL continuity, and ordered the remaining work into eight TDD slices.
- **Affected**: `PLAN.md`, `backend/internal/book/`, `backend/internal/api/`, `backend/internal/conformance/`.
- **Watch out**: Do not infer task completion from one successful source or isolated rule fixtures; the required raw-source workflow matrix remains the gate.
### [2026-07-16] Book-info initialization flattened structured content
- **Problem**: The analyzer had no `ruleBookInfo.init`; naïve string conversion also destroyed JSON/JavaScript objects, JSoup selections, table fragments, and XPath matches beyond item 50 before detail rules ran.
- **Fix**: Added pre-field initialization with preserved structured JS content, HTML collection serialization, connector semantics, explicit null failure, uncapped XPath elements, and deterministic CSS/JSON/JS/JSoup/XPath coverage.
- **Affected**: `backend/internal/analyzer/`, `backend/internal/book/search.go`, `backend/internal/book/bookinfo_executor_test.go`.
- **Watch out**: The parent task remains open until shared `@put`/`@get` variables and regex init behavior pass raw-source tests.
### [2026-07-16] Global rule-variable preprocessing broke evaluation order
- **Problem**: A first `@put`/`@get` attempt preprocessed whole rules, causing empty-content leaks, unselected branch mutation, lost JavaScript evaluation, and incorrect intermediate-content lookup.
- **Fix**: Discarded the attempt and attached lenient put maps/get templates to parsed rule segments; values now evaluate against root content in segment order, persist through SourceState, and substitute before replacement parsing.
- **Affected**: `backend/internal/analyzer/analyzer.go`, `backend/internal/analyzer/ruleparser.go`, `backend/internal/analyzer/rulevars.go`, `backend/internal/analyzer/modes_regex.go`.
- **Watch out**: Rule variables must remain session-scoped; source-global storage would leak values across concurrent books.
### [2026-07-16] Crawl context snapshots diverged from mutable Legado objects
- **Problem**: URL/body JavaScript, TOC pagination, detail field dependencies, TOC item state, content pagination, and exact next-chapter stopping did not share complete mutable book/chapter context; persisted chapters also lacked base URL/pay/metadata fields.
- **Fix**: Added typed URL/analyzer bindings, persistent book/chapter maps, API/conformance propagation, exact next redirect checks, Legado field ordering/truth/fallbacks, SQLite migration/round-trip fields, and deterministic context/redirect tests.
- **Affected**: `backend/internal/analyzer/`, `backend/internal/sourceexec/`, `backend/internal/book/`, `backend/internal/api/server.go`, `backend/internal/conformance/workflow.go`.
- **Watch out**: TOC pagination still needs structured partial-failure results and URL-only deduplication before Phase 4 closes.
### [2026-07-16] TOC accumulation kept weak duplicate and hid pagination rule failures
- **Problem**: TOC deduplication used URL plus title, so changed titles survived as duplicate chapters; pagination treated JavaScript/list-rule errors like normal end-of-list and error messages omitted partial counts.
- **Fix**: Matched Legado's reverse-before-dedup/final-reverse order with URL-only deduplication, added update-time/reversal regression coverage, introduced `analyzer.ErrNoListValues` for normal optional terminal rules, and surfaced page/chapter counts for parse, next-rule, and fetch failures.
- **Affected**: `backend/internal/analyzer/analyzer.go`, `backend/internal/book/chapterlist.go`, `backend/internal/book/crawl_context_test.go`, `backend/internal/book/toc_pagination_test.go`.
- **Watch out**: The remaining pagination decision is whether callers should receive partial chapters with a typed diagnostic; multiple next URLs are not yet traversed.
### [2026-07-16] TOC pagination lacked a typed fail-closed contract
- **Problem**: Paginated TOC failures returned unstructured errors, only the first next URL was followed, and redirect aliases or same-endpoint URL options could either loop or truncate distinct pages.
- **Fix**: Added `TOCPaginationError` with page/URL/cause/partial-count context, breadth-first multi-next traversal, full request identity for scheduling, final-URL alias protection, and deterministic option/retry/redirect tests while keeping incomplete TOCs out of the cache.
- **Affected**: `backend/internal/book/chapterlist.go`, `backend/internal/book/toc_pagination_test.go`, `PLAN.md`.
- **Watch out**: Content pagination is the next compatibility slice; it needs the same fail-loud diagnostics and request-identity checks.
### [2026-07-16] Content pagination hid mode and boundary semantics
- **Problem**: Content pagination followed only one URL, recursively expanded multi-link page sets, returned untyped partial failures, and could discard valid queued pages when a sibling redirected to the next TOC chapter.
- **Fix**: Matched Legado single-chain versus fixed multi-page behavior, added typed `ContentPaginationError`, preserved full request-option identity and bodyJs/retry behavior, drained queued pages across TOC boundaries, and made SPA fallback explicitly diagnostic after declared-rule extraction.
- **Affected**: `backend/internal/book/content.go`, `backend/internal/book/search.go`, `backend/internal/book/context.go`, `backend/internal/book/content_pagination_test.go`, `backend/internal/book/content_next_chapter_test.go`.
- **Watch out**: API handlers still need to map typed crawl failures to stable HTTP diagnostics before raw-source end-to-end coverage.
### [2026-07-16] API handlers collapsed crawl, storage, and not-found failures
- **Problem**: TOC/content pagination diagnostics were reduced to generic HTTP 500 responses, database errors were reported as missing resources, and configured-source enrichment silently saved incomplete books.
- **Fix**: Added stable structured 502 crawl responses with typed pagination fields, mapped storage failures to 500 and missing resources to 404 codes, and made configured-source enrichment fail loudly while retaining the unimported-source fallback.
- **Affected**: `backend/internal/api/errors.go`, `backend/internal/api/server.go`, `backend/internal/api/crawl_error_test.go`, `PLAN.md`.
- **Watch out**: The raw-source first/middle/last API workflow still needs deterministic and live verification.
### [2026-07-16] Book workflow verification covered only the first chapter
- **Problem**: The conformance runner and API workflow could pass while middle or final chapter URLs/content were broken; the runner also accepted empty extracted content as success.
- **Fix**: Added additive first/middle/last chapter checks with non-empty enforcement, a deterministic raw-source import → search → enrich/add → TOC → content API test, and live verification of raw index 778 across a 179-chapter real book.
- **Affected**: `backend/internal/conformance/workflow.go`, `backend/internal/conformance/workflow_test.go`, `backend/internal/api/workflow_e2e_test.go`, `backend/cmd/conformance/main.go`, `README.md`, `PLAN.md`.
- **Watch out**: Phase 4 still requires the broader source-class completion matrix; one live HTML source does not prove JSON, XPath/Regex, POST/charset, or multi-page coverage.
### [2026-07-16] Isolated fixtures did not prove source-class workflows
- **Problem**: JSONPath, XPath/Regex, POST/charset, and pagination each had isolated rule or transport tests, but none proved detail, TOC, and representative chapter content together.
- **Fix**: Added a deterministic raw-source workflow matrix covering normal HTML, JSONPath, XPath plus Regex, exact GBK POST request encoding, and multi-page TOC/content through first/middle/last checks.
- **Affected**: `backend/internal/conformance/workflow_matrix_test.go`, `README.md`, `PLAN.md`.
- **Watch out**: Keep these fixtures deterministic; live-site checks supplement the matrix but cannot replace it.
### [2026-07-16] Heuristic fallbacks overrode declared crawl rules
- **Problem**: Successful TOCs with book-like chapter URLs could be discarded by URL-pattern catalog discovery, broken declared content selectors could be masked by script JSON, and obsolete URL/content helper wrappers remained after shared executor pagination replaced them.
- **Fix**: Removed heuristic TOC rediscovery and dead wrappers, limited script diagnostics to sources without a declared content selector, and added regressions proving declared TOC/content rules remain authoritative.
- **Affected**: `backend/internal/analyzer/urlbuilder.go`, `backend/internal/book/chapterlist.go`, `backend/internal/book/search.go`, `backend/internal/book/toc_fallback_test.go`, `backend/internal/book/content_pagination_test.go`, `PLAN.md`.
- **Watch out**: Keep intentional Legado fallbacks; empty `tocUrl` must still use the book page and omitted source booleans must retain imported defaults.
### [2026-07-16] Explore contract was undefined despite widespread raw usage
- **Problem**: Phase 7 named Explore fields but did not define category grammar, interactive state, paging/error semantics, source eligibility, session ownership, or a frontend-safe API; 722 raw sources already rely on nonblank independently enabled Explore definitions.
- **Fix**: Audited current Legado and the hash-pinned 939-source compilation, fixed the execution order, defined staged-full category/control compatibility, bounded session and diagnostics contracts, additive endpoints, and strict domain data flow before implementation.
- **Affected**: `PLAN.md`, `reference/legado`, raw `test_booksource4.json` audit evidence.
- **Watch out**: `exploreScreen` and Android style metadata have no current executable upstream contract; preserve them but do not invent behavior.
### [2026-07-17] Omitted Explore enablement imported as disabled
- **Problem**: `enabledExplore` used a non-pointer JSON boolean, so an omitted field became false even though Legado defaults it to true; eligible legacy sources could disappear before category parsing.
- **Fix**: Decode presence separately, default omission to true, preserve explicit false, and add hash-guarded raw Explore fixtures covering the representative source classes.
- **Affected**: `backend/internal/booksource/json.go`, `backend/internal/booksource/json_test.go`, `backend/internal/conformance/explore_fixture_test.go`, `testdata/booksource/explore-sources.json`, `testdata/booksource/README.md`, `PLAN.md`.
- **Watch out**: Normal search `enabled` remains outside this Explore-specific change; Explore eligibility is intentionally independent.
### [2026-07-17] Explore arrays were not strict JSON
- **Problem**: 26 of 373 array-shaped raw Explore definitions use Gson-tolerated multiline strings, single quotes, unquoted keys, or trailing commas; strict JSON would reject valid categories, while evaluating them as JavaScript would execute hidden expressions.
- **Fix**: Added a data-only lenient parser using the existing JavaScript AST parser, recursively accepting literals and rejecting calls/functions; pinned unit and raw-fixture checks cover lenient and legacy forms.
- **Affected**: `backend/internal/book/explore.go`, `backend/internal/book/explore_test.go`, `PLAN.md`.
- **Watch out**: Two raw entries (indices 504 and 522) are syntactically broken even as object literals and must surface category-parse diagnostics.
### [2026-07-17] Explore pages could bypass lifecycle and result semantics
- **Problem**: The first service draft could bypass global source capacity, evict sessions during active requests, inherit search's 20-result cap before reversal, mask mixed list-rule failures as empty results, lose URL-script Book mutations, hold capacity through uncancellable rate sleeps, and omit Legado's single-book detail fallback.
- **Fix**: Added cancellation-safe shared-slot/rate accounting, leased bounded sessions, uncapped Explore parsing with fail-loud empty-list detection, complete-list reversal, shared Book-context detail parsing, sequential replay/dedup state, and regressions for each boundary.
- **Affected**: `backend/internal/analyzer/analyzer.go`, `backend/internal/analyzer/elements_error_test.go`, `backend/internal/book/bookinfo_parse.go`, `backend/internal/book/bookinfo_executor_test.go`, `backend/internal/book/explore_page.go`, `backend/internal/book/explore_service.go`, `backend/internal/book/explore_session.go`, `backend/internal/book/explore_capacity_test.go`, `backend/internal/book/explore_compat_test.go`, `backend/internal/book/search.go`, `backend/internal/booksource/store.go`.
- **Watch out**: Interactive controls are still intentionally unavailable and must not be exposed as working until their session-bound fixtures pass.
### [2026-07-17] JavaScript contexts could not stop CPU-bound rules
- **Problem**: `JSVM.EvalContext` blocked unconditionally on pooled runtimes and used `RunString` without interruption, so a source rule such as `while(true){}` could permanently consume both a runtime and global Explore capacity despite its timeout.
- **Fix**: Made runtime checkout context-aware, interrupted goja when the context ends, synchronized the interrupt callback, cleared interrupt state before pool reuse, and added CPU-loop/runtime-reuse plus AJAX cancellation regressions.
- **Affected**: `backend/internal/analyzer/js.go`, `backend/internal/analyzer/js_context_test.go`, `backend/internal/book/explore_script.go`, `backend/internal/book/explore_script_test.go`.
- **Watch out**: Every new JavaScript entry point must call `EvalContext` with a bounded context; direct unbounded `Eval` remains suitable only for explicitly synchronous internal/tests usage.
### [2026-07-18] Explore field templates were treated as selectors
- **Problem**: JSON field rules containing `{{...}}` were dispatched as JSONPath selectors, and newline `@js:` continuations were not split from their preceding rule, so raw JSON Explore results lost generated book URLs and fell into unrelated detail fallback.
- **Fix**: Added shared quote/escape-aware template scanning, evaluated embedded rules and JavaScript against each current intermediate value, split newline `@js:` chains, and executed local responses for all pinned raw Explore request/rule classes.
- **Affected**: `backend/internal/analyzer/analyzer.go`, `backend/internal/analyzer/ruleparser.go`, `backend/internal/analyzer/rulevars.go`, `backend/internal/analyzer/template.go`, `backend/internal/analyzer/urlbuilder.go`, `backend/internal/analyzer/rule_template_test.go`, `backend/internal/book/explore_page_fixture_test.go`, `testdata/booksource/explore-*`, `testdata/booksource/README.md`.
- **Watch out**: Template delimiters inside quoted JavaScript must remain ignored by brace balancing; malformed/unclosed templates remain literal rather than being partially evaluated.
### [2026-07-18] Explore refresh could retarget queued entry IDs
- **Problem**: Regenerated catalogs reused positional `entry-N` IDs, so a control or page request queued before refresh could resume afterward against a different entry at the same position and execute the wrong source action or URL.
- **Fix**: Added catalog generations with fresh ordered IDs on every successful refresh, atomically replaced categories and page state, and proved a leased page queued behind a blocked control action fails as stale after refresh.
- **Affected**: `backend/internal/book/explore_control.go`, `backend/internal/book/explore_control_test.go`, `backend/internal/book/explore_session.go`, `backend/internal/book/explore_service.go`, `backend/internal/analyzer/js.go`.
- **Watch out**: API/frontend consumers must replace their displayed catalog with every control response and never persist entry IDs beyond the owning session/catalog generation.
### [2026-07-18] Explore API could leak causes and null collections
- **Problem**: Naively logging `ExploreError.Error()` exposed its private source-rule cause, while exhausted pages inherited a nil result slice and serialized `books:null`, making both security and collection shape depend on failure details.
- **Fix**: Added strict bounded handlers and safe copied diagnostics, logged only stable classification fields, normalized empty results/diagnostics to arrays, and mapped unavailable WebView transport through a typed sentinel rather than error text.
- **Affected**: `backend/internal/api/explore.go`, `backend/internal/api/explore_test.go`, `backend/internal/api/server.go`, `backend/internal/book/explore_page.go`, `backend/internal/book/explore_service.go`, `backend/internal/sourceexec/router.go`.
- **Watch out**: New Explore diagnostics must add explicit status mapping and must never serialize or log wrapped causes, source syntax, headers, cookies, or action code.
### [2026-07-18] Revisiting an Explore category could lose prior pages
- **Problem**: The first frontend selected every category by requesting page 1, but the server retains authoritative per-category progression; revisiting a category already on page 2 produced `page_conflict` and left the client unable to reconstruct pages it had discarded.
- **Fix**: Cached results, next page, and exhaustion by generation-scoped category ID, restored cached state on revisit, ignored active-category reselection, cleared caches on catalog generation change, and added recovery-state regressions.
- **Affected**: `frontend/src/lib/ExplorePage.svelte`, `frontend/src/lib/exploreState.js`, `frontend/src/lib/exploreState.test.mjs`.
- **Watch out**: Category caches are intentionally tab-memory only and must be discarded whenever refreshed catalog IDs change or the source session reopens.
### [2026-07-18] Final Explore audit found cross-session and capacity leaks
- **Problem**: Reused goja runtimes retained source globals, concurrent rate waiters reserved the same instant, tolerant field helpers could turn rule failures into exhausted pages, retained URL sets had no per-session ceiling, and fresh runtimes dropped libraries installed through `LoadLib`.
- **Fix**: Replaced runtimes after every evaluation while reloading validated shared libraries, atomically reserved spaced source timestamps, added strict composite field evaluation with missing-JSON classification, capped retained URLs at 2,000 before mutation, reset capacity on catalog refresh, and exposed an explicit capacity diagnostic.
- **Affected**: `backend/internal/analyzer/analyzer.go`, `backend/internal/analyzer/js.go`, `backend/internal/analyzer/js_session_test.go`, `backend/internal/analyzer/modes_json.go`, `backend/internal/analyzer/rule_template_test.go`, `backend/internal/api/explore.go`, `backend/internal/api/explore_test.go`, `backend/internal/book/explore_capacity_test.go`, `backend/internal/book/explore_compat_test.go`, `backend/internal/book/explore_control.go`, `backend/internal/book/explore_page.go`, `backend/internal/book/explore_session.go`, `backend/internal/book/search.go`.
- **Watch out**: Missing selectors may fall through `||`, but parse/JS/context failures must remain fatal in strict Explore fields; runtime-pool optimizations must never reintroduce globals across source sessions.
### [2026-07-18] Progress could reset or regress across Reader lifetimes
- **Problem**: Missing JSON fields silently meant chapter 0/position 0, the intended app scroll container had no bounded height, and component-local asynchronous writes could finish after a newly mounted Reader loaded and re-saved stale state.
- **Fix**: Required pointer-valued progress fields with book/readable-chapter validation, bounded the app shell, added normalized scroll helpers, queued writes per book in a shared module, awaited pending unmount writes before loading, and restored after layout using tick plus animation frame.
- **Affected**: `backend/internal/api/server.go`, `backend/internal/api/progress_test.go`, `backend/internal/book/store.go`, `backend/internal/book/store_progress_test.go`, `frontend/src/App.svelte`, `frontend/src/api/client.ts`, `frontend/src/lib/BookDetail.svelte`, `frontend/src/lib/Bookshelf.svelte`, `frontend/src/lib/Reader.svelte`, `frontend/src/lib/ReaderSettings.svelte`, `frontend/src/lib/progressWriter.ts`, `frontend/src/lib/readingProgress.js`, `frontend/src/lib/readingProgress.test.mjs`.
- **Watch out**: Every Reader instance must use the shared per-book queue; source switching must not overwrite one source's state with another source's approximate chapter mapping.
### [2026-07-18] Slow source validation could overwrite newer reading state
- **Problem**: The initial switch design captured progress before upstream book/TOC requests, so a progress write during that delay could be overwritten; source URL alone also failed to reject a stale write after an A→B→A cycle.
- **Fix**: Added an idempotent monotonic `state_version`, made progress and source-switch commits optimistic and versioned, serialized client writes around returned versions, and returned 409 on stale state. Replaced planned per-source histories with user-approved canonical title/index migration.
- **Affected**: `backend/internal/api/server.go`, `backend/internal/api/progress_test.go`, `backend/internal/api/source_switch_test.go`, `backend/internal/book/store.go`, `backend/internal/book/store_progress_test.go`, `backend/internal/book/source_switch.go`, `backend/internal/book/source_switch_test.go`, `frontend/src/api/client.ts`, `frontend/src/lib/BookDetail.svelte`, `frontend/src/lib/Reader.svelte`, `frontend/src/lib/progressWriter.ts`, `frontend/src/lib/readingProgress.js`.
- **Watch out**: Every state-mutating reader endpoint must increment and validate `state_version`; approximate index fallback must remain visible to the user rather than masquerading as an exact title match.
### [2026-07-18] Bookmark annotations could drift to the wrong source chapter
- **Problem**: Approximate index migration would silently attach user notes to unrelated text after a source change, while stale bookmark capture could race the switch that replaced the TOC.
- **Fix**: Stored book-scoped annotated locations with idempotent IDs and source/version guards, migrated only normalized-title matches inside the source-switch transaction, retained unmatched titles as visible orphans, and added explicit-position Reader links. Raw request UTF-8 is validated before strict JSON decoding and notes are bounded to 1,000 runes.
- **Affected**: `backend/internal/api/bookmarks.go`, `backend/internal/api/bookmarks_test.go`, `backend/internal/api/server.go`, `backend/internal/api/source_switch_test.go`, `backend/internal/book/bookmark.go`, `backend/internal/book/bookmark_test.go`, `backend/internal/book/source_switch.go`, `backend/internal/book/store.go`, `frontend/src/App.svelte`, `frontend/src/api/client.ts`, `frontend/src/lib/Reader.svelte`, `frontend/src/lib/ReaderBookmarks.svelte`, `frontend/src/lib/progressWriter.ts`.
- **Watch out**: Orphan bookmarks must never use approximate index fallback; future bookmark editing must preserve ID idempotency and the same UTF-8/note limits.
### [2026-07-18] Late and empty upstream reads could corrupt offline cache
- **Problem**: A slow read could recreate cache after its book was deleted or switched, and a 200 response with empty extraction could overwrite a valid copy with processor placeholder text.
- **Fix**: Made cache upserts conditional on the still-active book/source inside the retention transaction, treated empty extraction as content failure, and fell back only to exact book/source/raw-index/chapter-URL processed copies. Added 100/book and 500/global LRU caps plus an explicit offline response flag.
- **Affected**: `backend/internal/api/chapter_cache.go`, `backend/internal/api/chapter_cache_test.go`, `backend/internal/api/server.go`, `backend/internal/book/chapter_cache.go`, `backend/internal/book/chapter_cache_test.go`, `backend/internal/book/store.go`, `frontend/src/api/client.ts`, `frontend/src/lib/Reader.svelte`.
- **Watch out**: Cache identity must remain source- and chapter-URL-specific; browser-disconnected PWA behavior and batch book downloads remain out of scope.
### [2026-07-18] Broad live Explore compatibility was below deterministic coverage
- **Problem**: A stratified 50-source live sample found only 20 credible non-empty passes; shared URL-filter ordering, Default-mode detection, connector/range, JSONPath/interpolation, and Java-bridge gaps caused 18 failures, while stale rules and upstream failures caused 8 more.
- **Fix**: Captured engine, response, direct Playwright, and 390px UI evidence; classified all 50 by stable raw index/URL and ranked shared fixture-driven fixes without changing production behavior.
- **Affected**: `PLAN.md`, `testdata/booksource/explore-live-audit-2026-07-18.md`.
- **Watch out**: Live outcomes are time-dependent; reruns must use the pinned corpus/seed, keep engine gaps separate from WAF/stale rules, and never introduce source-specific parser exceptions.
### [2026-07-19] Relative URL filtering and Default detection hid valid Explore books
- **Problem**: Absolute `bookUrlPattern` checks discarded relative links before URL resolution, while standalone `class.*`/`id.*` and shorthand traversal rules such as `#j@li` were routed as CSS, producing empty or concatenated results.
- **Fix**: Resolve extracted URLs before pattern checks and detect only common HTML element traversal after shorthand CSS parents as Default mode, preserving CSS attribute getters. Added captured-shape regressions and reran the pinned 50-source manifest plus mobile Playwright.
- **Affected**: `backend/internal/analyzer/ruleparser.go`, `backend/internal/analyzer/default_live_compat_test.go`, `backend/internal/book/search.go`, `backend/internal/book/explore_url_pattern_test.go`, `testdata/booksource/explore-live-audit-2026-07-18.json`, `testdata/booksource/explore-live-audit-2026-07-18.md`, `testdata/booksource/explore-live-audit-priority-fix-rerun-2026-07-19.json`.
- **Watch out**: Raw 897 still needs separate `!0` exclusion semantics; raw 576 is a stale trailing-slash pattern, not evidence that URL resolution remains broken.
### [2026-07-19] Second Explore sample exposes recurring rule families
- **Problem**: A new disjoint 50-source sample produced empty, concatenated, or fatal results for valid upstream pages using mixed-mode fallbacks, `@tag`/`@.class` traversal, `!index` exclusion, single-brace interpolation, empty connector branches, optional JSONPath-to-JS defaults, and Java helpers.
- **Fix**: Recorded engine requests/responses, reproduced upstream behavior with Playwright, separated 14 engine gaps from 17 stale/auth/upstream failures and one legitimate empty response, and ranked shared fix candidates without changing parser or source behavior.
- **Affected**: `testdata/booksource/explore-live-audit-v2-2026-07-19.json`, `testdata/booksource/explore-live-audit-v2-2026-07-19.md`, `PLAN.md`.
- **Watch out**: The stratified sample intentionally overrepresents uncommon rule shapes; do not treat 18/50 as a corpus-wide pass-rate estimate or patch named sources individually.
### [2026-07-19] Template connector fixes exposed nested replacement ambiguity
- **Problem**: Correct double-brace connector depth exposed that outer `##` parsing could consume replacement syntax inside `{{...}}`, while pinned `<js>##regex</js>` wrappers had relied on accidental parsing and did not apply implicit empty replacement correctly.
- **Fix**: Detect replacement tokens only at rule level, evaluate wrapped `##regex` generically as regex mode, and define a missing replacement as empty. Added inner-template, mixed-template, connector, and pinned Explore regressions.
- **Affected**: `backend/internal/analyzer/analyzer.go`, `backend/internal/analyzer/ruleparser.go`, `backend/internal/analyzer/rulevars.go`, `backend/internal/analyzer/modes_regex.go`, `backend/internal/analyzer/rule_template_test.go`.
- **Watch out**: Keep `{{...}}`, `{$...}`, and wrapped regex boundaries distinct; changing scanner depth or literal classification requires running pinned JSON Explore tests.
### [2026-07-19] Live rerun exposed transformed fallback and JSoup enumeration gaps
- **Problem**: After AJAX POST support worked, raw 920 still lost the transformed JSON across `||` branches and formatted ID arrays incorrectly; after `toast` worked, raw 209 iterated enumerable JSoup helper properties as if they were elements.
- **Fix**: Preserve a leading JavaScript transform across element fallback branches, join JSON list strings with newlines, honor Legado's `@@` forced Default prefix, and expose JSoup selections as array-like objects with non-enumerable helpers plus preserved HTML export.
- **Affected**: `backend/internal/analyzer/analyzer.go`, `backend/internal/analyzer/js.go`, `backend/internal/analyzer/modes_json.go`, `backend/internal/analyzer/ruleparser.go`, `backend/internal/analyzer/connectors_conformance_test.go`, `backend/internal/analyzer/js_helpers_conformance_test.go`, `backend/internal/analyzer/rule_template_test.go`.
- **Watch out**: Shared-prefix evaluation must transform once then re-detect branch modes from the transformed content; JSoup objects must enumerate only numeric elements without losing `__html` when exported.
