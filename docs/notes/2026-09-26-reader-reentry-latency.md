# Reader re-entry latency: live browser measurements

## Finding

The client chapter cache works, including IndexedDB after a full reload. Reader entry nevertheless waits for uncached metadata/catalog requests before consulting it. Traditional Chinese display conversion is also recreated on every reader lifetime. The bottleneck is the entry dependency chain and missing catalog/display reuse, not an observed chapter-cache miss on warm reopen.

This is a measurement report and **proposed direction**, not an accepted implementation plan. No production code or cache policy was changed.

## Evidence

Measured through the user's authenticated Windows Chrome via the Playwright extension on 2026-09-26 (local project date). Source inspection: `79c4012`. The deployed container revision was not independently queried. Measurements concern one real BookSource book with 1,137 catalog entries; no book text, source definitions, account credentials, private hostnames or response bodies are retained here.

Method:
- Use a separate foreground tab; hidden-tab samples are not valid end-to-end reader completion measurements because position restoration waits for animation frames.
- With the current chapter and its forward entries already warm, click the reader's Back link, wait for the detail page's Continue Reading link plus 250 ms, and click Continue Reading without changing chapter/position.
- Measure click-to-document DOM insertion with `performance.now()` and a MutationObserver for `.reader .chapter-end`. This is DOM readiness, not a compositor/pixel-paint measurement.
- Use Resource Timing for request start, response-header/body times, sizes and status. Wait another 500–700 ms to capture nonblocking progress/preparation requests. A catalog started by the detail page can finish during the reader sample; distinguish it by its negative start offset.
- Three Original-mode and three Traditional-mode samples. The Chrome profile initially used Original, so Traditional was explicitly authorized as a temporary comparison. Restore the exact original local-storage preference afterward; verified restored, then close only the profiling tab and detach.
- Separately issue three authenticated catalog GETs from the browser, collecting only header allowlist, durations, byte/entry counts and JSON parse duration.
- Finally reload the reader in Original mode; time document insertion relative to navigation and inspect the network for chapter-content requests.

### Observed results

| Measurement | Result |
|---|---|
| Warm reopen, Original | 392 / 384 / 389 ms |
| Warm reopen, Traditional | 737 / 595 / 592 ms |
| Chapter-content GETs in those six warm reopens | 0 |
| Full reload, Original | 1,055 ms; 0 chapter-content GETs (IndexedDB reuse) |
| Small book/source/progress requests in warm samples | approximately 128–139 ms |
| Catalog size | 103,729 bytes, 1,137 entries |
| Catalog on warm reader entry | 239–262 ms |
| BookSource metadata, after catalog completes | 130–131 ms |
| Blocking conversion stage, Traditional | 193–327 ms; catalog/document conversions run in parallel, not additive |
| Direct catalog GET totals | 529 / 144 / 143 ms |
| Direct catalog time to headers | 147 / 143 / 142 ms |
| Direct catalog body-read time | 382 / 1 / 1 ms |
| Direct catalog JSON parse | 0.2 / 0.2 / 0.1 ms |

The first nonrepresentative/background entry still provided a resource-only observation: its catalog took 517 ms (142 ms request-to-response-start, 369 ms response-body interval). Its foreground-display/cache-commit timing is excluded.

Every inspected catalog response sent `Cache-Control: private, no-store`; none advertised `ETag`, `Last-Modified`, `Content-Encoding`, `Vary` or `Server-Timing`. Encoded and decoded resource sizes were equal: the response was uncompressed. All measured catalog responses were HTTP 200, not conditional 304 responses.

The warm Traditional path was:

```text
fresh book GET and full catalog GET in parallel (~248–262 ms)
  → BookSource metadata GET (~130 ms)
  → catalog + chapter conversion POSTs in parallel (~193–327 ms)
  → DOM-ready chapter
  → progress write and two forward-chapter conversion preparations (nonblocking)
```

The detail page started its own catalog GET before Continue Reading; the reader started another rather than joining/reusing it. The detail-page request took 395–403 ms in the recorded cycles and sometimes overlapped the reader request.

The full reload also performed setup status (~135 ms), followed by account/registration checks in parallel (~130 ms), before reader entry. It did not download chapter text. This separates startup cost from the persistent chapter cache.

### Limits

- This is one authenticated browser/network/book, a small sample, and warmed repeated interactions. It does not establish a percentile or reproduce every reported 1–2 second reopen.
- The deployed profile originally used Original, not the user's reported Traditional setting on their usual reader; both were measured explicitly.
- Resource Timing separates time to response headers, body reception and frontend parsing, but **does not isolate backend CPU/SQLite time from network, proxy, upload or buffering time**. No server-side tracing was installed. Do not call the entire catalog duration database time or attribute body variation to a specific network mechanism.
- Backend source inspection shows a populated catalog is read from SQLite, not recrawled on every GET. Empty/syncing catalogs take a different path; that was not benchmarked.
- No source switch, restore, deletion, TTL expiry or cache-limit mutation was performed on live user data. Those protections were inspected, not live fault-injected.
- Reading the same location causes normal progress/last-read writes. No BookSource or chapter selection was changed. Temporary conversion preferences were restored. Diagnostic scripts were disposable and did not alter application source.

## Why requests still occur

