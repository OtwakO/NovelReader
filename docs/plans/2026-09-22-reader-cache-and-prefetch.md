---
status: completed
updated: 2026-09-25
---

# Low-latency reading: chapter caching and prefetch

## Goal

Make opening/reopening books and adjacent chapter navigation responsive, without repeatedly retrieving unchanged content. Show destination feedback immediately when content is not ready. Preserve reading correctness through source changes, reparse, restore and account changes.

**Simplify ownership and implementation, not the desired behavior.** Persistent client caching, backend cache-first reads, display-ready prefetch, active-reader renewal and immediate feedback all belong to this workstream. Do not defer a requirement merely to make the design smaller. Use cohesive existing modules, small interfaces and explicit identities/lifetimes rather than patches, generic frameworks or parallel implementations.

Application implementation is authorized. Deployment, data reset and schema-epoch migration are not authorized. The architectural direction is accepted; concrete resource, execution and identity contracts must still be settled before dependent implementation.

## Fresh-session entry

1. Read repository [instructions](../../AGENTS.md), the [project router](../../PLAN.md), then this plan. Check `git status` and recent history; do not assume the last session's branch or working tree is unchanged.
2. Read **Current State**, **Next Action** and **Verification** before treating any accepted behavior as implemented. The source observations below were established against implementation checkpoint `009c21b`; verify the affected code if the checkout has advanced.
3. Follow only the relevant source/test entries in Current State. For wider invariants, use [discovery and reading](../architecture/discovery-and-reading.md) or [reader storage and backup](../architecture/authentication-and-reader-storage.md), not completed plans as live specifications.
4. When implementation is authorized, resolve the engineering gates below first. Record the chosen concrete contracts in this plan before dependent code; do not restart product discovery or silently choose a schema change. If a simpler design changes behavior, obtain approval rather than relabeling it an implementation detail.
5. Continue through the tracked increments without deferring required behaviors. Update this plan's Current State, Next Action/checklist and actual Verification before handing unfinished work to another session. Do not create a second plan for the same workstream.

## Scope and settled policy

| Concern | Accepted behavior |
|---|---|
| Providers | Shared client loading for BookSource, TXT and EPUB; provider-owned server retrieval |
| Client storage | Memory plus IndexedDB, behind one chapter-cache module |
| BookSource freshness | **24 hours** from successful upstream retrieval; copies/access do not renew it |
| TXT/EPUB freshness | Identity-based validity; no TTL or repeated preparation |
| Client retention | **3 recently read books including the current book**, up to 5 retained chapters each |
| Chapter window | Current chapter, previous **2 if already fetched**, next **2** |
| Speculation | Forward only, nearest first, using the same loader; preserve the existing default-on preference |
| Renewal | Both forward targets while actively reading with prefetch enabled; one scheduler/timer |
| Refresh | Bypass client caches and explicitly bypass backend BookSource cache |
| Expired content | Cannot satisfy a new load; no expired-copy/outage fallback; already-displayed prose is not cleared by expiry |
| Portable backups | Exclude disposable caches on export and restore; preserve durable reading and recovery state |
| BookSource image availability | Immutable backend bundles cover the advertised fresh-cache interval, independent of chapter-text eviction; no indefinite displayed-document guarantee |
| Resource admission failure | Preserve useful prose with explicit unavailable figures; never issue unbacked URLs or treat incomplete preparation as fully reusable |
| Source-definition changes | Invalidate affected resource execution identities; do not archive old source runtimes |

Offsets refer to **main readable sections**, skipping volume headings and auxiliary sections, not arithmetic on chapter indices. Short catalogs/ends of books yield smaller windows. Explicit EPUB note/auxiliary navigation remains supported on demand.

Exclude offline app/PWA support, service-worker chapter caching, background sync, whole-book downloads, image-blob prefetch/caching, automatic retry systems, server prefetch subscriptions/heartbeats, permanent backend refresh jobs, new provider/plugin frameworks and cache configuration UI. Existing static-asset/image policies are separate. Required image-reference correctness is in scope even though image-byte caching is not.

## Accepted Approach

### Ownership and data flow

```text
Reader navigation / next-two preparation
  → existing chapter loader
  → one client chapter cache: memory → IndexedDB
  → authenticated backend reading interface
      BookSource: saved processed chapter → upstream on miss/expiry/Refresh
      TXT/EPUB: existing prepared reading data
```

| Owner | Responsibility |
|---|---|
| Reader screen / existing navigation | Requested destination, committed display, progress, note trail, anchors and recovery UI |
| Existing chapter loader | Lookup sequence, matching-request sharing, foreground priority, two-target prefetch/renewal |
| Small client chapter-cache module | Memory/IndexedDB storage, eligibility, retention, invalidation and storage failures |
| Existing display converter | Memory-only conversion reuse and shared pending conversion for canonical documents |
| Backend reading / BookSource execution | Identity/freshness, cache-first retrieval, shared upstream work and safe source-session ordering |
| BookSource resource owner | Immutable image bundles, authorized resolution, finite availability, bounded admission and cleanup |
| Reader-home lifecycle / portable preparation | Replacement identity, coherent backup/restore and disposable-state exclusion |

These are ownership boundaries, not instructions to create one new class/interface per row. Extend existing cohesive modules. Keep memory and IndexedDB behind one policy; do not build separate caches with independent lifetimes. Do not add another backend RAM cache, generic storage adapters or a parallel prefetch scheduler.

Store canonical structured content, not converted output or HTML. Display successful content without awaiting IndexedDB persistence, pruning or progress acknowledgements. Client-cache persistence failure is not a reading failure. Backend image-resource admission has the explicit degraded-reading contract below; it must not be hidden as an ordinary successful cache write. No API/service-worker HTTP cache should silently retain chapter JSON in parallel with application-managed persistence; use an explicit non-storing HTTP policy for those responses.

### Identity and lifecycle

Separate three concepts:

