---
status: active
updated: 2026-08-31
---

# Decouple Shelf Admission from Catalog Synchronization

## Goal

Make adding a valid book independent of chapter-catalog size while preserving bounded source execution. A book should enter the shelf after Book Info establishes its identity and source binding; its complete chapter catalog should synchronize separately, exactly once per concurrent demand, and publish atomically when ready.

Also expose source-provided latest-chapter and update-time metadata consistently on Search, Explore, candidate preview, and stored book detail without requiring a TOC crawl.

## Scope

Included:

- change candidate admission from Book Info → full TOC → content verification to bounded Book Info validation and metadata-first shelf commit;
- introduce one cohesive catalog synchronization module that owns source lookup, one active synchronization per book, bounded concurrency, TOC parsing, atomic persistence, retry, and observable state;
- adapt the stored-book chapter endpoint and frontend book detail to represent catalog synchronization rather than holding one request open indefinitely;
- preserve source-provided `lastChapter` and `updateTime` through existing Search/Explore/Book Info data paths and render both when present;
- retain typed source failures and source-recovery behavior.

Excluded:

- a generic durable job framework or process-wide task API;
- partial/streaming publication of chapter lists;
- source-specific large-catalog exceptions;
- pretending full TOC construction can be sublinear in the number of output chapters;
- universal parsing or normalization of source-defined update-time strings;
- proactive background monitoring of every shelf book or BookSource;
- parser optimization before profiling the decoupled real workflow.

## Accepted Approach

### Separate metadata from the catalog

Search, Explore, and Book Info provide bounded summary metadata: identity, source binding, cover, description, `lastChapter`, `updateTime`, word count, and TOC URL when the source defines the corresponding rules. This data is sufficient for candidate preview and shelf admission.

The complete chapter list is a separate cached resource. Producing `N` chapter entries is inherently at least O(N), especially because arbitrary Legado JavaScript may generate, filter, reverse, or format the whole catalog. The system will move this unavoidable cost out of shelf admission rather than claim to eliminate it.

### Metadata-first candidate admission

Candidate resolution validates Book Info for source bindings in stable priority order. The first binding that returns usable book identity/source metadata wins. Candidate commit persists through `book.Store.AddOrMergeBook` without chapters.

TOC and chapter-content failures no longer classify whether a book may exist on the shelf. Catalog or content incompatibility is surfaced later as catalog/read availability with retry and source recovery.

### Catalog synchronization module

Add a feature-local backend module near the book domain with a small interface such as:

```go
type CatalogResult struct {
    State    CatalogState // syncing, ready, failed
    Chapters []book.Chapter
    Failure  *CatalogFailure
}

Get(context.Context, bookID string) (CatalogResult, error)
Retry(context.Context, bookID string) (CatalogResult, error)
```

The implementation hides:

- current book/source loading;
- one in-flight synchronization shared by concurrent callers for the same book;
- a small bounded global concurrency limit;
- TOC execution and cancellation;
- chapter ID assignment;
- atomic chapter replacement and `totalChapterNum` update;
- retryable versus terminal source failures;
- process-local in-flight state cleanup.

Do not introduce an adapter interface until a second real implementation exists. Tests may exercise the module through its public interface with existing concrete stores/fixtures.

### HTTP and frontend behavior

`GET /api/books/{id}/chapters` returns cached chapters immediately when ready. If no catalog exists, it starts or joins synchronization and returns a typed `202 Accepted` state rather than blocking for the complete crawl. A failed state returns structured workflow/retry information. An explicit retry action restarts a failed synchronization; it does not create a generic jobs resource.

Book Detail loads book metadata independently from catalog state. It can render the book immediately, show `Synchronizing catalog…`, poll with bounded backoff while syncing, render chapters when ready, and offer retry/source recovery after failure.

Candidate preview no longer embeds a complete chapter array. It shows metadata and communicates that the catalog will synchronize after shelving.

### Metadata presentation

The existing fields and rules remain canonical:

- Search and Explore: `ruleSearch.lastChapter`, `ruleSearch.updateTime`;
- Book Info/candidate preview: `ruleBookInfo.lastChapter`, `ruleBookInfo.updateTime`;
- shelf persistence: `Book.LastChapter`, `Book.UpdateTime`.

Render the values on the shared Search/Explore result card, candidate preview, and stored Book Detail. Omit missing values. Keep source text intact; localization supplies labels, not date reinterpretation.

`lastChapter` and `updateTime` may later be used together as a non-authoritative freshness hint. They must not invalidate or refresh a catalog automatically in this work because sources may omit them or use inconsistent formats.

## Decisions

### Shelf admission guarantees metadata, not readability

**Decision:** Add a book immediately after Book Info validates identity and source binding.

**Why:** The user explicitly selected metadata-first admission. It makes add latency independent of chapter count and gives all books one lifecycle instead of special timeout behavior for large catalogs.

**Alternatives:** Waiting for full catalog verification preserves the old readability guarantee but keeps O(N) work on the admission path. Offering both modes adds lifecycle and UI complexity without a demonstrated need.

**Revisit when:** Product requirements demand a strict guarantee that every shelf entry is readable before insertion.

### Complete catalogs remain O(N) and atomically published

**Decision:** Parse complete catalogs in the synchronization module and publish only after success.

**Why:** Arbitrary source rules prevent a general sublinear or safely streaming implementation. Atomic publication keeps readers, bookmarks, progress, and source switching away from partial catalogs.

**Alternatives:** Streaming chapters into the active table complicates reversal, deduplication, formatting, failures, and existing reading-state invariants. A bounded probe cannot prove full-catalog compatibility for arbitrary JavaScript.

