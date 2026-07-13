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
  webview/      optional browser transport implementation (new seam)
  book/         search, enrichment, TOC, content domain workflows
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

- [ ] Create fixture corpus for raw search, detail, TOC, content, JSON, XPath, Regex, JS, POST/GBK, cookie, pagination, and WebView-option sources.
- [ ] Build a source identity tool keyed by raw `bookSourceUrl` plus JSON index/hash, never name alone.
- [ ] Add a conformance runner that records raw source JSON, expanded request, method/body/headers, response status/final URL/body sample, rule field, extracted values, and classification.
- [ ] Add golden tests for each known regression from `test_booksource4.json`.
- [ ] Define expected categories: transport failure, HTTP/WAF, legitimate zero results, rule mismatch, JS failure, unsupported WebView, and successful extraction.
- [ ] Verify the server remains alive during the test; abort a run on process crash instead of continuing with contaminated results.

Completion gate: deterministic tests can distinguish a broken request from a broken selector and reproduce a raw Legado request independently.

### Phase 1 — Lossless source model and unified request executor

**Goal:** every workflow uses Legado-equivalent URL execution.

Tasks:

- [ ] Preserve unknown BookSource fields and losslessly export imported JSON.
- [ ] Implement `RequestSpec`, structured `Response`, and `SourceSession`.
- [ ] Move URL template expansion, JSON options, `<,>` page syntax, relative URL resolution, URL-level JS, body JS, charset, headers, retry, redirect metadata, and method/body construction into `sourceexec`.
- [ ] Apply URL options to search, book info, TOC, content, `nextTocUrl`, `nextContentUrl`, and JS bridge requests.
- [ ] Use RFC-compatible URL resolution against the actual response/found page URL.
- [ ] Preserve response bodies for non-2xx responses and classify rather than discard them.
- [ ] Add request-parity diagnostics for enrichment: record exact expanded URL, method, headers, redirect chain, status, and body when NovelReader differs from curl/browser.
- [ ] Reproduce and resolve the `m.22biqu.net` enrichment discrepancy where curl returned 200 but NovelReader observed 404.
- [ ] Implement per-source/per-user cookie policy and source-variable persistence.
- [ ] Ensure `Referer`, `Origin`, content type, custom headers, and cookies are forwarded exactly as the source requests them.
- [ ] Implement `bodyJs`, `webJs` dispatch metadata, and response-body transformation hooks.

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

- [ ] Enrich book info with `bookInfoInit` JavaScript/regex behavior.
- [ ] Parse TOC with correct documented reversal semantics, volumes, VIP flags, timestamps, relative URLs, and duplicate handling.
- [ ] Implement TOC pagination with cycle detection, request options, retries, and explicit partial-failure reporting.
- [ ] Implement `nextContentUrl` pagination and content concatenation.
- [ ] Bind complete book/chapter context for every rule evaluation.
- [ ] Preserve source/final URLs for subsequent relative content requests.
- [ ] Make content extraction use the declared rule first; keep SPA/script fallback as an explicit diagnostic fallback, never as a replacement for rule correctness.
- [ ] Add full E2E tests using raw source entries: search actual book, add to shelf, enrich, load TOC, open first/middle/last chapter, and verify non-empty content.

Completion gate: at least one normal HTML source, one JSON source, one XPath/Regex source, one POST/charset source, and one multi-page TOC/content source pass end-to-end.

### Phase 5 — WebView transport with minimal refactor

**Goal:** support WebView sources through the existing request contract.

Tasks:

- [ ] Define `WebViewTransport` behind the same `RequestSpec`/`Response` interface.
- [ ] Add a browser-backed implementation only after HTTP and rule conformance are stable.
- [ ] Support page JavaScript, `webJs`, delayed execution, cookies, redirects, headers, and final DOM/response capture.
- [ ] Add a configurable browser lifecycle, timeout, concurrency limit, and cancellation policy.
- [ ] Make WebView optional at build/deploy time; HTTP-only deployments report a precise unsupported capability.
- [ ] Add a fake WebView transport for deterministic tests.

Completion gate: a source marked `webView:true` can run through the same search/detail/TOC/content workflow without book-domain changes.

### Phase 6 — Diagnostics, frontend extensibility, and operational hardening

Tasks:

- [ ] Add source debug API: preview request, headers/body (redacted), response metadata, rule-by-rule extraction, and JS logs.
- [ ] Add source health history without labeling failures as permanently outdated.
- [ ] Add explore/discovery using the same executor and rule engine.
- [ ] Add source editor/import validation and raw JSON round-trip preview.
- [ ] Add frontend source-debug, source health, and crawl-progress components using typed API contracts.
- [ ] Add reading progress, bookmarks, offline cache, and alternate-source switching without coupling them to source parsing.
- [ ] Add Docker multi-stage build and clean-checkout E2E verification.

Completion gate: frontend features consume stable domain APIs; no frontend code knows CSS/Default/JS source syntax.

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

**Phase:** Phase 0 — compatibility baseline and harness; Phase 1 request-contract slice started.

**Last completed:** Routed search, book-info, TOC, and chapter content through `sourceexec.Executor`; content supports URL options, `nextContentUrl`, and Legado’s next-TOC-chapter stop condition; TOC pagination reports failures; explicit mode prefixes, standalone Regex, `###`, `&&`, `%%`, Analyzer-backed Java helpers, scoped sessions, and Default indexed selectors have conformance tests. Added Legado-compatible HTTP retry on unsuccessful responses, explicit response-charset decoding, multi-class Default selectors, chainable Jsoup selections, JavaScript-returned URL-option parsing, declaration scoping for pooled runtimes, redirect-preserving `java.get().header()`, fingerprint-first regular source transport with normal fallback, source-scoped cookie continuity, segmented URL `@js`, and POST-body page selectors. Full Go tests pass. Fresh raw-compilation Playwright verified `八叉书库` after regular transport integration: search result, book detail, 1-chapter TOC, and 352 rendered readable paragraphs. `笔趣小说` independently produced 12 search results before a later upstream DNS/503 outage.

**In progress:** Continuing cross-source compatibility checks and auditing the remaining Phase 1 request-contract gaps.

**Next action:** Re-run the raw `笔趣小说` two-step flow when its upstream endpoint is available, then verify another JavaScript/POST source before expanding WebView support.

**Environment notes:** `reference/legado` is the local upstream reference. `test_booksource4.json` is raw test input and must be sampled by stable URL/index identity, never source name alone. Existing server processes must be stopped before live E2E tests.

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Compatibility boundary | Documented Legado behavior first | Prevents source-specific patches and accidental semantic drift. |
| Request architecture | One SourceExecutor for every workflow | Search-only URL support caused confirmed failures. |
| Transport | HTTP and WebView behind one interface | Enables plug-in WebView without rewriting book workflows. |
| Source identity | `bookSourceUrl`, not name | Names are duplicated in compilations. |
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