- **Identity:** reader + reader-home generation + book + content revision + chapter.
- **Freshness:** whether a matching BookSource copy can satisfy a new load.
- **Display:** the document/location actually committed to the screen.

Validate authenticated current book identity/resume and a matching catalog before local reuse on reader entry. Do not optimistically display disk content before that check. IndexedDB improves retained chapter reuse; it does not eliminate entry validation or promise offline opening.

Existing content revisions cover source switches and TXT reparse. Switching sources must make old-source copies ineligible, retrieve content under the newly selected binding and prepare its forward window. Never fall back to old-source content or admit late old-binding responses. Reparse likewise cannot reuse old ordinals merely because titles/indices match.

The chapter lookup identity is not an immutable document-instance identity: an upstream Refresh may produce a different document under the same interpretation revision. The resource design must distinguish those instances without treating every refresh as a catalog replacement or remapping progress/bookmarks. Home generation, interpretation revision, document-instance identity and client request/invalidation generations have different jobs; do not substitute one for another.

Add one durable **reader-home generation** for creation/replacement. Preserve it across ordinary restarts/runtime eviction; do not derive it from Reader ID or the Legado device identity, and do not import the old value from a portable archive. Establish the new generation in the staged replacement and publish it with that home; rollback retains the prior home's generation. Carry/validate it at the reader interface so separate entry requests and later content/state operations cannot accidentally combine pre-/post-restore data with repeating book IDs/revisions. This home-generation boundary is implemented (Current State); it does not resolve same-home document-resource identity.

Logout/account change retires pending work and clears the previous reader's entries; removal clears that book. Restore retires old reader work before reopening fresh state. Reuse existing request/navigation generation guards rather than introducing a global frontend state machine.

The client cache owns invalidation. Check invalidation/write eligibility in the **same IndexedDB transaction** as a persistent write so another tab's known invalidation cannot be undone by a late response. Notifications may promptly retire other tabs' memory/work, but notifications alone are not the correctness mechanism. Keep this narrow: no tab pinning, heartbeat, distributed eviction or continuous polling. Revalidate authoritative identity on entry/appropriate resume; do not claim instantaneous discovery of remote-device mutations without a request.

Ordinary pruning is not invalidation. A different tab may lose a disk entry while retaining its displayed document; its next miss can reload normally.

### Loading, freshness and explicit Refresh

For an ordinary load, use fresh matching memory, then IndexedDB, then backend. Share matching pending work; remove failures from the registry so foreground retry remains possible. A persisted entry is eligible only with the correct identity, supported document/cache format and valid freshness.

For BookSource, check saved processed content before upstream retrieval. On miss/expiry, acquire the appropriate execution ownership, recheck cache eligibility, retrieve/process upstream content and publish only if the binding/revision/home are still current. Return authoritative remaining freshness with the document. Reuse existing `cached_at`/access metadata where sufficient; reads must not rewrite retrieval time.

Centralize translation of server remaining freshness into client eligibility, accounting for elapsed request time and later aging/persistence/reload. Do not restart a full 24-hour lifetime on receipt, disk promotion, conversion, access or a backend cache hit; do not assume synchronized client/server clocks or add a general clock-synchronization service.

TXT/EPUB serve prepared content directly through existing providers. Do not place another backend content cache in front of those durable files or run import preparation during reads.

Explicit Refresh is a mode of the same loading path, not a separate implementation: bypass client entries and explicitly request BookSource upstream retrieval. It must not be satisfied by an older pending cache lookup or overwritten by its late result. Define joining/draining rules at the execution gate. Reset relevant conversion reuse while preserving the current location/note state. Imported content may be reread, not reinterpreted.

Expired copies cannot satisfy new foreground loads; failed fetches never renew freshness. Present Retry/Switch source as applicable rather than the old offline-copy fallback. This deliberately changes current BookSource behavior. Expiry alone never clears/replaces displayed prose. A failed Refresh may retain that committed display with an error; this is not serving an expired cache entry as a successful new load.

Chapter-list freshness remains separate. Do not expand this work into catalog-refresh scheduling.

### Execution ownership and priorities

Matching-request sharing and mutable source-session ordering are distinct responsibilities within a narrow backend owner:

1. Matching chapter retrievals share work.
2. Workflows using the same mutable execution session are ordered safely for their actual shared scope.
3. Active work retains that owner/session lifetime; registry eviction must not create a second independent owner while old work still runs.
4. Recheck eligibility after acquiring ownership; fresh cache reads need not queue behind upstream execution.

**Accepted shared-request/Refresh contract:** the reading provider owns matching chapter flights. Refresh waits for earlier ordinary work, then performs a new upstream retrieval; concurrent Refresh callers share that new retrieval, never an ordinary cached result. Started shared work finishes within the existing source timeout even if callers disconnect. The initiating handler retains its runtime lease until execution completes; other canceled waiters may detach. This deliberately avoids waiter-count cancellation and a background-job manager. Extend the existing registry session lease through cache recheck, processing, validation and publication/save using a scoped chapter-workflow callback; do not add a disconnected execution lock. Fresh cache hits bypass that execution queue. No new durable schema or dependency is required; the optional Refresh query is additive, but older servers do not honor it. This contract is implemented and verified in the backend cache-first/Refresh milestone. Reload older reader tabs on upgrade: older clients omit the new Refresh query and would perform an ordinary cache-first read.

Foreground joins a matching prefetch and outranks speculative work not yet started. Drop obsolete queued speculation after a jump/window change. Do not promise foreground can safely overtake already-started conflicting source work: browser abort is not proof backend execution stopped. Source switching/Refresh must drain or authoritatively cancel started work before conflicting replacement execution. Retire late results without waiting to show destination feedback.

### Two-target prefetch, display preparation and renewal

After successfully displaying a main chapter, derive at most the next **two main readable chapters** from the catalog. Prepare nearest first through the same loader; completion may advance to the second already-selected target but must never derive a third from the prefetched chapter. Do not fetch backwards to fill the retained window or fetch intervening chapters after a far jump.

