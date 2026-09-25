---
status: active
updated: 2026-09-26
---

# Reader-entry caching and validated snapshot reuse

## Goal

Make reopening a recently read book materially faster by reusing its catalog and prepared display, without weakening current identity validation, chapter freshness, retention, source recovery or progress correctness.

For a warm, unchanged reader entry, after outstanding progress writes drain, the blocking path should be one small authenticated validation response followed by local catalog, chapter and prepared-display reuse. A missing/changed catalog, expired chapter, cold conversion or pending progress write legitimately adds work. This is not a promise of one HTTP request for the entire application or zero-network opening.

The [live measurement report](../notes/2026-09-26-reader-reentry-latency.md) owns baseline timings, methodology and limitations. Do not duplicate its raw results here. Improvement must be measured against comparable conditions, not inferred from fewer lines of code or request counts alone.

## Scope and authorization

**Accepted direction:** lightweight authoritative entry validation, bounded persistent catalog reuse, memory-only conversion reuse across reader visits, and removing source-recovery metadata from the prose critical path. Extend existing ownership rather than build a general caching framework.

**Current authorization:** documentation and planning before application implementation. The user requested this durable tracking plan; no application changes begin in this step. Resolve the implementation gates below and obtain authorization to start coding before marking a milestone in progress.

Included:
- Reader entry, re-entry, reload and existing visible-reader revalidation.
- Catalog handoff/request sharing between Book Detail and Reader; BookSource, TXT and EPUB qualification.
- The existing client cache's memory, IndexedDB, retention and invalidation boundaries.
- Conversion ownership, source-recovery loading/error handling, and associated regression/performance checks.

Excluded:
- Stale-first display, offline opening, background synchronization, push invalidation infrastructure or a new validation TTL.
- Larger chapter TTLs/windows, whole-book downloads, image-byte caching, new backend content caches or a new revision system.
- Persisted converted text/HTML, cache settings UI, whole-app state management or unrelated ReaderView refactoring.
- Authentication/startup redesign. A full reload still authenticates before reading private state.
- Compression changes in the core milestone. They remain a separately approved complement for cold/changed catalog transfers, not a prerequisite or substitute for warm reuse.

**Risk:** implementation is structural: additive API semantics, cache-format evolution and concurrent invalidation. Keep durable reader data unchanged; use focused boundary/race tests and explicit rollback behavior.

## Accepted Approach

### Ownership and data flow

```text
Reader entry / existing resume validation
  → drain relevant ordered progress writes
  → capture reader/request/cache invalidation generations
  → obtain fresh authoritative entry qualification + current reading state
  → reuse matching catalog from the existing cache owner, or fetch/validate it
  → existing chapter loader: memory → IndexedDB → backend on miss/expiry
  → reuse/prepare display for that canonical document and conversion mode
  → restore and commit the requested location
  → existing progress writer and next-two preparation
```

Source recovery obtains its native metadata independently when needed. It must remain available on failure and source-switch paths, not merely disappear because its previous eager request was removed.

| Owner | Change/responsibility |
|---|---|
| Backend book/reading boundary | Coherent current resume/state version and cache qualification without reading/serializing the full catalog on a qualified warm hit |
| `reader-session.ts` | One entry-validation flow; compare qualification, obtain matching catalog and reject superseded work |
| Existing chapter-cache policy/coordinator/storage | Own retained catalogs alongside chapters, sharing the current reader scope, retention and transactional invalidation authority |
| Existing chapter loader | Preserve chapter lookup, Refresh, foreground joining and next-two scheduling; no second scheduler |
| Existing display converter | Reuse pending/completed conversion while canonical documents/catalogs remain retained; no separate persistent display cache |
| Reader and Book Detail views | Navigation, display/restore, source UI and recovery; consume the shared catalog path instead of owning competing cache lifetimes |
| Existing progress writer | Preserve state-version ordering and entry's pending-write barrier |

These are responsibilities, not instructions to add one class per row. Keep API/storage mechanics behind their existing cohesive owners. Rename or split a file only if the changed responsibilities actually require it.

### Entry qualification and API contract