Relevant source paths:
- `frontend/src/features/reader/reader-session.ts:18`: `loadReaderSnapshot()` always fetches current book and catalog before binding client cache identity.
- `frontend/src/features/reader/ReaderView.vue:262`: entry drains pending progress writes, loads that snapshot, then awaits `getBookSource()` for BookSource books before chapter lookup. It constructs a fresh display converter.
- `frontend/src/features/books/BookDetailView.vue:96`: detail independently loads book → BookSource metadata → catalog; no shared snapshot/request owner with reader entry.
- `frontend/src/features/reader/chapter-loader.ts:26` and `chapter-cache.ts:178`: chapter reads do use memory → IndexedDB → backend; a hit makes no HTTP request and will not appear as an HTTP disk-cache response.
- `frontend/src/features/reader/chinese-conversion.ts:53`: conversion memoization is reader-instance-owned and keyed by canonical objects. Reopening discards it, including converted catalog titles.
- `backend/internal/api/reader_api.go:83`: reader APIs default to `private, no-store`; browser HTTP caching is intentionally not a parallel authority for private, identity-sensitive JSON.
- `backend/internal/reading/booksource.go:28`, `backend/internal/book/catalog_sync.go:84`, `backend/internal/book/library_projection.go:70`: a ready catalog reads the full stored chapter set and projects it; the provider additionally qualifies the source definition. The handler returns the full JSON with no conditional reuse protocol.
- `frontend/src/features/reader/progress-writer.ts`: progress/bookmarks share ordered state-version writes; entry drains outstanding writes before obtaining fresh resume/version. The measured progress writes after display are not foreground blockers.

The existing contract explicitly requires online entry validation: see the completed [cache plan](../plans/2026-09-22-reader-cache-and-prefetch.md#identity-and-lifecycle). It does not require retransmitting all catalog entries or serially loading recovery metadata on every entry.

## Proposed direction — not yet accepted

Prefer a small, coherent reader-entry boundary over a generic HTTP cache or another independent cache framework:

1. **One fresh validation response on entry.** Return current resume/state version and cache qualification (reader/home, provider, book/content revision, source definition) together. Prefer extending the existing book/reading boundary rather than adding separate validation calls for every resource. Ensure qualification is coherent under concurrent source changes/restores; it cannot be assembled from unrelated stale reads.
2. **Retain catalogs with the same bounded reader cache owner.** Reuse the matching cached catalog after validation; fetch it only when missing/changed. Share pending catalog work between detail and reader. Existing content revisions identify catalog/interpretation replacement; source identity separately guards definition changes. No new revision system or arbitrary catalog TTL is needed just to avoid retransmitting unchanged catalogs. Metadata retention must remain bounded and must not make detail browsing retain unlimited books.
3. **Remove source-recovery metadata from the prose critical path.** Load it when needed by recovery/source UI, or include only genuinely required lightweight fields in the entry response. Preserve its revision checks and recovery behavior rather than merely removing the await.
4. **Retain memory-only conversion with retained canonical objects.** Reuse the existing converter at the cache/reader-account lifetime, not per mounted view. Catalog object identity must survive reuse too. Converted results inherit canonical eligibility and eviction; Refresh's new document must not reuse old converted text just because chapter/revision are unchanged. Do not introduce an independently aged or unbounded converted cache.
5. **Compress full text/JSON transfers where appropriate.** This helps cold/changed catalogs but cannot remove a 130–150 ms round trip. It is complementary, not the primary fix for warm reopening. Capability/startup requests are lower-priority opportunities; do not weaken authentication or expand this into whole-app caching without evidence and approval.

Expected warm shape, not a measured improvement:

```text
outstanding progress writes drain if any
  → one small authenticated current-state/qualification request
  → qualified catalog + chapter + converted display reused locally
```

On the measured connection, one small request has roughly a 130–150 ms floor, rather than several sequential stages. That is a design target, not a promised final latency. Full reload can still require authentication, disk access and conversion when memory-only display preparation is absent.

## Invariants to preserve

- Source switch and TXT reparse: old revision/catalog/ordinals and pending responses cannot become reusable.
- Source-definition edits: source identity mismatch invalidates related content and preparation even without an interpretation-revision change.
- Reader/home replacement, logout, account change and deletion retain current isolation/invalidation boundaries; retain the transactional IndexedDB epoch/write guard, not merely BroadcastChannel notifications.
- BookSource freshness remains the existing authoritative remaining 24-hour interval. Catalog validation, cache access, display conversion and disk promotion must never renew chapter TTL. Expired content cannot satisfy a new load; already displayed prose remains governed by its existing contract.
- Keep three retained books and the five-main-chapter window, nearest-two preparation and existing backend/resource limits. Tie catalog/conversion lifetime to those owners; no hidden unlimited secondary cache.
- Explicit Refresh still bypasses reusable content and retires relevant conversion results. No stale response can overwrite the refreshed entry.
- Preserve progress/bookmark state-version ordering and restoration, and do not count speculative preparation as reading.
- Preserve the no-store HTTP boundary; application-managed reuse is qualified deliberately, rather than silently caching authentication-sensitive URLs by URL alone.

**Zero-request entry is a separate consistency decision.** A client cannot discover a remote-device source change or restore with no communication. A short validation TTL would introduce a stale window; background validation can display stale content first; a pushed invalidation channel alone does not prove currentness across disconnects. None is assumed accepted here. One lightweight fresh validation preserves the current entry/resume guarantee without requiring full catalog retransmission.

## Next action

Review the proposed owner/API shape with the user before implementation. If accepted, create a focused implementation plan with cold/warm/reload timing baselines and deterministic invalidation, TTL, Refresh and retention regressions. No cache behavior change is authorized by this report.