Use one shared window policy for client retention, target selection, renewal and tests. Do not duplicate numeric offset logic across views. Keep at most one speculative preparation in execution at a time; two forward targets do not require parallel mutable source execution.

Warm the existing display converter for each loaded forward target and the current Chinese-conversion mode, using the canonical object retained by the loader. Foreground shares its pending/completed conversion. Conversion stays memory-only; no second persisted display cache. Preference changes retire obsolete preparation and cannot publish text in the wrong mode. Preserve existing visible conversion-failure guidance and readable canonical-content fallback. No image downloads are started just to prefetch a chapter.

One scheduler and **one expiry timer** track both targets; wake for the earliest eligible target deadline and recompute after work. BookSource target expiry initiates renewal while the reader is visible and prefetch is enabled. Fresh targets need no content request; imports need no expiry timer.

Reconsider targets after committed navigation, completion of work that blocked scheduling, enabling prefetch, relevant display-preference changes and return to a visible reader. Disable/hide/exit cancels future speculative scheduling/renewal; started work still follows safe retirement rules. Never renew abandoned books on the server.

Failed speculation is quiet and cannot immediately retry through its own completion/idle/expired-timer path. Foreground can retry normally; otherwise only a meaningful new scheduling event may reconsider it. A failed nearer target must not create a retry loop or permanently starve the second target. Successful target renewal does not replace the currently displayed chapter.

**Return-after-absence example:** while chapter 20 remains displayed, returning after 24+ hours checks targets 21 and 22 and prepares expired/missing ones nearest first. Their backend copies may already be fresh from another device; otherwise backend retrieval goes upstream. Clicking Next joins matching work. Reopening/reloading instead performs normal identity validation and also loads chapter 20 if its copy expired. There is no backend timer while the client is absent and no promise of exact renewal during device sleep.

### Immediate feedback and committed reading state

Keep requested destination separate from the last committed document/navigation snapshot. Select destination/title immediately. A display-ready memory hit can commit immediately; otherwise clear the previous prose from the pending view and show destination loading, then content/error with no artificial spinner delay. Retry targets the failed requested destination, not the old displayed index.

Preserve the prior committed snapshot for recovery rather than overwriting its identity early. Commit document, converted display, location and EPUB navigation proposal together after generation checks and anchor restoration. Missing anchors/failed note transitions retain or restore the prior note trail and position. Retaining that snapshot internally does not require showing old prose under a new title.

Progress/bookmark capture always describes committed visible content. Flush departure state through the existing ordered non-blocking writer; never attach old scroll position to a new requested index or count failed loads/prefetch as reading. Preserve last-read semantics, orphan handling, revision-qualified links, same-section/nested notes and write-free auxiliary visits. Conversion and late content cannot overwrite a newer navigation.

### Retention and storage failure

Keep up to **5 retained chapters per book across 3 books**: current, up to two fetched predecessors and two successors. Recent-book retention follows actual reader use, not speculative fetch completion. A jump moves the window; it does not fetch the backwards/intermediate gap. Near boundaries retain fewer entries.

The 15-entry policy bounds reusable cache entries, not all memory held by multiple tabs, in-flight responses or committed/rollback display snapshots. Explicit note/auxiliary visits remain on demand and must not advance the main reading window or introduce an unbounded second cache; preserve their existing return behavior within bounded retention.

For the client chapter cache, no per-chapter size cutoff or application byte budget initially. Device quotas still apply. Prune obsolete/out-of-window/inactive entries. On storage pressure, evict inactive entries and retry persistence once; continue with memory/network if it remains unavailable. Do not promise unlimited storage or guaranteed persistence, and do not loosen parser/input safety bounds.

Backend retention is independent of the client window and cannot use only one device's saved progress. Reuse bounded access-recency pruning at existing storage operations; the current 100-per-book/500-per-reader limits are the starting point, not a reason to add a cleanup service or matching ±2 scheduler. Those limits govern chapter copies, not permission to evict promised image bundles. The new resource store needs its own bounded admission and expired-data cleanup (next section), not server chapter prefetch. Never prune managed TXT/EPUB originals/preparations as caches.

### Document and image-resource identity — accepted design

**Owner:** an immutable BookSource image bundle owns a document's ordered private image descriptions and the request context actually needed to resolve them. Retain URLs/options and inline `data:image/` payloads (supported by `book/image.go` up to its existing 10 MiB decoded-image limit); do not fetch remote image bytes during chapter preparation. Prose stays in the existing independently evictable chapter cache, not a second complete snapshot store. A single cohesive backend resource module owns issuance, persistence, authorization, resolution and cleanup; callers do not assemble retention rules.

**References and publication:** ordinary authenticated image URLs identify a bundle and its image ordinal, scoped to reader/home/book/interpretation and resource execution identity. They never consult the latest chapter row to choose the image. Commit bundle admission before issuing usable URLs. Refresh does not overwrite an old bundle; identical descriptors and captured context may reuse a chapter-scoped bundle and preserve the maximum deadline already promised. This requires no per-image sharing, reference counts or tab registry. No plaintext source descriptors or encrypted descriptor payloads are sent to the frontend. Keep native browser image loading; no application-managed image-blob cache or POST/object-URL loading path.

**Finite availability:** admitted descriptions remain available for at least the document's advertised reusable-content interval, despite normal server chapter eviction or another tab's Refresh. The baseline deadline is successful upstream retrieval plus 24 hours, not 24 hours from each access. Cache-hit issuance must ensure the bundle covers the remaining interval; it cannot advertise eligibility beyond either content freshness or resource availability. Reusing a bundle for a later successfully fetched document may extend its resource deadline, never the freshness of an older document. Persist deadlines across ordinary restart; restart does not renew them.