Required semantics, regardless of final field names:
- Return current saved location and state version, plus provider, book/content revision and BookSource definition identity where applicable. Preserve authenticated reader scope and the current reader-home generation header.
- Prefer an additive extension of the existing single-book/reading boundary. Do not enrich every shelf item with expensive source-definition loading just to optimize reader entry.
- Establish book binding, interpretation and source qualification coherently. A source switch, source edit or restore during the read must not yield a hybrid response. Preserve the runtime/home lease; no database transaction may span upstream network execution.
- The warm validation path must not load all chapter rows and hash/serialize a complete catalog just to answer whether a retained one is eligible.
- Source-definition identity is separate from content revision. Do not replace it with an unsafe timestamp comparison or change progress/bookmark revisions for definition-only edits.
- Only reuse a catalog after fresh qualification matches its owner/provider/revision/source identity. Cache lookup is not permission to skip authentication, account/home validation or current resume retrieval.
- On a missing/changed catalog, keep current synchronization, retry and revision-pair checks. Bound coherence retries rather than spin under repeated changes. Missing qualification must fail closed for reuse, not be guessed from a book ID or ordinal.
- Keep existing endpoints usable by old frontends. A new frontend talking to an old backend must take the current uncached validation path when the additive qualification is absent.
- Keep reader JSON `private, no-store`; application-managed reuse remains explicit. HTTP disk caching by URL alone must not become another authority.

A remote mutation after a coherent validation point remains subject to the existing entry/resume/request guards; this work does not promise instantaneous cross-device invalidation while a page remains open.

### Catalog persistence, sharing and retention

- Store one validated canonical catalog per retained book scope, including all provider navigation/auxiliary-section data required by existing reader behavior. Do not persist mutable progress as an authoritative offline entry snapshot.
- Extend the existing disposable IndexedDB database and control epoch, not a separate independently invalidated catalog database.
- Catalog writes must check reader, captured epoch and retention eligibility in the same transaction as publication. Apply the existing late-response protection to catalog requests and writes.
- Keep a stable canonical catalog object on a memory hit so conversion memoization can actually reuse it. Do not clone/reparse matching in-memory catalogs on every view mount.
- Share matching pending catalog work across Detail and Reader, qualified by the relevant identity and invalidation generation. A detail-page request does not replace the reader's fresh validation. Do not join stale pre-mutation work just because the book ID matches.
- Retire failures and superseded requests from pending registries. Detaching one consumer must not abort still-needed shared work or allow an obsolete consumer to publish into a new reader lifetime.
- Persist catalogs only within the existing three-retained-book policy. Detail-only visits may hold their current view/request result but must not accumulate an unbounded cache or promote reading recency. Successful foreground reading remains the retention admission/recency event.
- Evict/prune a catalog and associated reusable display state when its book scope is evicted. Chapter-window movement prunes chapter-specific data but does not delete an unchanged retained catalog.
- A chapter-only Refresh invalidates that chapter's canonical/display preparation, not an unchanged catalog. Book/provider/home invalidation covers the corresponding catalogs too.
- Continue useful reading if IndexedDB is unavailable, blocked, malformed or full. Retain current memory/network fallback and bounded quota retry behavior. Persistence must not block committing otherwise valid displayed content.

Catalog validity follows interpretation/source qualification, not the chapter's 24-hour expiry. Adding a fresh catalog or validating one must never extend a chapter's expiry.

### Display preparation and source recovery

- Move converter lifetime from a mounted ReaderView to the existing reader-scoped retained-content lifetime. Keep canonical content untouched; converted output remains memory-only.
- Key reuse by the actual canonical catalog/document and conversion mode, not merely `(book, revision, index)`: Refresh can change text without changing those numbers.
- Tie conversion retention to canonical ownership. Do not retain old document generations, inactive books or arbitrary mode keys indefinitely; settle the finite supported-mode policy before implementation.
- Retain pending conversion sharing, failed-conversion eviction/retry, readable canonical fallback and existing warning behavior. Preference/generation changes must still prevent publication of a result in the wrong mode or navigation.
- Preserve next-two conversion warming through the existing loader. Do not add preparation triggered by every cache completion or by inactive books.
- Removing eager `getBookSource()` requires an explicit recovery-metadata load state: open source controls/recovery → loading → current metadata or retryable error. Guard delayed metadata against book/revision changes and unmounts.
- Source recovery must still work after catalog/content failure; source switching still drains/retires the old loader and performs authoritative invalidation. Do not require successful chapter loading to make recovery available.
- Resolve conversion-capability/engine qualification during the API design gate. Longer-lived results must not silently survive a meaningful converter configuration change. Prefer existing capability/version information; do not introduce a separate revision system. Do not add a new serial capability request to the warm critical path.

