# Legado Compatibility Task Tracker

> Created from the verified second-pass compatibility audit on 2026-08-01.
>
> This document preserves the implementation queue and the audit's important qualifications. It is a task tracker, not a claim that every item should be implemented with equal urgency. Work should proceed one focused TDD slice at a time, using current vendored Legado behavior in `reference/legado` as the semantic source of truth. An offline snapshot of the important booksource authoring tutorial is indexed at [`docs/legado-reference/README.md`](legado-reference/README.md).

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

- **Status:** Next
- **Priority:** High
- **Important qualification:** Ordinary Cascadia/GoQuery CSS, `@text`, `@html`, and arbitrary attributes already work. CSS support is partial, not absent.
- **Verified gaps:**
  - Jsoup-only selectors such as `:eq`, `:lt`, `:gt`, and other extensions.
  - `textNodes`.
  - Correct `ownText` behavior in explicit CSS mode.
  - Distinct `html`, outer HTML/`all`, and getter behavior.
  - Correct script/style handling for HTML getters.
  - Current `@html` returns inner HTML; `html` and `all` behave identically.
- **Approach:** Split into small getter/selector slices rather than replacing the analyzer wholesale.
- **TDD seam:** Analyzer public rule methods with exact upstream-shaped fixtures.
- **Do not:** Describe common CSS as unsupported or replace GoQuery without evidence that a focused compatibility layer cannot solve the selected slice.

### LC-008 — Execute `ruleToc.preUpdateJs`

- **Status:** Queued
- **Priority:** High
- **Impact:** Approximately six bundled sources actively use it.
- **Audit finding:** NovelReader does not run the script before TOC refresh.
- **Required behavior:** Match upstream ordering and permitted effects, including state mutation and any supported TOC URL/book refresh behavior.
- **TDD seam:** Public TOC workflow, proving execution order and one meaningful state/URL mutation.
- **Risk:** Avoid recursive refresh or hidden network retries; confirm upstream boundaries first.

### LC-009 — Execute `ruleToc.formatJs`

- **Status:** Queued
- **Priority:** Medium
- **Corpus note:** No active non-empty use was found in the pinned corpus, but the field is part of the current source contract.
- **Audit finding:** NovelReader extracts chapter names but does not run the chapter formatting script.
- **Required behavior:** Apply it after chapter construction in the same context and order as Legado.
- **TDD seam:** Public TOC parsing with a title transformation and chapter context assertion.

### LC-010 — Support content-rule `webJs`

- **Status:** Queued
- **Priority:** High
- **Important qualification:** URL-option `{ "webView": true, "webJs": "..." }` rendering already works through the browser worker.
- **Audit finding:** The distinct `ruleContent.webJs` contract is not executed.
- **TDD seam:** Public content workflow with an injected browser transport fixture.
- **Scope boundary:** Browser-side Legado bridge injection and `java.webView*` methods are separate tasks; do not claim them from content-rule script support alone.

### LC-011 — Support content `sourceRegex` and resource sniffing

- **Status:** Queued
- **Priority:** High for media sources
- **Audit finding:** Resource/media sniffing semantics and `ruleContent.sourceRegex` are unsupported.
- **Implementation checkpoint:** Confirm which source types and response/resource events consume this field before choosing a transport interface.
- **Risk:** Likely crosses browser networking, media domain models, and UI playback. Keep any initial implementation explicitly bounded.

### LC-012 — Support `payAction`

- **Status:** Design needed
- **Priority:** High for paid/VIP sources
- **Impact:** Approximately five bundled sources use it.
- **Audit finding:** Purchase actions cannot run.
- **Required planning:** Define explicit user consent, idempotency, failure reporting, credential/payment safety, and whether the current application permits transactional source actions.
- **Do not:** Execute purchase scripts automatically during ordinary content fetch.

### LC-013 — Execute content `callBackJs` and source `eventListener`

- **Status:** Design needed
- **Priority:** Medium
- **Audit finding:** Content callbacks are ignored and top-level `eventListener` has no effect. Current corpus use is low, but both belong to the active source model.
- **Required planning:** Enumerate supported events, lifecycle ordering, error policy, and side-effect boundaries before exposing callbacks.

### LC-014 — Apply `coverDecodeJs`

- **Status:** Queued
- **Priority:** Medium
- **Impact:** At least one bundled source actively uses it.
- **Audit finding:** The field is imported but never applied by a cover image fetch/decode pipeline.
- **Important distinction:** This is separate from chapter-content `imageDecode`.
- **Implementation checkpoint:** Locate or define one owned image decode boundary; do not duplicate fetch/decode policy in the frontend and backend.

### LC-015 — Complete chapter image decoding behavior

- **Status:** Queued
- **Priority:** Medium/high for image sources
- **Audit finding:** Existing image handling does not constitute complete type-2/image compatibility.
- **Implementation checkpoint:** Inventory current `imageDecode` behavior and define preserved chapter image/resource output before expanding it.

### LC-016 — Implement the automatic login lifecycle

- **Status:** Design needed
- **Priority:** High
- **Corpus impact:** Approximately 179 non-empty `loginUrl`, 17 `loginUi`, and seven `loginCheckJs` values.
- **Audit finding:** Fields survive import and may be visible as metadata, but the host does not render/execute login UI, perform login actions, check authentication, re-authenticate, or durably retain credentials/source variables.
- **Required planning:** User interaction, secret storage, multi-user isolation, expiry, re-authentication, browser/HTTP continuity, and cancellation.
- **Do not:** Equate `java.login` support with the complete host lifecycle.

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

- **Status:** Design needed
- **Priority:** Structural/security
- **Important qualification:** Workflow continuity already exists across many detail → TOC → content flows.
- **Verified limitations:** State is in bounded memory, expires after idle TTL, is lost on restart, may be capacity-evicted, is generally workflow/book scoped rather than durable source/user scoped, and does not represent multi-user isolation in session keys.
- **Required planning:** Persistence ownership, encryption/secret handling, source/user keys, eviction, migration, logout/deletion, and compatibility with disabled cookie jars.

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