After that interval, already-displayed prose and still-loaded images remain. A later lazy/repeated image request may explicitly report expired/unavailable resources. Offer a user-requested chapter reload preserving location where possible; never silently replace text or resolve to a newer bundle. No separate display lease, heartbeat or active-image renewal timer. The accepted next-two **chapter** prefetch/renewal remains unchanged. This finite promise supersedes an interpretation that every indefinitely displayed document must keep its server descriptions forever; it does not permit breaking a fresh document's promise through ordinary eviction.

**Admission and failure:** bound resource bytes and entry/descriptor counts, including inline payloads, at reader and installation scope. Reclaim expired allocations before admission; do not evict unexpired promised bundles to admit another. If storage/admission fails, preserve useful prose with explicitly unavailable figures, not unbacked URLs or a fully prepared persistent/prefetch result. For image-only content, clearly report that usable content could not be loaded. Allow explicit retry without a completion-triggered retry loop. Keep failure representation local to figures where possible; no generic partial-response framework, foreground reservation scheme or new admission scheduler. Concrete numerical budgets, accounting and response shape remain engineering gates, not invented defaults.

**Reclamation and lifecycle:** cleanup is owned by the backend and must eventually visit inactive resource data, not rely only on reopening a book. Use bounded startup/periodic metadata cleanup, reusing existing lifecycle infrastructure where suitable; it must not crawl upstream or renew content. Store bundles atomically as owned units rather than loose descriptor files requiring a reference-counting/garbage-collection framework. Resolve an authorized descriptor into bounded request-owned state before network execution so cleanup can proceed without a database transaction held across crawling. Book/source removal, interpretation changes and home replacement invalidate ownership rather than merely aging an entry. Portable backups omit the new disposable store without changing live resources. Plan storage accounting/reclamation for database journals and temporary writes as well as payloads; distinguish bounded allocated space from immediate filesystem shrinkage. Do not broaden this into whole-application memory hardening or promise zero possible leak bugs.

**Source changes and limits:** invalidate affected references when their source execution definition changes; explicit reload must issue against the current definition rather than resurrecting an invalid bundle. Use a simple, reliable execution identity, not a script-dependency analyser. Decide its exact fields and cache/reload integration before implementation. Do not preserve historical source definitions, credentials, cookies or JavaScript sessions. Live authentication/session state and upstream availability remain dynamic: retaining a resource recipe guarantees neither unchanged remote pixels nor successful execution forever. This work does not solve expiring upstream URLs, captcha/WAF challenges, arbitrary Android decoders or offline image reading. EPUB retains its existing home/revision-qualified durable-resource path.

**Storage decision (accepted):** use one process-owned disposable SQLite database at `DATA_DIR/cache/chapter-resources.sqlite`, outside reader homes. This permits atomic reader/installation admission and cleanup of inactive data without a second global accounting registry; portable snapshots never include it. Reader deletion requires explicit scoped cleanup. Every lookup must bind reader, home generation, book, interpretation revision and source execution identity; storage partitioning alone is not authorization. Expose reader/installation byte and bundle-count budgets through environment variables. Initial defaults are 256 MiB / 1 GiB and 2,000 / 10,000 bundles respectively; these are ceilings, not preallocation. A 10 MiB inline image needs about 13.3 MiB as base64; URL-only bundles are much smaller. Limits bound encoded payloads plus stored owner metadata, not exact filesystem or heap usage; leave disk headroom for SQLite pages, transactions and the rest of the application. Deployment sizing must follow active workload, not registered-user count. Lowering limits must not revoke existing promises: reject additional allocations until usage falls. Use a private directory, short serialized SQLite transactions and periodic expiry cleanup; no network work while holding storage transactions. No new dependency or reader-schema change.

The initial storage implementation is `backend/internal/chapterresource`: format-1 SQLite, immutable JSON image recipes and script book/chapter maps, SHA-256 identity over ownership plus recipes, one connection for atomic admission, and one-minute expiry housekeeping. SQLite DELETE journals and FULL auto-vacuum reclaim freed pages on commit rather than retaining a WAL or running full periodic VACUUM. Resolution returns a request-owned copy before upstream work. Authenticated-server startup/shutdown and retryable account-deletion cleanup now own the store. A storage-open failure logs the error and leaves existing files intact; unrelated reader functionality stays available, while account deletion cannot falsely report successful cache cleanup. Chapter publication and image resolution now use admitted bundles. Captured maps come from the content workflow and its chapter metadata, not a later mutable book read. Whole-definition checks run before publication and after image execution; unknown source fields participate, while application timestamps and representational default differences do not.

Configuration is parsed in `backend/internal/config/chapter_resources.go` and passed explicitly from `cmd/server` to authenticated-server composition:

| Environment variable | Default | Meaning |
|---|---:|---|
| `CHAPTER_RESOURCE_CACHE_MAX_MIB` | 1024 | Installation encoded-byte budget in MiB |
| `CHAPTER_RESOURCE_CACHE_READER_MAX_MIB` | 256 | Reader encoded-byte budget in MiB |
| `CHAPTER_RESOURCE_CACHE_MAX_BUNDLES` | 10000 | Installation bundle-count ceiling |
| `CHAPTER_RESOURCE_CACHE_READER_MAX_BUNDLES` | 2000 | Reader bundle-count ceiling |

Both reader and installation constraints apply independently. Invalid/nonpositive/overflowing values fall back to defaults, following existing configuration conventions. These are not total application disk limits. Effective operator settings and graceful image failure/retry behavior are documented in README and the deployment Compose file.

**Image integration contract:** capture the content workflow's book script map and the chapter metadata used for image execution; retain them with the bundle instead of reconstructing them from later progress/catalog reads. Fingerprint the canonical source definition (including unknown imported fields), excluding only application-created/updated timestamps; this is deliberately conservative, not dependency analysis. Check the identity again before publishing content or image bytes. Bundle-qualified image URLs replace ordinal-only resolution; unqualified old image URLs must fail rather than consult a mutable row. Resource admission failure uses an explicit unavailable image reference, handled by the renderer without creating an `<img>`; image-only content reports failure. Frontend/backend wire changes must land together. Persist source identity/context in a versioned chapter-cache payload using existing columns, accepting legacy array payloads for inspection but not reusing them as qualified document snapshots. Rollback may lose disposable cache hits but does not change the reader schema or durable locations.

