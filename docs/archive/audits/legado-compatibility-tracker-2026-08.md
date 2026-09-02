# Legado Compatibility Task Tracker — 2026-08 Audit Snapshot

> **Archived historical tracker.** Statuses and priorities reflect the 2026-08 audit sequence and are not a current implementation queue. Use [`docs/roadmaps/legado-compatibility.md`](../../roadmaps/legado-compatibility.md), current tests, and fresh audit evidence for future work.

> Created from the verified second-pass compatibility audit on 2026-08-01.
>
> This document preserves the implementation queue and the audit's important qualifications. It is a task tracker, not a claim that every item should be implemented with equal urgency. Work should proceed one focused TDD slice at a time, using current vendored Legado behavior in `reference/legado` as the semantic source of truth. An offline snapshot of the important booksource authoring tutorial is indexed at [`docs/reference/legado/README.md`](../../reference/legado/README.md).

## Status legend

- **Done** — implemented and committed.
- **Next** — next focused implementation slice.
- **Queued** — verified compatibility work that can be implemented incrementally.
- **Design needed** — broad, stateful, UI-dependent, media-specific, or otherwise expensive-to-reverse work that needs a scoped decision before coding.
- **Preserve/classify** — intentionally preserve metadata and document fallback behavior; do not invent unsupported Android semantics.

## Working rules

1. Add a focused regression through the nearest public production seam.
2. Confirm the regression fails for the intended compatibility mismatch.
3. Make the smallest complete implementation change.
4. Run only the targeted test and nearest affected package tests unless failures justify expanding.
5. Update this tracker and `PLAN.md` after each completed slice.
6. Make one atomic commit per field or tightly related field group.
7. Verify uncertain behavior against current vendored Legado code before encoding semantics.

## Implementation queue

### LC-001 — Apply `ruleContent.replaceRegex`

- **Status:** Done
- **Priority:** Very high
- **Impact:** Approximately 198 bundled sources define it.
- **Required behavior:** Apply the top-level content replacement rule once after all `nextContentUrl` pages are aggregated. This is distinct from inline `##pattern##replacement` syntax in the content extraction rule.
- **Verification:** Public `Searcher.GetChapterContent` regression and focused chapter-content tests.
- **Commit:** `9356ea5 fix: apply content replacement rules`

### LC-002 — Preserve Search/Explore `wordCount` and `updateTime`

- **Status:** Done
- **Priority:** Very high
- **Impact:** Approximately 250 search and 170 Explore `wordCount` rules were found; both fields already existed in the result contract.
- **Required behavior:** Evaluate both fields in the shared ordered result-field loop so later JavaScript fields can read earlier values from mutable book context.
- **Verification:** Public `Searcher.ParseSearchResult` regression and focused Search/Explore parser tests.
- **Commit:** `f0b461f fix: preserve search list metadata`

### LC-003 — Preserve `ruleSearch.checkKeyWord` for source validation

- **Status:** Preserve/classify — no ordinary-search runtime change required
- **Priority:** Low until NovelReader adds a source-validation workflow
- **Impact:** Approximately 77 bundled sources define it; raw BookSource JSON preservation already retains it.
- **Corrected finding:** Current Legado does not use `checkKeyWord` to override an ordinary user search. `BookSource.getCheckKeyword(default)` is called by `CheckSourceService` only when validating source health. The source-debug UI also presents it as a suggested query, but the submitted query remains user-selected.
- **NovelReader contract:** Keep the field preserved inside `ruleSearch`. If a dedicated source-validation feature is added, use a non-blank value unless it contains `http`, `::`, `++`, or `--`; otherwise use that validator's configured default keyword, matching current upstream `getCheckKeyword` behavior.
- **Do not:** Substitute it in `Searcher.searchSource`; doing so would silently replace real user queries and diverge from Legado.
- **Evidence:** `reference/legado/app/src/main/java/io/legado/app/data/entities/BookSource.kt` and `reference/legado/app/src/main/java/io/legado/app/service/CheckSourceService.kt`.