## Behaviors that must remain unchanged

| Event/condition | Required result |
|---|---|
| Source switch, catalog interpretation replacement, TXT reparse | Old scope/ordinals cannot satisfy a new entry; reject late old-scope results; preserve mapping/recovery behavior |
| Source-definition edit with unchanged content revision | Definition mismatch makes affected chapter/catalog/display reuse ineligible |
| Logout/account change, home restore/replacement, book/source removal | Existing identity retirement remains authoritative; affected persistent and memory entries cannot be resurrected |
| Other-tab invalidation while a response/write is pending | Transactional epoch guard rejects stale publication; broadcasts assist memory retirement but are not the sole safety gate |
| Invalidation unrelated to a retained book/chapter | Preserve legitimate unaffected reuse without renewing stale in-flight tickets; do not regress the retained-epoch correction |
| Chapter expiry | New load misses; validation, conversion and disk promotion never restart TTL; existing displayed prose follows its current contract |
| Explicit Refresh | Existing upstream bypass/sharing rules remain; refreshed document cannot receive an old converted result |
| Rapid navigation, unmount, visibility changes | Superseded work cannot commit; visibility revalidates authoritative identity; hidden/abandoned readers do not renew speculatively |
| Progress/bookmarks and saved resume | Preserve ordered state versions and outstanding-write drain; only committed foreground reading changes reading recency/progress |
| EPUB notes/anchors/volumes and short catalogs | Preserve navigation payloads, restoration/rollback and main-section window semantics |
| Storage failure, malformed cache or mixed client versions | Safe miss/fallback without using unqualified data, losing durable reader data or hanging the reader |

## Resource budget

- Reusable chapter copies remain bounded by **three books and five main chapters per book**. Existing backend chapter/resource limits and image-resource promises do not change.
- Additional persistent data: at most one catalog per retained book. Additional memory: retained catalog objects and their supported prepared display variants, tied to the same owners.
- No persistent conversion/HTML, whole-library metadata cache or new backend RAM layer.
- These are count bounds, not a byte ceiling. Large individual catalogs remain larger; active views, rollback snapshots and in-flight work can temporarily retain additional objects. Multiple tabs do not share one global RAM heap.
- Verify pruning through all paths, including account reset, home qualification, book/provider invalidation, retention changes and quota eviction. Report serialized storage growth and representative memory behavior using synthetic catalogs; do not present JSON byte size as JavaScript heap size.

## Decisions and implementation gates

Accepted decisions above must not be reopened merely to defer core requirements. The following engineering details are **not yet settled**; record the outcome here before editing the corresponding production boundary.

| Gate | Required resolution | Preferred constraint |
|---|---|---|
| G1 — coherent entry API | Choose exact additive response fields/endpoint and server snapshot/revalidation sequence; define old-server fallback, catalog-not-ready/mismatch behavior and capability qualification | Extend existing book/reading owners; one small warm validation; no full catalog work on a match |
| G2 — cache-format evolution | Choose versioned catalog representation and IndexedDB upgrade/old-tab/downgrade handling; define capture/read/write/retain/invalidate transaction scopes | One control epoch and retained-book registry; no durable reader schema migration |
| G3 — consumer lifetime | Specify detail-to-reader pending sharing/cancellation and transient detail-only retention; define converter/mode/engine lifetime | Reuse existing generation guards; no application-wide event framework or unlimited pending/result map |
| G4 — recovery integration | Identify exactly which native fields are required before prose display and how source controls load/retry on normal and failure paths | Do not trade away source recovery to remove one await |

Confirm with the user if resolution requires changing accepted behavior, adding a dependency, a new public route rather than the preferred additive shape, changing data retention, or expanding scope. Routine private helper choices do not need separate interviews. API/schema decisions must be recorded as chosen rather than buried in code.

## Implementation milestones

Each implementation milestone should leave affected code runnable with its focused regressions passing. Update Current State, Next Action and Verification before a significant commit or unfinished handoff, not after every edit.