**Compatibility gate:** validate these concrete wire/storage contracts and source-change checks with synthetic integration tests before activating persistent client reuse. The independently versioned cache must fail clearly on an unsupported format rather than silently discard active promises. Assess their authorization, portable exclusion, rollback and schema compatibility before dependent code. No reader-schema epoch bump, reset, deployment or new dependency is authorized by this decision.

### Portable backup and restore boundary

Back up durable reader state; recreate disposable acceleration state. “Dynamic” or “derived” does not automatically mean disposable.

- **Preserve:** books, sources, portable settings, progress/bookmarks, saved catalogs/revisions, TXT/EPUB originals and prepared reading data/resources, acquired imports awaiting review, and records required for recovery/cleanup. Catalogs and prepared interpretations give existing locations meaning; rebuilding them implicitly would compromise restore.
- **Exclude:** disposable fetched BookSource chapter copies and their cache metadata, any new disposable resource-cache records/files identified by the resource design, temporary transfers, in-flight execution/timers and device chapter caches. Keep existing credential/source-authentication exclusions unchanged; lossless source definitions are not newly sanitized.

Use feature-owned `ReaderSchema.PreparePortable` on staged copies for export **and** restore. The hook already runs in both directions; BookSource should remove its disposable cache there. Compact sanitized database snapshots so discarded payloads are not carried in unused pages. Exclude only proven disposable file locations through the existing copy boundary; do not guess that every cover/resource/file is a cache. Never clear live caches during export.

Compatible older archives containing cache rows are accepted under existing compatibility rules and sanitized before publication; this does not make older schema epochs compatible. Restored BookSource chapters need upstream retrieval when next requested; imported publications remain readable from their preserved data. Cache exclusion does not clear other devices' caches: the new home generation handles eligibility.

Preserve validation, quiescence, staged atomic replacement, uncertain-outcome recovery and rollback. Specify generation initialization for compatible existing homes and supported manual portable restore before implementation. A stopped-server **complete `DATA_DIR` copy remains complete disaster recovery**, not a filtered portable export; document client-cache invalidation implications of that restore route without silently weakening it.

## Decisions and simpler alternatives

Design patterns describe existing needs, not mandatory new abstractions:

- **Read-through loading:** one operation owns lookup, miss retrieval and population; callers do not orchestrate cache levels themselves.
- **Shared in-flight work (single-flight):** matching callers share retrieval; this is separate from ordering different requests against a mutable source session.
- **Immutable identity plus generation-checked publication:** results belong to a specific lifetime; reject stale results at admission instead of repairing contamination afterward.
- **Requested versus committed state:** navigation is a proposal until display succeeds; reuse the existing EPUB proposal/rollback model.
- **Feature-owned portable projection:** each storage owner strips its disposable state from a staged copy; backup orchestration stays independent of source/provider internals.

- Memory/IndexedDB/backend storage each removes a different cost. Do not defer persistence or active renewal merely to reduce implementation scope; implementation increments are not feature cuts.
- HTTP caching is useful generally but cannot enforce this selected per-book window and precise application-owned deletion. Use one persistent chapter-cache owner; keep static/image HTTP policies separate.
- Reuse the loader, converter, reader revisions, backend timestamp/recency fields and portable-preparation hooks. A small IndexedDB helper may be considered if it reduces real boilerplate; no dependency/framework is mandated.
- Use ordinary observable fetches, not browser `rel=prefetch` hints. ETags do not establish upstream freshness; browser cache bypass does not force backend refresh.
- Expiry is eligibility, not a UI replacement event. Only successful upstream retrieval establishes a new BookSource freshness interval.
- No server renewal subscriptions/jobs, recursive prefetch, failure retry loops, synchronized eviction or distributed tab pinning. Bounded resource reclamation is housekeeping, not a content-renewal job. Correctness comes from ownership and explicit lifetime promises, not timing assumptions.
- Prefer backend-owned image bundles and native image URLs for low-end clients. Self-contained encrypted descriptors avoid server retention but add inline-payload round trips, key management and custom browser loading; they are not selected. Independent resource LRU eviction can break still-promised documents, so admission must preserve existing deadlines. Indefinite retention and display leases are not selected; revisit only if actual image-expiry experience justifies a changed contract. Whole-bundle reuse is sufficient initially; per-image deduplication needs measured benefit.
- This work supersedes the session-only retention/outage-fallback direction of the completed [navigation performance plan](reader-navigation-performance.md), not its historical verification. Other import issues stay in [their canonical note](../notes/2026-09-21-import-user-testing-and-branch-review.md).

## Original-plan cross-check

All original behavior families remain covered: provider sharing; persistent canonical content; online entry validation; lifecycle and cross-tab invalidation; explicit Refresh; backend-owned freshness; no expired fallback; non-blocking display/progress; speculative failure recovery; visibility-aware renewal; storage-pressure fallback; bounded relevant retention; and compatibility/rollback. Original exclusions also remain unless the necessary portable-cache exclusion/resource-correctness work above explicitly adds scope.

Intentional changes: one forward target becomes two; the retained ±1 window becomes ±2; tentative TTL/book-count examples become approved policy; portable-cache exclusion and display-conversion warming are explicit. EPUB rollback semantics and the missing backend execution/home-generation/resource contracts are now explicit rather than assumed. The later accepted resource contract explicitly limits server descriptor availability to the advertised fresh-cache interval, allows clearly reported figure unavailability under admission failure, and invalidates affected source-definition references. It does not remove or defer client persistence, backend freshness, forward renewal, navigation feedback or portable-cache exclusion.

## Current State