### LC-004 — Respect `enabledCookieJar`

- **Status:** Done
- **Priority:** Very high
- **Corrected default:** Current Legado `BookSource.enabledCookieJar` defaults to true. NovelReader therefore treats an omitted value as enabled and an explicit false as disabled.
- **Implemented behavior:** Automatic response-cookie capture is gated centrally on `SourceSession` across normal HTTP, JavaScript HTTP adapters, fingerprint transport, and WebView transport. Explicit source-script cookie operations and explicit Cookie headers remain available when automatic capture is disabled.
- **Verification:** Public detail → TOC workflow covers omitted/default-enabled and explicit-false behavior; focused nearby transport/session tests pass.
- **Commit:** `fix: respect source cookie jar policy`

### LC-005 — Handle `ruleBookInfo.canReName`

- **Status:** Done
- **Priority:** High
- **Verified upstream contract:** `canReName` is a nullable string presence flag. A nonblank value permits detail rules to replace an existing name and author when the caller allows renaming; without it, detail parsing only fills empty identity fields.
- **Implemented behavior:** Added `GetBookInfoForBook` for enrichment against an existing book. Name and author are preserved unless `canReName` is nonblank, while all other detail fields continue to update. The legacy URL-only method starts from an empty book and remains compatible. Add-book enrichment and source switching now pass their known identity into this workflow.
- **Verification:** Public book-info regression covers absent/preserve, nonblank/replace, and absent/fill-empty behavior. Full `internal/book` and `internal/api` package tests pass.
- **Commit:** `fix: respect book detail rename policy`

### LC-006 — Extract and expose `ruleBookInfo.downloadUrls`

- **Status:** Done
- **Priority:** High
- **Impact:** Approximately 23 bundled sources define it.
- **Verified upstream contract:** For `bookSourceType: 3` file sources, detail parsing evaluates `downloadUrls` as a URL list and fails when no links resolve. Legado keeps the resulting list transient rather than persisting it on the shelf record.
- **Implemented behavior:** `Book.downloadUrls` is exposed as a typed transient `[]string`/`string[]` API field. File-source detail parsing resolves every extracted link against the final response URL and rejects an empty result. Ordinary source types retain TOC parsing and do not populate download URLs. Download/import execution remains intentionally deferred to LC-019.
- **Verification:** Public book-info coverage checks multiple relative links, ordinary-source exclusion, and the required empty-link failure. Full `internal/book`, `internal/api`, and `internal/conformance` tests pass; the frontend production build succeeds.
- **Commit:** `feat: expose file source download URLs`

### LC-007 — Complete explicit CSS/Jsoup compatibility

- **Status:** Complete for audited pinned-corpus contracts
- **Priority:** High
- **Important qualification:** Ordinary Cascadia/GoQuery CSS, `@text`, `@html`, and arbitrary attributes already work. CSS support is partial, not absent.
- **Scope qualification:** This closes the explicit-CSS gaps verified in the pinned corpus; it does not claim universal support for every selector added by future Jsoup releases.
- **Completed slice:** Explicit `@css:...@ownText` now uses Jsoup-compatible direct-child text semantics for both string and list extraction, skips empty results, and preserves selection order. Full Analyzer package tests pass. Commit: `fix: support explicit CSS ownText`.
- **Completed slice:** Explicit `@css:...@textNodes` now extracts only direct text nodes, trims and removes blanks, joins nodes within each selected element with newlines, and preserves element order for string/list extraction. Full Analyzer package tests pass. Commit: `fix: support explicit CSS textNodes`.
- **Completed slice:** Explicit `@css:...@html` and `@all` now return the selected elements' aggregate outer HTML as one value. `html` removes selected script/style descendants first; `all` preserves them. Full Analyzer package tests pass. Commit: `fix: distinguish explicit CSS html and all`.
- **Completed slice:** Jsoup's zero-based sibling-index selectors `:eq(n)`, `:lt(n)`, and `:gt(n)` now translate to equivalent standard `:nth-child(...)` selectors across string, list, and element extraction. Translation works at intermediate selector positions, counts all element siblings, and leaves quoted attribute values unchanged. Corpus inventory confirms the other active explicit-CSS pseudo families are already handled by Cascadia. Full Analyzer package tests pass. Commit: `fix: support Jsoup positional selectors`.
- **Approach:** Split into small getter/selector slices rather than replacing the analyzer wholesale.
- **TDD seam:** Analyzer public rule methods with exact upstream-shaped fixtures.
- **Do not:** Describe common CSS as unsupported or replace GoQuery without evidence that a focused compatibility layer cannot solve the selected slice.