- [x] **M0a — measurement and accepted direction.** Live evidence recorded; branch created; this plan establishes scope and invariants.
- [ ] **M0b — resolve G1–G4 and authorize coding.** Inspect direct callers/tests for the selected boundaries; write down the wire shape, coherence proof, storage upgrade and lifetime rules. Do not implement during the documentation-only step.
- [ ] **M1 — lightweight server qualification.** Add the selected additive contract and frontend parsing/fallback. Test coherent binding/source/home/state qualification and unchanged legacy behavior, including an invalidation race. Keep the current reader path working until integration is ready.
- [ ] **M2 — bounded catalog reuse.** Add catalog storage/memory eligibility and integrate transactional epoch/pruning rules. Test persistence/reload, retention and late-write rejection with the existing fake-indexeddb setup. Include safe blocked/older-tab/failure handling.
- [ ] **M3 — shared reader-entry flow.** Integrate fresh validation + retained/pending catalog reuse in Reader and Detail. Preserve progress drain, polling/retry and revision-qualified navigation. Remove redundant catalog work, not validation itself. Prove a warm entry does not fetch the full catalog or cached chapter.
- [ ] **M4 — reusable display and nonblocking recovery metadata.** Integrate retained conversion and lazy/noncritical source metadata with explicit loading/error/retry state. Cover Refresh, conversion-mode changes and failure/source-switch recovery. Prove warm prepared Traditional entry makes no repeated conversion POSTs.
- [ ] **M5 — integrated regression and resource/performance verification.** Run boundary/race and native-browser checks; compare warm/cold/reload behavior and retained storage/memory. Fix causes of any regressions before considering merge.
- [ ] **M6 — final documentation and delivery readiness.** Update only affected current architecture/usage docs, mark outcomes/limits here, and summarize readiness. Merge, push, hosted CI and deployment require the applicable user authorization; none follows automatically from this plan.

### Expected touchpoints

| Area | Existing entry points/tests to extend or inspect |
|---|---|
| Backend qualification | `backend/internal/api/library_reads.go`, `reader_api.go`, `reader_handlers.go`; `backend/internal/reading/service.go`, `booksource.go`; library/book/source store snapshot interfaces |
| API parsing/entry | `frontend/src/api/books.ts`, `models.ts`, `chapter-catalog.ts`, `reader.ts`; `frontend/src/features/reader/reader-session.ts` and its tests |
| Cache policy/storage | `chapter-cache-policy.ts`, `chapter-cache.ts`, `chapter-cache-storage.ts` and their existing tests |
| Reader integration | `ReaderView.vue`, `chapter-loader.ts`, `progress-writer.ts`; `ReaderView.navigation.test.ts` and chapter-loader tests |
| Detail/recovery | `frontend/src/features/books/BookDetailView.vue`, source-recovery UI and its existing tests |
| Display | `frontend/src/features/reader/chinese-conversion.ts` and conversion tests; capability API only if G1 requires it |
| Current documentation after delivery | `docs/architecture/discovery-and-reading.md`, applicable README cache behavior, and `PLAN.md` routing |

This is a routing map, not permission to edit every listed file or refactor unrelated responsibilities.

## Verification plan

### Deterministic regressions

Extend existing fixtures, fake-indexeddb and deferred-promise tests rather than duplicate the whole reader test suite. A compact set of scenarios should cover:

1. Fresh warm entry reuses matching catalog/chapter/prepared display after authoritative validation; cold or changed entry fetches the required data. Detail and Reader share only eligible pending catalog work.
2. A source switch or definition-only change during pending validation/catalog retrieval cannot display or persist the old result; cover another-tab invalidation at the transaction boundary.
3. Restore/account replacement with repeating book IDs/revisions does not reuse the old home; missing/unsupported qualification takes a safe fallback.
4. Chapter expiry still misses after catalog validation/conversion; Refresh under the same revision produces newly converted content; unrelated retained entries remain eligible without renewing in-flight tickets.
5. Fourth-book retention, chapter-window movement, removal and storage-pressure eviction prune the appropriate catalog/display/chapter entries; detail-only browsing does not expand persistent retention.
6. Conversion failure/mode change and delayed source metadata preserve readable fallback, recovery access and current-book ownership. Successful reading does not wait for optional recovery metadata or disk writes.
7. Progress write pending on departure is ordered before fresh resume binding; failed navigation, prefetch, notes and anchor rollback do not record the wrong location.
8. TXT/EPUB qualification and catalog/navigation payload round trips remain correct, including volumes/auxiliary entries. Mixed-version clients and blocked/corrupt/unavailable IndexedDB remain usable through fallback.