**Implemented and verified:** portable-cache exclusion, durable reader-home identity, bounded immutable image bundles with captured execution context and explicit figure failures, full chapter workflow ownership, backend cache-first/Refresh and matching-request sharing. These milestones require no reader-schema change. The client now uses a single memory/IndexedDB cache owner with online identity validation, source-definition qualification, original freshness, commit-time retention and cross-tab write protection. Disconnected started backend work still drains under runtime ownership. The final reader-experience milestone is implemented: the existing loader now owns a selected-target pump and one renewal timer, and warms the existing converter without another display cache. Requested destinations are separate from committed navigation/progress; failed destination retry preserves its target. Focused reader regression tests, production compilation and a synthetic real-browser reader journey pass. Implementation is complete; live-source latency and deployment remain unverified. No latency improvement has been measured.

| Current fact | Entry points |
|---|---|
| One memory/IndexedDB owner retains three committed books and their five-section windows; canonical documents, original freshness and transactional invalidation gates; loader prepares two selected targets with one speculative operation and one expiry timer | `frontend/src/features/reader/chapter-cache.ts`, `chapter-cache-policy.ts`, `chapter-cache-storage.ts`, `chapter-loader.ts` and adjacent tests |
| Entry pairs book/catalog revisions; display commits after conversion/anchor restoration; note rollback and ordered progress already exist | `reader-session.ts`, `ReaderView.vue`, `reader-navigation.ts`, `progress-writer.ts` and reader tests under `frontend/src/features/reader/` |
| Forward preparation warms the existing mode-qualified converter; foreground shares its pending/completed work | `frontend/src/features/reader/chinese-conversion.ts`, `frontend/src/api/system.ts` |
| Backend is cache-first with 24-hour retrieval-based freshness, explicit Refresh and no expired/unqualified fallback; access recency remains separate | `backend/internal/reading/booksource_content.go`, `backend/internal/book/chapter_cache.go` |
| BookSource responses expose `freshForMs`; client memory uses conservative request-start deadlines. `refresh=true` explicitly bypasses backend chapter reuse | `backend/internal/reading/service.go`, `document.go`, `backend/internal/api/reader_handlers.go`, `frontend/src/api/reader.ts` |
| Registry leases order shared session workflows through chapter publication; runtime-bound reading flights separately share chapter results and order Refresh | `backend/internal/sourceexec/session_workflow.go`, `session_registry.go`, `backend/internal/book/search.go`, `image.go` |
| Source switching/reparse already advance interpretation revisions | `backend/internal/book/source_switch.go`, `backend/internal/library/state.go`, `backend/internal/txtstore/reparse_apply.go` |
| BookSource images resolve immutable bundles independent of chapter rows; captured context and canonical source-definition identity qualify publication/resolution | `backend/internal/api/chapter_image.go`, `backend/internal/book/image.go`, `backend/internal/reading/booksource_content.go` |
| Durable manifest generation survives restart/eviction and changes with replacement; runtime API and generated image URLs validate it | `backend/internal/readerstore/home_identity.go`, `manager.go`, `backend/internal/api/reader_api.go`, `epub_resources.go`, `chapter_image.go` |
| Portable preparation strips BookSource cache rows/flags on export/import and compacts the staged database | `backend/internal/readerstore/backup.go`, `portable_validation.go`, `database.go`, `backend/internal/book/store.go` |
| Account/restore resets and book/source/reparse changes invalidate cache state; IndexedDB epochs reject late writes across connections, BroadcastChannel retires memory, entry/visible-return requests validate identity | `frontend/src/app/reader-state.ts`, `frontend/src/features/reader/chapter-cache-storage.ts`, `ReaderView.vue` and mutation views |

The reported unexpected Next-chapter refetch remains **unreproduced**. Failure-fallback non-retention, unfinished prefetch and conversion waits are investigation leads, not a diagnosed root cause. No latency gain has been measured.

## Next Action and implementation tracking

Implementation is complete. No further caching implementation is scheduled. Optional follow-up is normal real-book user testing and investigation of a reproducible live-source delay; do not claim measured speedups or a diagnosis of the original report. Push/deployment still require authorization.

1. **Completed:** put memory and IndexedDB behind one chapter-cache owner. Apply the accepted identity, three-book/five-chapter window and canonical-document policy; preserve backend remaining freshness rather than restarting TTL on copies.
2. **Completed:** gate persistent writes and cross-tab invalidation transactionally across logout/account change, removal, restore and interpretation changes. Validate reader entry before reuse; do not treat home generation as a secret or rely only on tab-local request cancellation.
**Accepted implementation boundary:** expose the existing source-definition fingerprint as optional `sourceIdentity` in BookSource catalog/content responses. Client entry and stored/network documents must agree on it; missing qualification disables BookSource persistence rather than assuming validity. Imports remain revision-qualified. This is additive wire metadata, not a schema migration or a new revision system. Use native IndexedDB with one shared invalidation counter: capture it before retrieval, then compare it in the same transaction as persistence; notifications retire affected memory but are not the write gate. Add pinned `fake-indexeddb` as a development-only dependency for deterministic transaction tests. This client milestone is implemented, enabled and verified. Cache sessions are bound only after online book/catalog validation; commit-time retention follows successful display/anchor restoration. Concurrent same-loader Refresh calls share one operation. Storage failure falls back to memory/network; quota pressure gets one inactive-book eviction retry. The development-only `fake-indexeddb` dependency covers real transaction semantics without adding a production dependency.

3. **Completed:** preserve explicit Refresh bypass and graceful storage failure. Unavailable-image responses have zero reusable freshness and must not produce immediate renewal loops. The Refresh query parameter remains optional for older clients; deployments must reload old reader tabs for the new Refresh protocol.
4. **Verification limit:** controlled delayed retrieval verifies destination feedback and foreground/prefetch sharing. The original live-source Next-chapter report remains unreproduced; distinguish retrieval, conversion and lifecycle causes if it recurs.