### LC-008 — Execute `ruleToc.preUpdateJs`

- **Status:** Complete for the audited upstream contract
- **Priority:** High
- **Impact:** Approximately six bundled sources actively use it.
- **Completed slices:** `preUpdateJs` executes before the first TOC request with source state and typed book context. Direct mutations synchronize back to the domain book, and a mutated `book.tocUrl`/`book.bookUrl` selects the request target. Workflow-scoped `java.refreshTocUrl()` runs the existing detail workflow and uses the refreshed TOC URL. Workflow-scoped `java.reGetBook()` performs uncapped exact name+author search, replaces search-result identity/state without overwriting shelf/progress fields, refreshes detail, clears stale TOC state when necessary, and preserves source cookies/variables through detail, TOC, and content. Both network bridges are queued during JavaScript evaluation and executed after the runtime is returned, preventing pool-size-one deadlock. Failures stop before stale TOC I/O with contextual errors. Targeted Analyzer/sourceexec/book/API/conformance tests pass. Commits: `feat: run TOC pre-update scripts`, `feat: refresh TOC URLs from pre-update scripts`, `feat: re-fetch books from TOC pre-update scripts`.
- **Scope note:** The pinned corpus actively uses `refreshTocUrl()` but contains no active `reGetBook()` call; deterministic regressions pin the upstream `reGetBook()` contract without a live-source repair claim. Direct mutations remain limited to fields represented by NovelReader's typed `Book` contract.
- **Required behavior:** Match upstream ordering and permitted effects, including state mutation and supported TOC URL/book refresh behavior.
- **TDD seam:** Public TOC workflow, proving execution order and one meaningful state/URL mutation.
- **Risk:** Avoid recursive refresh or hidden network retries; confirm upstream boundaries first.

### LC-009 — Execute `ruleToc.formatJs`

- **Status:** Complete for the audited upstream contract
- **Priority:** Medium
- **Corpus note:** One active non-empty use was found in the pinned corpus: removing a trailing six-digit marker from chapter titles.
- **Implemented behavior:** Runs once over the final TOC after pagination, reversal, deduplication, and zero-based index assignment. Each chapter receives persistent `gInt`, one-based `index`, mutable `chapter`, current `title`, book/source/session context, and source `jsLib`. Non-null results replace titles. Per-chapter errors are logged with the one-based index, retain mutations/title state reached before failure, and do not stop later chapters. Only the exposed chapter contract synchronizes back, preserving NovelReader-only fields.
- **Verification:** Public TOC regressions cover final-order/dedup timing, persistent `gInt`, one-based versus zero-based indices, mutable chapter state, fail-soft continuation, a one-runtime pool, and the exact active suffix-removal script. Targeted Analyzer/sourceexec/book/API/conformance tests pass. Commit: `feat: format TOC chapter titles`.
- **TDD seam:** Public TOC parsing with a title transformation and chapter context assertion.

### LC-010 — Support content-rule `webJs`