**Revisit when:** A measured source family exposes a stable page/cursor contract that supports a separate virtualized catalog without weakening Legado compatibility.

### Process-local synchronization before durable jobs

**Decision:** Use single-flight process-local synchronization with idempotent persistence and on-demand retry.

**Why:** Restart recovery is naturally “no cached catalog, request starts again.” A durable queue would add schema, lifecycle, cleanup, and API complexity without preserving unique user work.

**Alternatives:** Durable jobs become justified only if synchronization must continue across restarts or run without user demand.

**Revisit when:** Operators need scheduled refresh, guaranteed background completion, or durable progress across restarts.

### Metadata is a hint, not catalog truth

**Decision:** Display source-provided latest chapter/update time but do not derive chapter count, ordering, or automatic invalidation from them.

**Why:** These rules are optional free-form strings and may be stale or inconsistent.

**Revisit when:** A normalized source metadata contract is established with evidence across representative sources.

## Progress

- [x] Diagnose full TOC materialization as the chapter-count-dependent admission bottleneck.
- [x] Confirm existing models and parsers already carry `lastChapter` and `updateTime` across Search, Explore, Book Info, candidate preview, and shelf persistence.
- [x] Accept metadata-first shelf admission.
- [x] Change candidate resolution and commit interfaces to metadata-first behavior.
- [x] Implement catalog synchronization and typed HTTP state.
- [x] Adapt Book Detail and candidate preview to the new catalog lifecycle.
- [ ] Render latest chapter and source update time consistently.
- [ ] Profile the real large catalog after decoupling; optimize only demonstrated parser/persistence hotspots.

## Current State

Candidate admission is now metadata-first. Each binding runs only bounded Book Info; concurrent checks preserve stable source priority by holding successful lower-priority results in an observable `ready` state until every preferred binding has completed or failed. Only the selected binding becomes `verified`. Candidate preview is metadata-only, and commit uses `book.Store.AddOrMergeBook`, so full TOC size and chapter content no longer affect shelf admission.

Each reader runtime now owns a concrete `book.Catalogs` coordinator. Cached chapters return directly from SQLite; a missing catalog starts or joins one of at most two bounded crawls for that reader; successful catalogs and `totalChapterNum` publish in one transaction; successful entries leave process memory; failures remain observable until explicit retry; and runtime/server closure cancels and drains active work before closing Reader Data. Book deletion and active-source switching invalidate and drain that book's old crawl before mutation for prompt cleanup. Correctness does not depend on cancellation: catalog publication verifies the book's source ID and state version inside the same SQLite transaction, so stale work cannot replace a switched source's catalog. `GET /books/{id}/chapters` returns `200`, `202 syncing`, or a typed failure; `POST /books/{id}/chapters/sync` retries retained failures without force-refreshing ready catalogs.

`SearchResult`, `PreviewBook`, and `Book` already contain `LastChapter`/`UpdateTime`. Search/Explore parsing reads `lastChapter` and `updateTime`; Book Info refreshes both. The shared result card and detail views show latest chapter but not update time.

## Next Action

Run one real `光遇聚合` large-catalog verification from the UI: shelf admission must complete after Book Info without waiting for the TOC, Book Detail must remain usable while catalog synchronization reports `202`, and the catalog must eventually publish atomically or show a truthful retryable failure. Record source/title/timing evidence and any compatibility failure before deciding whether this workstream is complete.

## Verification

Verified during design:

- working tree clean after the interrupted inference;
- Search/Explore parser extracts `lastChapter` and `updateTime` when rules exist;
- Book Info parser extracts the same fields;
- backend/frontend DTOs and shelf schema already carry both values;
- Search and Explore share one result-card presentation module;
- existing `AddOrMergeBook` persists metadata without replacing an existing logical book's active source, chapters, progress, bookmarks, or cache;
- existing chapter endpoint lazily fetches missing catalogs, providing a concrete seam to deepen rather than inventing a parallel workflow.

Completed verification for metadata-first admission:

- complete candidate package passes;
- targeted candidate API tests pass;
- candidate frontend tests and locale-key symmetry pass;
- frontend typecheck and production build pass;
- a non-terminating TOC rule does not participate in admission;
- stable-priority regression proves lower-priority metadata may become `ready` but is not selected before a preferred source completes;
- no candidate preview or commit carries a chapter catalog.

Completed verification for catalog synchronization:

- cached chapters return without a crawl;
- concurrent missing-catalog requests share one crawl;
- chapter rows and `totalChapterNum` publish atomically;
- failed crawls remain stable until explicit retry;
- terminal state is published only after worker cleanup, preventing immediate-retry overlap;
- polling consults in-memory state before SQLite, preventing synchronization read/write contention;
- runtime closure cancels and drains active crawls;
- book deletion/source switching invalidate and drain stale per-book work before mutation;
- catalog publication rejects a changed source ID/state version transactionally, so cancellation races cannot publish stale chapters;
- GET/POST endpoint success, `202`, retry, not-found, storage, and typed pagination failures pass.

Completed verification for frontend catalog lifecycle:

- the feature-specific reader API distinguishes `200` chapter arrays from `202 syncing` without changing the global transport helper;
- explicit retry starts with `POST /chapters/sync` and then polls the normal GET endpoint with bounded backoff;
- Book Detail renders metadata while catalog synchronization is still pending and updates when chapters become ready;
- Reader and source switching wait for the same catalog path and use the existing generation guard to ignore stale results;
- all 46 frontend test files pass;
- locale key symmetry, Vue/TypeScript typecheck, and the production Vite build pass.

Still needed:

- one real `光遇聚合` large-catalog verification showing immediate shelf admission followed by successful or truthfully failed catalog synchronization.