TTL, client book count/window and finite resource behavior are settled; do not reopen them as unanswered preferences. Any necessary durable storage/API compatibility change must be explained before activation. Do not assume a fresh-data epoch bump or reset is permitted.

Track complete, verified increments here, adjusting order for actual dependencies rather than cutting scope:

- [x] Accept the scoped resource behavior: finite image bundles, explicit admission failure, source-change invalidation; reject indefinite-display coordination.
- [x] Settle and implement the shared-request/Refresh ownership contract; storage/budgets and wire compatibility are recorded.
- [x] Exclude existing disposable chapter caches through portable preparation; verify preserved durable reading/recovery state. Any new resource-cache state must join this boundary when introduced.
- [x] Implement image-bundle identity/lifetime and narrow session execution ownership. The same ownership now covers cached-reading publication.
- [x] Implement backend cache-first freshness and explicit Refresh without expired-copy fallback.
- [x] Implement client memory/IndexedDB lifecycle, retention, invalidation and storage-failure behavior.
- [x] Implement requested/committed navigation feedback while preserving progress and EPUB semantics.
- [x] Implement both forward targets, conversion preparation and active-reader renewal using the shared window policy.
- [x] Complete focused integration/browser verification and update affected current-truth documentation.

Update Current State, this checklist/Next Action and Verification at meaningful milestones. Accepted design is not evidence of delivery; keep this single workstream plan handoff-ready.

## Verification

**Final reader-experience milestone:** `npm test -- src/features/reader src/api/reader.test.ts` passed 101 tests across 23 files; direct `vue-tsc --noEmit` and Vite production build passed. Focused cases cover both targets, foreground priority/sharing, dropped obsolete targets, quiet failure, expiry while another target is retrieving, one-timer renewal/pause, imported-content timer exclusion, conversion reuse, destination feedback/retry, and existing note/anchor/progress invariants. Chromium with synthetic authenticated API responses verified pending navigation hiding old prose, foreground joining prefetch, failed-destination retry, and persisted reopen after online validation; desktop loading and mobile error screenshots were inspected. This browser journey used mocked HTTP, not a live source or deployed backend. The mechanical UI detector reported no findings. Backend code was unchanged in this final milestone; its earlier verification is recorded below. No live performance measurement, full deployed restore journey or claim of universal source compatibility.

Earlier paragraphs below report milestone-specific checkpoints, not additional final runs.

**Client persistence milestone:** the full frontend suite passed (88 files, 346 tests), direct `vue-tsc --noEmit` and Vite production build passed. After final Refresh-sharing and transaction-abort handling adjustments, 16 cache/storage/loader tests, TypeScript checking and the production build passed again. `go test ./internal/reading ./internal/api` and `go test -race ./internal/api -run ChapterContentCache -timeout 30s` passed, including source-tag changes without revision changes. Fake IndexedDB tests cover persistence, retention, invalidation/write races and storage failure. An isolated Chromium module-level check with synthetic content verified reuse across two tabs, BroadcastChannel invalidation and non-resurrection after reload; this was not a full authenticated reader journey. No live-source latency measurement or two-target renewal/browser journey is claimed. AFT lacked authoritative diagnostics; compiler/test results are the gate.

**Cache-first/Refresh milestone:** `go test ./internal/book ./internal/reading ./internal/api` passes. Focused `-race` tests for `ChapterFlights|ChapterPublication|ChapterContentCache|ChapterRequestCancellation` pass. Coverage checks normal hits without upstream/age renewal, explicit Refresh success/failure, expired-copy rejection, changed source/revision rejection, shared caller cancellation, Refresh ordering/sharing, publication-held session ownership and original freshness deadlines. Frontend reader/API tests pass (87 tests across 20 files at the integration checkpoint); direct TypeScript checking and Vite bundling pass. Final focused checks pass all 9 loader tests, including a pending client copy arriving before failed Refresh; the final API cache test also passes with `-race` after adding source-change and non-renewing-access assertions. No browser journey, live-source timing or persistent-cache test is claimed.

**Performed:** original-plan/conversation cross-check and fresh-session handoff review; documentation whitespace/link checks passed. The portable-cache regression first failed on retained cache rows, then passed after the fix. `cd backend && go test ./internal/readerstore ./internal/backup ./internal/book` passes; `go test ./internal/epubstore ./internal/txtstore -run Portable` passes. These cover the first increment, not cached-reading implementation. Home-identity verification: `go test ./internal/readerstore ./internal/backup ./internal/api` passes; targeted `-race` runs for `HomeGeneration|CompatibleManifest|ReaderGeneration|PublishReplacement` in readerstore/API pass. Frontend shared-transport checkpoint passed all 328 tests; the final reader-lifetime changes pass 80 targeted reader/transport tests. Typecheck and production build pass via `node node_modules/vue-tsc/bin/vue-tsc.js --noEmit` and `node scripts/run-node-tool.mjs vite build --logLevel warn`. Normal `npm run build` encountered a pre-existing local `vue-tsc` executable permission error; no tooling change was folded into this work. No browser journeys or timing measurements have been run. AFT did not provide authoritative diagnostics for the edited Go/TS files; compiler/test commands are the verification gate.

**Image integration verification:** affected `booksource`, `book`, `reading` and `api` package tests pass; focused API race tests for `StoredChapterImage|ChapterResources` pass. Synthetic coverage verifies captured script mutations, image identity after chapter-cache replacement/eviction, source edits, stale revisions and legacy URL rejection, plus prose/image-only admission-failure projection. Frontend reader/API/structured-prose tests pass (88 tests across 21 files), with `npm test -- src/api/reader.test.ts src/api/structured-prose.test.ts src/features/reader`; the final sleep-aware freshness adjustment passes all 7 loader tests; direct `vue-tsc` typechecking and Vite production bundling pass. Browser journeys and persistent-client-cache checks have not run. The fingerprint regression first caught timestamp edits materializing export defaults; canonical typed/default normalization fixed the rule while preserving unknown fields.