- **Status:** Complete for the audited upstream request-routing contract
- **Priority:** High
- **Important qualification:** `ruleContent.webJs` does not force browser rendering upstream. It is a fallback script only when the chapter URL already requests WebView; URL-option `{ "webView": true, "webJs": "..." }` takes precedence.
- **Implemented behavior:** Initial and paginated content requests copy the rule-level script into browser request metadata only when `WebView` is already true and URL-level `webJs` is blank. The original script, including `<js>…</js>` wrappers and regex text, is forwarded unchanged to the configured browser worker. Ordinary HTTP requests remain HTTP and do not evaluate this script in Go.
- **Verification:** Public content regressions with an injected browser transport cover initial-page fallback, paginated reuse, URL-option precedence, full active-corpus script preservation, and non-escalation without WebView. Targeted Analyzer/sourceexec/webview/book/API/conformance tests pass. Commit: `feat: route content webJs to browser requests`.
- **TDD seam:** Public content workflow with an injected browser transport fixture.
- **Scope boundary:** Browser-side Legado bridge injection and `java.webView*` methods remain separate tasks and are not claimed here.

### LC-011 — Support content `sourceRegex` and resource sniffing

- **Status:** Complete for the audited browser resource-sniffing contract
- **Priority:** High for media sources
- **Audit finding:** Upstream passes `sourceRegex` only when a chapter request already uses WebView. Its browser client applies a full regex match to loaded resource URLs and completes with the first matching URL as the response body. It is source-type agnostic and does not fetch or decode the matched resource.
- **Active corpus:** 15 configured rules across text (`bookSourceType=0`), audio (`1`), and archive/file (`2`) sources. Common patterns target `mp3`, `m4a`, `mp4`, `txt`, and `/files`; three selector-shaped values appear stale but are still preserved as regex text.
- **Implemented behavior:** Initial and paginated WebView content requests forward `sourceRegex` to protocol v2 of the browser worker. The worker observes browser request events, uses full-match semantics, and returns the first match immediately during navigation, settling, delay, or script execution. Ordinary HTTP requests are not escalated.
- **Verification:** Public content regressions cover browser-only routing, source-type-independent URL output, pagination propagation, and non-escalation. Go client tests cover protocol forwarding and rejection of older workers. Worker tests cover full matching, first-match wins, operation interruption, blank values, and invalid regexes. A real local Patchright smoke page requested two matching resources and returned the first (`/first.mp3`). Targeted Analyzer/sourceexec/webview/book/API/conformance and worker tests pass.
- **Scope boundary:** No playback model, media-body fetch, decode pipeline, UI changes, or browser bridge APIs were added.

### LC-012 — Support `payAction`

- **Status:** Intentionally deferred by product decision; unsupported
- **Priority:** High for paid/VIP sources, but blocked on an explicit transactional feature
- **Impact:** 11 configurations across the current corpus (including duplicates) define an action. Audited examples include a real paid-chapter POST, archive loan/return requests, browser-login handoffs, and generated payment URLs.
- **Upstream contract:** The reader exposes the action only from an explicit menu item for a remote chapter with `isVip=true` and `isPay!=true`, then displays a chapter-specific confirmation. The script receives authenticated source/book/chapter context. A `true` result invalidates cached content and refreshes the TOC; an absolute URL opens an interactive WebView.
- **Decision:** Keep `payAction` unsupported for now. Do not expose an execution endpoint, evaluate it during content fetch, or silently classify any action as safe.
- **Future implementation gate:** Require a user-initiated reader action and per-chapter confirmation; authenticated session scoping; no automatic retries; an in-flight deduplication/idempotency key; explicit success/failure/cancel/interactive-URL outcomes; cache/TOC refresh only after confirmed success; redacted logging; and clear disclosure that source scripts may spend funds, borrow/return items, or mutate an account. Because source rules do not reliably expose price/currency, the UI must not imply a known charge unless separately verified.
- **Revisit when:** The product intentionally adds transactional source actions and is prepared to own the API, reader UX, authentication, and safety boundaries above.

### LC-013 — Execute content `callBackJs` and source `eventListener`

