---
status: design-accepted
updated: 2026-09-22
---

# Low-latency reading: chapter caching and prefetch

## Goal

Make opening/reopening books and adjacent chapter navigation responsive, without repeatedly retrieving unchanged content. Show destination feedback immediately when content is not ready. Keep a strong, small core structure: explicit ownership, shared loading logic, limited speculative work and relevant retained chapters.

The user selected application-managed persistent caching and requested documentation first. **No implementation is authorized by this documentation step.** Avoid speculative hardening, exhaustive tests, unrelated refactoring and optional infrastructure. Practical correctness at actual storage/auth/source boundaries is required, not a framework for every imaginable failure.

## Scope

- Shared chapter-content caching and navigation for BookSource, TXT and EPUB.
- BookSource backend cache-first reads, TTL and explicit upstream refresh.
- Persistent device cache, one-chapter lookahead, active-reader renewal and immediate navigation feedback.
- Existing reading/source lifecycle and content identities remain the foundation.
- Exclude offline app/PWA support, service workers, background sync, whole-book downloads, image-blob caching, automatic retry systems, server prefetch subscriptions/heartbeats, permanent backend refresh jobs, new provider/plugin frameworks and cache configuration UI.
- Imported originals and prepared TXT/EPUB server content remain durable book data, not disposable chapter caches.

## Accepted Approach

### Ownership and data flow

1. **Reader screen:** requested destination, catalog title, loading/error presentation, displayed content and reading position.
2. **Existing chapter loader:** cache lookup, in-flight request sharing, foreground loading and next-chapter scheduling.
3. **Small client chapter-cache module:** memory and IndexedDB entries, identity/freshness eligibility, pruning and clear operations. Store canonical structured content, not converted display output or HTML.
4. **Existing backend reading/provider boundary:** authoritative identity and freshness; BookSource uses its saved chapter store before upstream retrieval; TXT/EPUB read prepared content.

Extend existing cohesive modules. Do not introduce generic storage adapters or a parallel scheduler. Persistent writes must not delay displaying successful content. A cache failure is not a reading failure.

### Identity and invalidation

Logical identity: reader + reader-data replacement generation + book + content revision + chapter. Reuse existing identifiers wherever they satisfy this contract; do not assume new fields or a schema epoch are necessary.

- Validate current authenticated book identity/resume on reader entry, then reuse matching local content. Do not show potentially obsolete content optimistically before that check.
- Source switch/reparse makes old identity ineligible, even if ordinal/title matches. A source switch must retrieve content under the new binding, never fall back to the old source.
- Restore needs a replacement identity even when restored book IDs and revision numbers repeat. Verify existing support before designing additions.
- Logout/account change retires pending work and clears previous reader entries; removal clears the book's entries.
- Late responses must not populate or render a replacement session as if they belonged to it. Keep existing generation guards rather than building a new global state machine.
- Known invalidation across tabs must not be undone by late writes. Choose the smallest existing lifecycle mechanism that establishes this; no tab pinning/heartbeat system.

### Ordinary loading and explicit refresh

Read fresh matching memory entry, then IndexedDB, then backend. Share matching in-flight requests. Remove failed requests from the registry so foreground retry is possible. Explicit Refresh bypasses client entries and explicitly instructs the backend to refresh BookSource content; bypassing browser HTTP cache alone does not bypass server storage.

For BookSource, backend reads fresh saved content first; otherwise retrieve upstream, process, save and return. Coordinate duplicate retrievals with existing source-session execution ownership. TXT/EPUB directly serve their prepared content without upstream or repeated preparation.

### Freshness across layers

- Backend successful upstream retrieval establishes BookSource freshness. Device copies preserve that deadline; receiving/accessing/persisting a copy never restarts its TTL.
- Centralize backend time/remaining-freshness handling; do not create competing TTL calculations or assume device/server clocks match.
- Expired content cannot satisfy a new load. Expiry never removes or replaces prose currently being read.
- Failed fetches do not renew freshness. No expired-copy/outage fallback in the proposed BookSource read contract: foreground failure exposes Retry/Switch source. This deliberately changes the existing fallback behavior.
- TXT/EPUB have identity-based validity, no time-based freshness TTL.
- Chapter-list freshness is separate; do not expand this work into catalog scheduling without demonstrated need.

### Prefetch and renewal

After successful display, identify the next navigable chapter and use the same loader to prepare it. Fresh local content needs no request. Otherwise the normal API fills backend/device caches as needed. Prefetch completion must not recursively prepare subsequent chapters.

One speculative target and one expiry timer while the reader is visible. Reconsider on displayed-chapter change, completion of work that blocked scheduling, enabling prefetch, and return to a visible tab. Replace/cancel scheduling on target change or exit. BookSource next-chapter expiry triggers one renewal during active reading; imports need no timer.

- Foreground navigation joins matching running work and outranks speculative work not yet started.
- Preserve BookSource serialization where source scripts mutate session state. Client abort is not proof server execution stopped.
- Failed prefetch is quiet and must not immediately retry through an idle callback. Foreground navigation retries normally; if that fails, present the failure.
- No renewal for abandoned books. Suspended/sleeping devices recheck on return; exact refresh during device absence is not promised.

### Immediate navigation feedback

Select the destination and known catalog title immediately. Display a memory hit immediately; otherwise clear previous prose and show a fetching indicator, then content or an error for the destination. No artificial spinner delay.

Requested destination and successfully displayed content are distinct. Do not write old scroll position against the new chapter or count a requested-but-failed chapter as read. Late responses cannot overwrite newer navigation. Keep this a small state correction, not a reader rewrite.

### Retention and storage failure