**Execution ownership verification:** `go test ./internal/book ./internal/sourceexec` and `go test ./internal/api ./internal/reading` pass. Focused `go test -race ./internal/book ./internal/sourceexec -run 'Workflow|SourceRetirement|SessionRegistry'` passes. Deterministic tests cover pinned identity under idle eviction, cancellation without releasing another owner, queued source retirement/alias suppression, and actual chapter/image ordering while another book proceeds. Existing inline-image tests caught a registry-less caller contract; that path continues to use an isolated session. These checks cover the earlier session-lease foundation; the later cache-first milestone above adds chapter sharing and complete publication ownership.

**Storage foundation verification:** `cd backend && go test ./internal/chapterresource ./internal/config` and the same packages with `-race` pass. Coverage includes immutable recipes, cross-owner/replacement/definition rejection, preserved deadlines, restart, expiry/reclamation, reader deletion, concurrent byte/count admission and environment defaults/overrides/invalid values. Server checkpoint: `go test ./internal/api ./cmd/server ./internal/chapterresource ./internal/config` passes; `go test -race ./internal/api -run 'ChapterResources|ServerCloseDrains'` passes. Integration covers runtime retirement, portable snapshot exclusion, reader removal and shared-store shutdown even when other cleanup fails. No image-rendering, cache-first or end-to-end verification and no performance measurement. AFT has no authoritative Go diagnostic reports for these new files.

**Resource scope review:** two read-only second-opinion reviews informed the design; consequential claims were checked against the chapter publication/image execution code, renderer and this plan. Confirmed code-derived paths include cache-save failure returning dependent image links, live source execution context, and renderer failures retained until document change. No image-lifetime incident was reproduced and no resource/performance tests were run. Acceptance is not implementation or evidence of measured optimization.

Use existing synthetic fixtures and the fewest tests that establish these contracts:

- Client memory → IndexedDB → backend flow and reopen reuse after online identity validation; both prose versions/providers; no unnecessary upstream retrieval on backend hits.
- Shared 24-hour freshness without renewal on cache hits/persistence; explicit Refresh; no expired fallback; fresh backend copies from another device; stable current display after expiry/failed Refresh.
- Source switch, TXT reparse, account/logout/removal and same-ID/revision restore invalidation, including a late write crossing another tab's known invalidation and replacement rollback.
- Image-bundle identity after changed/unchanged Refresh; chapter eviction cannot revoke a still-promised bundle; cache-hit deadlines never renew old prose freshness; restart preserves promises. Cover expiry without silently replacing displayed prose, resource admission failure with explicit figure errors, and owner/source-definition invalidation. A small boundary test should cover cleanup overlapping an admitted image request and reclamation/accounting of inactive expired data. No dependency on latest-row ordinals, historical sessions or unbounded resource retention.
- Two forward targets in main reading order; nearest-first preparation; no backwards/recursive fetch; foreground joins/priorities; safe started-work retirement; failure followed by normal foreground retry without speculative spin.
- One renewal timer for both deadlines, visible return after 24+ hours, disable/hide/exit, and no imported-content TTL. Conversion warming shares results/work and does not commit obsolete preferences.
- Immediate destination loading/error and Retry identity; committed progress/bookmark/last-read ownership; superseded navigation, nested notes, auxiliary sections and missing-anchor rollback.
- Three-book/five-entry retention, far jumps and boundaries, device storage unavailable/quota retry, independent multi-tab pruning without loss of displayed content.
- Portable export/import omit disposable cache data without mutating the live cache; compatible archives with cache rows are sanitized; durable catalogs, imported content and recovery data remain coherent/readable. Preserve atomic replacement and failure recovery.

Combine related cases instead of multiplying every provider/timing/storage combination. Use targeted backend/frontend tests first, race/integration checks for actual shared or replacement boundaries, frontend typecheck/build, and focused browser journeys covering reopen/Refresh, ±2 navigation and recovery (including conversion and EPUB notes). Inspect visible/request behavior before claiming snappiness. No live private sources, repo-wide stress campaign, generic benchmark framework or exhaustive quota simulation by default.

Starting commands below are **not executed verification**. Select only the affected boundary and narrow further with the test runner when useful; add the new tests beside existing ones:

- Reader loader/navigation: `cd frontend && npm test -- src/features/reader/chapter-loader.test.ts src/features/reader/ReaderView.navigation.test.ts`.
- Backend reading/cache: `cd backend && go test ./internal/reading ./internal/book`.
- Portable lifecycle: `cd backend && go test ./internal/readerstore ./internal/backup`.
- After frontend implementation: `cd frontend && npm run build` (includes TypeScript checking). Extend to converter, request/reset, sourceexec and provider tests only as the changed contracts require.

## Compatibility and rollback

Backend fallback and response/cache semantics change deliberately. Client cache data is disposable and format-versioned; clear incompatible entries rather than migrate them. Cache exclusion can reuse existing database tables/hooks; it alone does not require a reader-schema change. Home identity is implemented compatibly in the manifest/API. Image storage is independently versioned. Existing chapter-cache columns now contain a versioned blocks/context/definition payload; legacy arrays can be inspected but cannot become qualified snapshots without upstream retrieval. No epoch migration. Frontend/backend must be upgraded together for explicit unavailable-resource objects. Downgrading before bundle-aware routes requires retiring old reader URLs using the documented stopped-app home-generation renewal step: an older ordinal-based handler ignores bundle qualification and cannot safely serve active new documents. Do not silently delete durable reader data or claim downgrade resource compatibility.

Preserve current reader homes/backups and the existing rejection of incompatible schema epochs. Generation publication must follow replacement/rollback, not a later best-effort update. Preserve supported manual recovery paths; do not silently require API-only restore. A rollback can discard new client caches but must not reinterpret durable locations or delete originals/preparations.

When behavior actually ships, update only affected usage/backup guidance and current architecture, plus this handoff and the `PLAN.md` router. Until then, those current-behavior documents must not describe the proposal as implemented.