- **Status:** Dormant in the audited corpus; intentionally unimplemented pending real usage
- **Priority:** Medium
- **Corpus finding:** Zero nonblank `ruleContent.callBackJs` scripts and zero sources with `eventListener=true` across the current test corpora. Explicit `false` values are configuration defaults, not active usage.
- **Upstream contract:** The callback receives one of 20 events: `clickAuthor`, `longClickAuthor`, `clickBookName`, `longClickBookName`, `clickCustomButton`, `longClickCustomButton`, `clickShareBook`, `clickClearCache`, `clickCopyBookUrl`, `clickCopyTocUrl`, `clickCopyPlayUrl`, `clickBookLabel`, `longClickBookLabel`, `addBookShelf`, `delBookShelf`, `saveRead`, `startRead`, `endRead`, `startShelfRefresh`, or `endShelfRefresh`. Bindings vary by event and include `event`, `result`, `book`, `chapter`, and for interactive button callbacks an authenticated `java` bridge. A truthy button result suppresses the default UI action; errors are logged and lifecycle callbacks run asynchronously with upstream 30/60-second bounds.
- **Decision:** Do not add a speculative partial callback/event system with no active source to validate it. Revisit when a real imported source enables the listener or product requirements explicitly need source lifecycle hooks.
- **Future implementation gate:** Define the supported event subset and exact ordering at owned reader/shelf boundaries; preserve default-action suppression only for interactive callbacks; scope bridge capabilities by event; bound execution; isolate callback failures from core reading/state transitions; and add fixtures from actual active scripts before enabling it.

### LC-014 — Apply `coverDecodeJs`

- **Status:** Complete for stored books; search and Explore previews remain out of scope
- **Priority:** Medium
- **Impact:** One current source actively defines `coverDecodeJs` (`斋书苑` in `test-booksources/test_booksource4.json`).
- **Implemented boundary:** `GET /api/books/{id}/cover` derives the cover URL and source identity from the stored book—there is no arbitrary target-URL parameter. Bookshelf and book-detail images use this endpoint; search and Explore result covers remain direct browser URLs by explicit product choice.
- **Implemented behavior:** The backend evaluates source and URL-option headers, carries the existing source/book session, performs a 10 MiB bounded binary GET, evaluates `jsLib + coverDecodeJs` with byte-array `result` plus `source` and typed `book`, and returns decoded bytes with sniffed media type. Non-byte/null decoder results preserve the original bytes as upstream does. A narrow Rhino compatibility shim exposes only `Packages.java.io.ByteArrayOutputStream` and `InputStream`, required to initialize the active obfuscated library.
- **Verification:** API regressions cover source headers, URL-scoped headers, byte transforms, null-result fallback, and stored-identity-only lookup. The exact active `jsLib + coverDecodeJs` now initializes and runs with byte input. Analyzer/fetcher/book/API/conformance tests pass; frontend 13 tests and production build pass.
- **Important distinction:** This is separate from chapter-content `imageDecode` (LC-015). It does not add a general image proxy/cache, search/Explore decoding, WebView cover requests, or non-GET cover fetches.

### LC-015 — Complete chapter image decoding behavior

- **Status:** Portable subset complete; Android bitmap transforms explicitly unsupported
- **Priority:** Medium/high for image sources
- **Corpus finding:** Six active rules split into four portable byte decoders (AES or the obfuscated `decode(result)` family) and two scripts importing Android `BitmapFactory`/`Canvas`/`Matrix` for image rotation or slice reordering.
- **Implemented content contract:** Chapter responses retain legacy `paragraphs` and add ordered `blocks` containing text or server-assigned image indices. The cache persists both shapes, and the reader renders image blocks through `GET /api/books/{id}/chapters/{idx}/images/{imageIdx}`.
- **Implemented image boundary:** The endpoint accepts no target URL. It resolves the exact stored book/source/chapter and cached image index, resolves relative URLs against the chapter page, applies source and URL-option headers with the existing session, performs a 10 MiB bounded binary GET, supplies byte `result` and resolved URL `src`, runs executable `jsLib + imageDecode`, requires a byte result, and returns sniffed binary content.
- **Explicit limitation:** Scripts referencing Android bitmap APIs return HTTP 501 with `chapter_image_decoder_unsupported`; NovelReader does not silently return scrambled/encrypted bytes. JSON remote-library maps in `jsLib` are metadata and are not executed as JavaScript; the active portable scripts using them rely only on built-in bridges.
- **Verification:** Processor ordering/text-parity, current-schema cache persistence, fresh/offline API shape, URL redaction, stored identity/index lookup, source and URL-option headers, relative resolution, `src` binding, byte transform, and Android rejection are covered. The active portable scripts parse/initialize against the current bridge; data-dependent AES/obfuscated fixtures are qualified separately from syntax/bridge failures.