Start with the smallest relevant test file per milestone. As integration grows, run the affected reader/books/API frontend tests, typecheck/lint/build, affected backend reading/API/store tests and focused race tests. Use broader regression only where shared changes justify it. Record the actual commands/counts in Verification as they run; no command listed here is a passed result.

### Performance and resource acceptance

- Use synthetic deterministic browser fixtures with controlled ~150 ms request latency to assert the dependency chain and request counts. Avoid fragile wall-clock thresholds in unit CI tests.
- In a visible browser, repeat comparable warm Original and Traditional reopens, full reload and cold/invalidated entry; separate click-to-DOM readiness from position-restoration/cache commit and optional after-display work.
- Warm qualified entry: no full catalog GET, no cached current-chapter content GET and no repeated already-prepared conversion POST. One fresh small entry validation is expected; progress writes and independent background work must be identified separately.
- On reload, qualified catalog/chapter persistence should survive. Memory-only converted results need not survive; do not claim zero conversion/startup requests there.
- Demonstrate material warm latency improvement under matched conditions; the measured ~130–150 ms small-request floor is a hypothesis for the optimized critical path, not a release SLA.
- Inspect persistent row counts and serialized sizes after book/window changes and resets. Check representative conversion/catalog memory retention without exporting real user heap/content. Device quotas and normal GC variability are not a strict application byte budget.
- Optional authenticated live verification must preserve user preferences and use only sanitized timing/count data. Do not switch real sources, reparse, restore or delete user data to test invalidation; exercise those through isolated synthetic fixtures.
- If server timing remains unexplained, measure it explicitly before claiming a database/CPU improvement; browser TTFB alone cannot isolate it.

## Compatibility and rollback

- No durable reader-database migration, backup format change or archived-data rewrite is planned. Catalog persistence is disposable browser state, excluded from portable backups.
- Additive server metadata preserves old frontend requests. Missing metadata on an older backend must use the existing safe online path, not an incomplete fast path.
- An IndexedDB format upgrade must account for open old tabs, `versionchange`, blocked opens and downgrade `VersionError`. Reading must continue via memory/network when a client cannot use the disposable store. Do not delete unrelated browser storage to recover it.
- Backend rollback must not require rolling back reader data. Frontend rollback may lose cache reuse until compatible code is loaded; it must not lose books/progress or reuse unqualified entries. Record the exact selected upgrade/rollback behavior under G2 before coding.
- Keep fixes in coherent working commits; no feature flag or second permanent reader path unless compatibility actually requires it. Remove only newly superseded code and retain required old-server fallback.

## Current State

- Branch: `feat/reader-entry-cache`, based on `docs/reader-reentry-analysis` at `1451c35` (which contains the sanitized measurement report). The eventual implementation branch can be merged once; the analysis branch needs no separate merge.
- Direction accepted; detailed plan created. G1–G4 and all production milestones remain unfinished.
- No application/API/cache-format change has been made. No new performance improvement, regression pass, resource ceiling or live invalidation verification is claimed.
- User-owned untracked files are outside this work and must remain untouched.

## Next Action

Review this plan, resolve the G1–G4 engineering gates and obtain authorization to begin implementation. Start with the smallest coherent server entry-contract change and its regression tests (M1), not a broad frontend cache rewrite. Keep this plan as the single implementation handoff document as work progresses.

## Verification

Completed evidence: the linked measurement report contains live baseline method/results and limits, including observed warm chapter-cache hits, IndexedDB reuse after reload and restored profiling preferences.

Planning validation completed: dated-filename check passed; all 48 local documentation link targets across this plan, the measurement note and `PLAN.md` exist; `git diff --check` passed. Application tests/builds were not run for this documentation-only step.

Pending implementation evidence: all deterministic regressions, race checks, cache-upgrade compatibility, resource measurements and before/after performance checks described above. Replace this pending summary with concrete results and remaining limits at meaningful milestones; do not append session diaries.