Retain previous (if already fetched), current and next chapters, with a small fixed allowance for recently read books. Do not fetch previous chapters merely to fill the window. A far jump moves the window, not a download of intervening chapters.

No per-chapter size cutoff or application byte budget initially. This bounds entry count, not bytes; device quotas still apply. Prune obsolete/out-of-window/inactive entries. On storage pressure, evict inactive entries and retry once; if persistence remains unavailable, continue reading with memory/network. Do not promise unlimited storage or guaranteed persistence. Keep existing parser/input safety bounds separate from cache retention.

Multiple tabs keep their displayed content in memory. Shared persisted pruning may cause another tab to miss the disk cache; that is acceptable. Avoid distributed pinning logic.

Backend BookSource retention uses access recency and bounded relevant working sets, not solely saved progress from one device. Its allowance may be broader for multiple devices. Backend/client evictions need not synchronize. Never apply this pruning to managed TXT/EPUB originals or preparations. Decide the small concrete retention policy during the contract check; no new cleanup service unless existing boundaries genuinely cannot perform it.

## Decisions and simpler alternatives

- **Application-managed chapter persistence selected.** HTTP caching was considered first: it handles fresh response reuse and conditional requests, but cannot enforce a selected chapter window or reliably delete individual reader/book responses. Explicit retention/invalidation motivated IndexedDB, not a desire to replace browser facilities everywhere.
- Use explicit non-storing HTTP policy for chapter JSON when the application owns persistence; avoid a hidden second persistent chapter cache defeating expiry/refresh. Static assets and image policies remain separate.
- Ordinary fetch is preferable to browser `rel=prefetch` hints here: the existing loader needs observable outcomes, request sharing and source-safe ordering. Browser hints do not guarantee execution.
- HTTP expiry does not schedule requests and does not delete expired responses. ETags avoid retransmission but cannot establish upstream freshness. `fetch` cache bypass does not force an upstream refresh.
- No autonomous backend prefetch renewal. One active-client request prepares both layers. Do not add subscriptions or perpetual expiry jobs to optimize inactive devices.
- No automatic failure fallback/retry loop. The user explicitly preferred a normal navigation attempt after speculative failure, then visible recovery actions if it fails.
- No generic caching library/framework mandated. Use native IndexedDB behind a narrow module; assess a small helper only if transaction boilerplate demonstrably warrants it.

## Current State

Design accepted at the architectural level; not implemented. Existing behavior: five-entry reader-instance memory cache, discarded on reader exit/reload; client-triggered next-chapter requests; BookSource retrieval attempts upstream before using saved content as failure fallback. TXT/EPUB serve prepared server content. Reported unexpected Next-chapter refetch remains untraced.

Relevant entry points:
- `frontend/src/features/reader/chapter-loader.ts` and its tests; `ReaderView.vue`; `reader-session.ts`; `reader-preferences.ts`.
- `frontend/src/api/reader.ts`, `transport.ts`.
- `backend/internal/reading/booksource_content.go`; existing book chapter cache and source execution ownership.
- `backend/internal/api/reader_handlers.go`, `epub_resources.go`, `chapter_image.go`.
- `backend/internal/readerstore/device_identity.go`: existing device ID derives from reader ID, not reader-data replacement. Do not mistake it for a restore generation.

[Earlier navigation performance](reader-navigation-performance.md) is completed historical context. This proposal supersedes its session-only retention and fallback direction if implemented, not its recorded verification. Other user-reported import issues and the branch review remain in [their canonical note](../notes/2026-09-21-import-user-testing-and-branch-review.md); they are not silently included here.

## Next Action

Wait for implementation authorization. Then inspect only the direct contracts needed to settle:
1. Source switch/reparse revision changes and reader-data replacement identity across restore.
2. Existing upstream execution/deduplication ownership and backend cache metadata/retention.
3. One reproduction of unexpected Next-chapter loading to distinguish a genuine cache/lifecycle miss from prefetch not finishing.

Confirm BookSource TTL and recent-book retention count; one day and three books were discussion examples, not approved constants. Identify any necessary schema/API compatibility change before edits; no fresh-data epoch bump, migration, reset or deployment is authorized by this plan.

Suggested complete increments: immediate navigation feedback/root-cause lifecycle fix; backend cache-first freshness and explicit refresh; client persistence/invalidation; next-chapter renewal/retention. Adjust ordering to real dependencies rather than forcing artificial commits.

## Verification

Documentation and source inspection only for this workstream; no cache implementation tests, timing measurements or browser proof yet. Previous branch checks belong to the linked review note and do not verify this proposal.

After implementation, use existing fixtures and the fewest focused tests that establish:
- Reopen reuse and memory/IndexedDB/backend miss flow.
- Shared freshness, explicit refresh and source/restore invalidation.
- Request sharing, source-safe ordering and one-step prefetch (including failure followed by foreground retry).
- Destination loading/error feedback and late-response/progress ownership.
- Practical storage-failure fallback and relevance pruning.

Combine related cases where clear. Do not enumerate every timing, browser, provider, storage-size or malformed-state combination. Add concurrency coverage only for an actual shared boundary. Use one focused browser journey to verify reopening/refresh, adjacent navigation and failure feedback. Measure visible/request behavior before claiming performance improvement. No repo-wide suite, stress campaign, exhaustive quota simulation or generic benchmark tooling by default.

## Compatibility and rollback

Future implementation changes backend fallback and response/cache contracts. Existing persisted application caches do not yet need migration. Keep client cache disposable and schema-versioned; incompatible local cache can be cleared rather than migrated. Durable backend schema changes, if needed, require a concrete compatibility/rollback decision before activation. Preserve existing reader homes/backups; this documentation authorizes no destructive operation.