### LC-016 — Implement the automatic login lifecycle

- **Status:** Blocked on accepted application-authentication prerequisite
- **Priority:** High
- **Updated corpus impact:** Combined local source corpora contain roughly 296 non-empty `loginUrl`, 47 `loginUi`, and 21 `loginCheckJs` values. Most login URLs are ordinary web pages; source-defined UIs often execute privileged scripts, and many check scripts are browser/WAF challenge handlers rather than account checks.
- **Audit finding:** Fields survive import, in-memory source sessions can carry cookies/headers, and the Java bridge exposes login-header helpers, but `java.login` remains a stub. There is no application identity, durable credential/cookie store, interactive browser handoff, source login UI, authentication check/retry lifecycle, or user isolation.
- **Accepted prerequisite:** `docs/AUTHENTICATION_DESIGN.md` defines local Reader Accounts, a plaintext self-contained reader directory/database per immutable user ID, user-keyed live Source Sessions, and a separate encrypted credential store. Global durable login state is forbidden because it would share source accounts across readers.
- **Portability constraint:** Source credentials are excluded from portable reader exports, and loss of `NOVELREADER_SECRET_KEY` must not affect sources, shelf, progress/history, bookmarks, caches, preferences, fonts, or other Reader Data.
- **First login slice after prerequisite:** Manual per-user cookie/login-header import, encrypted with `NOVELREADER_SECRET_KEY`, plus status and logout. Interactive browser login and source-defined `loginUi` remain later capability/security designs.
- **Do not:** Equate `java.login` support with the complete host lifecycle or execute source credential forms before the capability sandbox is designed.

### LC-017 — Correct `java.timeFormat`

- **Status:** Queued
- **Priority:** Medium/high
- **Audit finding:** The current method returns the decimal timestamp instead of a formatted date and lacks the string overload.
- **TDD seam:** Public JavaScript bridge tests against exact upstream signatures, timezone/format behavior, and representative timestamps.

### LC-018 — Close focused JavaScript bridge gaps

- **Status:** Queued as independent slices
- **Priority:** Varies by corpus usage
- **Existing capability:** The bridge already includes useful HTTP, parser, crypto, source-state, cache, and cookie methods; it is a compatibility subset, not broadly absent.
- **Verified gaps/mismatches:**
  - `java.login` is a no-op.
  - `androidId` is a fixed placeholder.
  - `decode` is only a Base64 convenience implementation.
  - Unknown HMAC algorithms silently fall back to MD5.
  - `java.getCookie` is not exported under the documented name.
  - `java.ajaxAll` is missing.
  - Byte, hex, hash, RSA, AES convenience, file, ZIP, font, and WebView methods are incomplete.
  - `Packages`, `JavaImporter`, and arbitrary Android/Java class access are unavailable.
- **Approach:** Prioritize exact public methods demonstrated by real source usage. Add one signature/return-contract test per method family. Do not attempt arbitrary JVM/Android class emulation.

### LC-019 — Support `bookSourceType` 1–4 intentionally

- **Status:** Design needed
- **Priority:** Structural
- **Current behavior:** Normal search selects only type-0 text sources.
- **Upstream types:** `0=text`, `1=audio`, `2=image`, `3=file`, `4=video`.
- **Missing domains:**
  - Audio resources and lyrics.
  - Image chapter/resource preservation and decoding.
  - File/download URL workflows.
  - Video/resource sniffing.
  - Media-specific API and presentation behavior.
