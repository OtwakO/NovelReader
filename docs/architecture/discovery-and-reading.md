# Discovery, Shelf, Catalogs, and Reading

**Status:** Current architecture

## Discovery

Search fans out over effectively enabled installed BookSources in deterministic batches and streams source results/failures over SSE. Explore executes one effectively enabled source's native catalog, controls, and pagination. Both return typed result metadata and opaque source bindings; the frontend does not inspect source rules or source-specific payloads.

For discovery, a source is effectively enabled only when its own setting is enabled and either it is standalone or its Source Collection is available. Explore additionally requires the source's Explore setting and capability. Collection availability is a separate persisted policy: pausing a collection removes its members from Search and Explore without rewriting their individual settings, and re-enabling restores those saved states. The gate does not revoke existing shelf bindings or reading access.

A Search/Explore result may represent a logical book already on the shelf. The backend annotates results from SQLite using normalized title/author identity so the frontend routes to the stored book instead of offering a duplicate shelf entry.

## Source management

The source list returns compact management summaries, not full rule/script/header definitions.
Opening the raw editor fetches one lossless definition through `GET /api/sources/{id}`. Enable and
Explore toggles use a narrow `PATCH`; full `PUT` remains deliberate definition replacement. A list
summary must never be submitted as a replacement definition, because omitted rules would be lost.

Collection mutations capture prior Source IDs in the storage transaction. Manual and scheduled changes invalidate the same source-session/browser state after commit, even if credential cleanup fails. Cross-database cleanup failures report that the mutation committed; idempotent reconciliation retries on runtime startup and scheduler sweeps, including readers with no remaining collections.

## Covers

Cover display URLs are backend-issued and reader-scoped. Stored-book URLs carry a version query;
transient discovery references carry a signed revision. Revisions change with cover inputs, the
installed source definition, and independent source settings/authentication generations. Shelf lists
batch revision lookup rather than loading a source definition per book.

Responses use `Cache-Control: private, max-age=604800`, `Vary: Cookie`, and `nosniff`, without
`immutable`. Invalidation changes newly issued URLs; previously issued valid signed references remain
servable. Upstream bytes changing at an otherwise unchanged URL may remain cached for seven days.

The shared `BookCover.vue` keeps the full cover visible in a 3:4 frame. Nonstandard aspect ratios get
a subdued image-derived backdrop; callers own sizing/framing rather than duplicating cover rendering.

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

## Reading documents and resources

The current BookSource path opens each chapter as a versioned **Prose Document** containing ordered paragraph and inline-image blocks. This is an explicit current modality, not a universal media-block model: future image-sequence or audio reading should add their own Reading Document and renderer behavior behind [decision 0002](../decisions/0002-reading-documents-and-resources.md).

Inline-image blocks expose only opaque NovelReader-controlled Content Resource references. Source image origins remain backend-only in the bounded chapter cache. Authenticated chapter-image endpoints resolve remote resources from the active Exact Source Binding with source headers, cookies, request options, sessions, and portable decoding; bounded `data:image/...` resources are decoded locally through the same resource path. Existing text-only cached chapters without stored blocks are translated into paragraph blocks at the response seam rather than requiring a cache migration.

The frontend Reading Session owns chapter loading, navigation, common chrome, recovery, and progress coordination. A focused prose renderer owns paragraph and inline-image presentation. Images are responsive and centered; meaningful source alternative text is used accessibly and shown beneath the image as a centered caption. An image failure remains local to its figure and does not replace readable chapter prose.

## Reader state

Stored books own:

- the active source binding and catalog;
- chapter/index and normalized in-chapter progress;
- bookmarks, including explicit orphan state after source migration;
- bounded server-side processed chapter cache;
- source-provided latest chapter/update metadata.

Typography, Chinese conversion mode, image visibility, wake lock, and prefetch preferences are
browser-local settings shared across books, not fields on a stored book or part of its portable
backup. Their current localStorage key is not Reader Account-specific; do not describe these UI
preferences as server-synchronized or account-isolated.

The Reader waits on the same catalog synchronization interface as Book Detail and source switching.
Generation guards prevent stale catalog/content responses from replacing newer navigation or source
state. Converted text and its chapter identity commit together, so progress describes visible content.

The Reading Session retains up to five recent online chapter documents and deduplicates pending
loads. Default-on prefetch requests only the next readable chapter after display; it does not recurse,
load images, or save progress. Speculative and foreground fetches are serialized because source
scripts share mutable session state. Offline fallback documents are not retained in this session cache.

Progress writes remain ordered but their acknowledgements no longer block chapter display. Bookmarks
and source switching retain progress barriers. Source switching and explicit Refresh discard chapter
and conversion reuse and drain started requests before loading new content, preventing late source
state writes. Unmount aborts outstanding requests; cancellation reaches the backend chapter workflow.
Refresh preserves position and still reports an explicit offline copy if the upstream source fails.

Catalog/chapter display conversion is memoized by original object identity and conversion mode within
the Reading Session, without mutating canonical content or introducing a cross-reader cache. The
three-dot menu contains Bookmarks and Refresh; the settings sheet owns the prefetch toggle. Disabling
images removes their figures, captions, placeholders, and image requests rather than merely hiding pixels.

The Vue frontend owns presentation and interaction: shelf filtering/sorting/restoration, TOC filtering/ordering/current positioning, keyboard/tap navigation, wake lock, typography, overlays, responsive behavior, and modality-specific rendering. It never crawls or evaluates source rules and does not reconstruct provider resource locations.

## Failure model

- BookSource failures are typed by workflow and remain distinct from storage/not-found failures.
- Parser/transport failures never become successful empty results.
- Opaque `data:` payloads remain backend-only and are redacted from user-visible crawl errors.
- External source/gateway failure may leave a shelf book without a ready catalog; the book remains valid metadata with explicit retry and source-recovery actions.

## Logical-book identity contract

Shelf identity and frontend recovery matching follow backend `NormalizeBookIdentity`: lowercase individual Unicode characters, trim Unicode whitespace, remove the exact author prefixes/suffixes in that implementation, then retain letters and numbers. Whitespace before the author-label colon is not accepted as a prefix. Browser locale and contextual casing must not change this identity. Discovery's looser result grouping remains a separate contract.

Both implementations run the shared synthetic cases in `testdata/book-identity.json`. The backend's persisted identity behavior is unchanged.
