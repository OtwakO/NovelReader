# Discovery, Shelf, Catalogs, and Reading

**Status:** Current architecture

## Discovery

Search fans out over enabled installed BookSources in deterministic batches and streams source results/failures over SSE. Explore executes one selected source's native catalog, controls, and pagination. Both return typed result metadata and opaque source bindings; the frontend does not inspect source rules or source-specific payloads.

A Search/Explore result may represent a logical book already on the shelf. The backend annotates results from SQLite using normalized title/author identity so the frontend routes to the stored book instead of offering a duplicate shelf entry.

## Logical books and source bindings

- Normalized `(title, author)` identifies one logical shelf book.
- Exact `(SourceID, BookURL)` identifies a source binding beneath it.
- A binding carries the imported source identity plus source-returned display metadata such as source name/group, capabilities, discovery-query provenance, and the opaque `lastChapter` snapshot.
- Persistence keeps one explicit active binding and zero or more alternates.
- Source Recovery presents active, stored alternate, and newly discovered bindings as one known-source list. The active binding remains visible and cannot be selected again.
- Clearing and rescanning removes alternate/discovery state but preserves the active binding.

Source switching validates an already registered exact alternate through Book Info and TOC, migrates reading position by normalized chapter-title match with a documented nearest-index fallback, and atomically promotes the binding with chapters, progress, and bookmark state. Failure leaves the previous binding authoritative.

See the completed [source binding plan](../plans/2026-09-01-source-binding-state.md).

## Candidate and shelf admission

Search/Explore cards and Candidate Book Detail share one asynchronous candidate-resolution operation.

- Every deduplicated binding is queued in stable primary-first order.
- Up to five Book Info checks run concurrently and freed slots refill immediately.
- Lower-priority successful metadata waits in a `ready` state until preferred bindings complete or fail.
- The selected binding becomes `verified`; losing/untouched attempts become `skipped` rather than falsely unavailable.
- Direct card Add requests automatic commit. Candidate Book Detail requires an explicit Add action.
- Commit is idempotent and persists Book Info metadata and all known bindings without recrawling.
- Shelf admission does not require TOC or chapter-content validation.

Candidate operations are transient, reader-owned, bounded, reconnectable over SSE, and release their runtime lease on commit, cancellation, expiry, eviction, or shutdown.

## Catalog synchronization

Catalog availability is separate from shelf existence.

- Cached chapters are read from SQLite.
- A missing catalog starts or joins one active synchronization for that book.
- Each reader runs at most two catalog crawls concurrently by default.
- Full TOC parsing has a bounded deadline and cancellation checks through fetch, extraction, deduplication, and title formatting.
- Chapters and `totalChapterNum` publish atomically only if the book still has the source ID/state version the crawl started with.
- Successful catalog state leaves process memory; failures remain observable until explicit retry.
- Book deletion and source switching invalidate/drain old work for prompt cleanup, while the transactional source/version guard provides correctness.

`GET /api/books/{id}/chapters` returns ready chapters, `202` synchronization state, or a typed failure. `POST /api/books/{id}/chapters/sync` retries a retained failure; it does not force-refresh an already ready catalog.

See the completed [catalog synchronization plan](../plans/2026-08-31-catalog-synchronization.md).

## Reader state

Stored books own:

- the active source binding and catalog;
- chapter/index and normalized in-chapter progress;
- bookmarks, including explicit orphan state after source migration;
- bounded server-side processed chapter cache;
- source-provided latest chapter/update metadata;
- display-only Chinese conversion preferences and device-local Reader settings.

The Reader waits on the same catalog synchronization interface as Book Detail and source switching. Generation guards prevent stale catalog/content responses from replacing newer navigation or source state.

The Vue frontend owns presentation and interaction: shelf filtering/sorting/restoration, TOC filtering/ordering/current positioning, keyboard/tap navigation, wake lock, typography, overlays, and responsive behavior. It never crawls or evaluates source rules.

## Failure model

- BookSource failures are typed by workflow and remain distinct from storage/not-found failures.
- Parser/transport failures never become successful empty results.
- Opaque `data:` payloads remain backend-only and are redacted from user-visible crawl errors.
- External source/gateway failure may leave a shelf book without a ready catalog; the book remains valid metadata with explicit retry and source-recovery actions.