- **Required planning:** Domain models, API contracts, storage/cache behavior, frontend readers/players, and migration/compatibility boundaries. Implement per type, not as one monolithic change.

### LC-020 — Handle reviews, custom buttons, and newer source actions

- **Status:** Design needed
- **Priority:** Low/forward compatibility unless corpus evidence changes
- **Audit finding:** Current upstream includes `customButton`; reviews and custom source actions remain outside the runtime. Unknown-field preservation keeps data lossless but provides no behavior.
- **Required planning:** Define supported action/event contracts and UI before execution.

### LC-021 — Make source state durable and user-isolated

- **Status:** Portable per-user storage and encryption design accepted; implementation begins with the fail-closed `readerstore` foundation
- **Priority:** Structural/security
- **Important qualification:** Workflow continuity already exists across many detail → TOC → content flows.
- **Verified limitations:** State is in bounded memory, expires after idle TTL, is lost on restart, may be capacity-evicted, is generally workflow/book scoped rather than durable source/user scoped, and does not represent multi-user isolation in session keys.
- **Accepted planning:** `docs/AUTHENTICATION_DESIGN.md` assigns Reader Data to plaintext per-user `reader.db` directories, isolates reversible source credentials in a separate encrypted non-exportable store, keys live sessions by immutable user ID, and defines backup, key-loss, deletion, migration, and logout behavior.

## Additional verified compatibility backlog

These findings were not all in the revised top-15 ordering, but must remain tracked.

### LC-022 — Correct standalone Regex `replaceFirst` semantics

- **Status:** Queued
- **Audit correction:** “OnlyOne” is tutorial terminology; current upstream primarily calls this `replaceFirst`.
- **For `##pattern##replacement###`, Legado:** finds the first matching substring, replaces within that match, returns only the resulting matched substring, and returns empty when no match exists.
- **Current NovelReader behavior:** replaces within and returns the full surrounding input, and returns the original input when no match exists.
- **TDD seam:** Analyzer standalone Regex tests for match and no-match cases.

### LC-023 — Preserve list-oriented Regex capture groups

- **Status:** Queued
- **Audit finding:** List Regex extraction can flatten each match to capture group 1 instead of preserving each match's complete capture-group list as Legado does.
- **TDD seam:** Analyzer list extraction with multiple matches and multiple groups.

### LC-024 — Align non-2xx response-body policy across workflows

- **Status:** Queued, semantics check required
- **Current behavior:** Explore evaluates received non-2xx bodies and records a warning; Search requires exactly 200; detail/TOC/content require 2xx; transport failures still fail normally.
- **Audit finding:** The policy is workflow-specific and potentially incompatible when the same valid source technique is used outside Explore.
- **Implementation checkpoint:** Verify upstream behavior per workflow before changing shared executor policy. Preserve diagnostics and do not treat DNS/TLS/timeouts as received HTTP bodies.

### LC-025 — Default omitted top-level `enabled` correctly

- **Status:** Queued
- **Audit correction:** `enabledExplore` already defaults to true. Only omitted `enabled` was confirmed to incorrectly become Go's false instead of Legado's true.
- **TDD seam:** BookSource JSON import/default regression. Do not generalize the change to unrelated booleans.

### LC-026 — Apply per-source `respondTime` semantics

- **Status:** Queued, semantics check required
- **Audit finding:** The field is imported/stored, but request deadlines use global Searcher timeouts; per-source timeout and any source-ordering behavior are absent.
- **Implementation checkpoint:** Confirm units, default, and every upstream consumer before wiring it into request policy.

### LC-027 — Preserve structured object-shaped rules at runtime

- **Status:** Queued
- **Audit finding:** Runtime rule parsing flattens nested values to `map[string]string`, recognizes only selected wrapper keys, can discard metadata, may choose unintended fallback strings, can depend on Go map iteration, and stringifies numeric/boolean values without typed semantics.
- **Important qualification:** Raw import/export is already lossless; this task concerns runtime interpretation.
- **Approach:** Introduce typed interpretation only where real object-shaped contracts require it. Avoid a repository-wide rule representation rewrite unless focused cases prove it necessary.

### LC-028 — Classify and expose Explore style metadata intentionally

- **Status:** Preserve/classify
- **Audit finding:** Category `style` is accepted but not meaningfully returned/applied. Android fields include flex grow/shrink, alignment, basis percent, and wrap-before metadata.
- **Decision:** Either map selected fields to documented web equivalents or explicitly return/preserve them with a defined web fallback. Do not imitate Android layout behavior speculatively.

### LC-029 — Complete browser-side Legado bridge methods

- **Status:** Design needed
- **Audit finding:** URL-option WebView works, but browser-side bridge injection and `java.webView`, `java.webViewGetSource`, and `java.webViewGetOverrideUrl` are incomplete.
- **Required planning:** Browser worker protocol, page lifecycle, cancellation, security/isolation, redirect/override capture, and method return contracts.

### LC-030 — Define intentional handling for every source field

- **Status:** Queued documentation/contract work
- **Goal:** Maintain a canonical field-support matrix covering every top-level and nested current Legado field.
- **Required classifications:** `supported`, `partial`, `preserved-only`, or `unsupported`.
- **Acceptance:** No current field remains unclassified; each supported/partial field links to its implementation/test, and preserved-only fields state their fallback behavior.

### LC-031 — Support generic typed `data:` BookSource requests

- **Status:** Design analyzed; implementation not approved
- **Priority:** High for aggregated/gateway BookSources
- **Primary reference:** Use maintained successor fork `reference/legado-E` for current semantics; retain `reference/legado` as historical context.
- **Fixture:** `test-booksources/test_光遇聚合_aggregated_booksource.json` uses ordinary Legado primitives to carry Search, Detail, TOC, and Content state through `data:;base64,...,{"type":"..."}` URLs. Labels such as `gysearch` are opaque and must not receive source-specific dispatch.
- **Current gap:** NovelReader preserves URL-option `type` and builds the synthetic URL, but routes every non-WebView request into HTTP; it does not decode data bytes into the Legado-compatible hexadecimal response body. `java.hexDecodeToString`, contextual `java.get/put`, and `book.getVariable` also need focused compatibility slices.
- **Architecture:** Add a bounded generic in-memory request path behind the existing `sourceexec` request/response interface, then close exact bridge/context gaps. Do not add a GuangYu adapter or aggregate-source domain type.
- **Separate structural work:** Durable per-reader source variables/settings remain distinct from encrypted login credentials and from optional source-defined login UI, browser verification, comments, media modes, and external bookshelf synchronization.
- **Detailed analysis:** `docs/LEGADO_E_AGGREGATED_BOOKSOURCE_ANALYSIS.md`.

## Audit qualifications that must not be lost

- **CSS:** Common CSS works; explicit CSS is not fully Jsoup-compatible.
- **WebView:** URL-option WebView and `webJs` work; content-rule WebView, browser bridge, and resource sniffing remain incomplete.
- **Sessions:** Workflow continuity exists; durable source/user persistence does not match Legado.
- **Non-2xx:** Behavior differs by workflow; there is no single current parse-or-reject rule.
- **Defaults:** `enabledExplore` defaults correctly; omitted `enabled` does not.
- **Java bridge:** It is a useful subset with concrete stubs, missing methods, and signature/return mismatches—not an absent bridge.
- **Unknown fields:** Raw JSON import/export preservation does not imply runtime support.
- **Android metadata:** Preserve and classify it; do not invent executable or visual semantics without a web contract.

## Current stopping point

- Completed: LC-001 and LC-002.
- Next implementation slice: LC-003, `ruleSearch.checkKeyWord`.
- The working tree contained a pre-existing user modification to `AGENTS.md` when this tracker was created; it is unrelated and must not be included in compatibility commits.
