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

- A library ID identifies an independent publication. Shared admission does not merge display names/authors.
- Within BookSource acquisition only, normalized `(title, author)` identifies one logical BookSource shelf book.
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

Catalog availability is separate from shelf existence. The synchronization workflow below is
BookSource-owned; TXT reads its already-published index without crawling or reanalysis.

- Cached chapters are read from SQLite.
- A missing catalog starts or joins one active synchronization for that book.
- Each reader runs at most two catalog crawls concurrently by default.
- Full TOC parsing has a bounded deadline and cancellation checks through fetch, extraction, deduplication, and title formatting.
- Chapters and the library-owned `totalChapterNum` publish atomically only if the book still has the source ID/content revision the crawl started with. Progress and bookmark changes do not invalidate a catalog crawl.
- Successful catalog state leaves process memory; failures remain observable until explicit retry.
- Book deletion and source switching invalidate/drain old work for prompt cleanup, while the transactional source/version guard provides correctness.

`GET /api/books/{id}/chapters` returns `{chapters, contentRevision}` for both providers. Common
chapter entries contain only `index`, `title`, and `isVolume`; native IDs, URLs and file paths stay
behind the reading interface. BookSource may instead return `202` synchronization state or a typed failure. `POST /api/books/{id}/chapters/sync` retries a retained failure; it does not force-refresh an already ready catalog.

See the completed [catalog synchronization plan](../plans/2026-08-31-catalog-synchronization.md).

## Reading documents and resources

`backend/internal/reading` coordinates provider selection, catalog/document adaptation and
revision-qualified locations above the native stores. Its private provider interface covers only
catalog lookup, document opening and chapter lookup; acquisition, source management and file
removal are separate operations. HTTP still owns authorization and issues image resource URLs.

Both BookSource and TXT open chapters as versioned **Prose Documents**. BookSource supplies ordered
paragraph and inline-image blocks; TXT reads one saved byte range and supplies literal paragraphs,
without passing the original through HTML extraction or source-specific text cleanup. This is an explicit current modality, not a universal media-block model: future image-sequence or audio reading should add their own Reading Document and renderer behavior behind [decision 0002](../decisions/0002-reading-documents-and-resources.md).

Inline-image blocks expose only opaque NovelReader-controlled Content Resource references. Source image origins remain backend-only in the bounded chapter cache. Authenticated chapter-image endpoints resolve remote resources from the active Exact Source Binding with source headers, cookies, request options, sessions, and portable decoding; bounded `data:image/...` resources are decoded locally through the same resource path. Existing text-only cached chapters without stored blocks are translated into paragraph blocks at the response seam rather than requiring a cache migration.

Content requests carry the catalog revision and responses repeat it. Superseded requests are rejected
before upstream execution and snapshots are revalidated after the fetch. Cache admission checks the
current source and revision transactionally; an old response cannot replace a newer cached entry.
Image references carry the interpretation revision and cannot resolve images from a later catalog.
The frontend loader is bound to one book/revision and rejects mismatched responses before retention
or display.

The frontend Reading Session owns chapter loading, navigation, common chrome, recovery, and progress coordination. Its candidate structured-content lifecycle commits revision-bound note visits with displayed content, retains a transient nested return stack, and restores scoped anchors before permitting progress. Missing anchors roll back the prior view. Same-main-section note visits do not update main progress, and focused links/tables retain native interaction. Client transport admits version-1 and structured version-2 prose through separate strict parsers. A shared Go-projected synthetic fixture and composed desktop/mobile browser checks cover this boundary; current backend providers still emit version 1 and EPUB provider/resource integration remains unfinished in the [EPUB plan](../plans/2026-09-17-epub-support.md). A focused prose renderer owns paragraph and inline-image presentation. Images are responsive and centered; meaningful source alternative text is used accessibly and shown beneath the image as a centered caption. An image failure remains local to its figure and does not replace readable chapter prose.

## Reader state

`library` owns publication IDs, provider discrimination, display metadata, catalog summaries,
revision-qualified chapter/index and normalized in-chapter progress, and bookmarks. BookSource owns
active/alternate bindings, native chapters and the bounded processed chapter cache. TXT owns its
published byte-range index and managed original. The reading module validates each provider's
readable chapter identity before library CAS commits progress or bookmarks. Empty title metadata is
valid: bookmarks retain it, and the UI uses the one-based section number when no title is available. Catalog sections may carry
`auxiliary: true`: these remain addressable/bookmarkable but cannot commit main progress. Omission
means main membership; section indices are not renumbered when selecting main reading order.
The frontend also excludes auxiliary sections from ordinary navigation, prefetch, saved-resume
fallback and local progress updates. Current TXT/BookSource providers do not emit auxiliary sections;
the unexposed EPUB projection and note-return work are tracked in the [EPUB plan](../plans/2026-09-17-epub-support.md).
Optional catalog `navigation` is a contents hierarchy, never the reading sequence. Its authored-versus-section-list provenance, grouping/unavailable entries and revision-qualified targets survive client parsing. Reader and Book Detail share an expanded semantic outline with ancestor-preserving search; selection reuses qualified reader links or the existing anchor/note lifecycle. Display conversion preserves canonical labels and targets. Legacy catalogs retain the flat TOC. Current providers omit it, while the unregistered EPUB projection is covered by a shared Go/frontend wire fixture.
There is no shared section table or duplicate shared metadata in BookSource storage.

`GET /api/books` and `/api/books/{id}` return shared library fields plus optional cover/display-label
enrichment. `/api/books/{id}/booksource` exposes the combined BookSource-context projection separately.
Shelf metadata and native display inputs are read in one SQLite snapshot with a fixed number of
queries; source cover revisions remain batched. Native bindings are not required on a generic item.

`library_items.last_read_at` / JSON `lastReadAt` is the server UTC Unix-millisecond time of the
last accepted reading-progress write; zero means never read. It is library-owned, shared by TXT and
BookSource projections, and preserved in portable data. The Reader queues progress after displaying
a main chapter even at the initial/unchanged position, then through its existing progress lifecycle.
Admission, metadata/catalog updates, bookmarks, source switching, reparse and content fetch/prefetch
do not independently change this timestamp. `updatedAt` remains generic modification metadata.
Continue Reading uses only positive `lastReadAt`; recently-read sorting orders by it, then creation
time (unread books follow read books). No clock is inferred from position or state version. Future
relative-time displays can format this same public field without another stored value or event log.

Content revision identifies a catalog/interpretation; state version orders progress and bookmark
mutations. Catalog publication advances the former, progress and successful bookmark add/delete
advance the latter, and source switching validates both before atomically replacing the interpretation
and relocating reading state. Unresolved bookmarks retain their old location revision and are marked
orphaned; deleting them guards current library state, not the orphan's old location revision.

Typography, Chinese conversion mode, image visibility, wake lock, and prefetch preferences are
browser-local settings shared across books, not fields on a stored book or part of its portable
backup. Their current localStorage key is not Reader Account-specific; do not describe these UI
preferences as server-synchronized or account-isolated.

The Reader waits on the same catalog synchronization interface as Book Detail and source switching.
Generation guards prevent stale catalog/content responses from replacing newer navigation or source
state. Converted text and its chapter identity commit together, so progress describes visible content.

The Reading Session retains up to five recent online chapter documents and deduplicates pending
loads. Default-on prefetch requests only the next main readable chapter after display; it does not recurse,
load images, or save progress. Speculative and foreground fetches are serialized because source
scripts share mutable session state. Offline fallback documents are not retained in this session cache.

Progress and bookmark mutations share one per-book queue and state-version owner. Progress
acknowledgements do not block chapter display. Bookmark capture snapshots its revision-qualified
location before awaiting progress; source switching drains that same mutation queue. Source switching and explicit Refresh discard chapter
and conversion reuse and drain started requests before loading new content, preventing late source
state writes. Unmount aborts outstanding requests; cancellation reaches the backend chapter workflow.
Refresh preserves position and still reports an explicit offline copy if the upstream source fails.

Catalog/chapter display conversion is memoized by original object identity and conversion mode within
the Reading Session, without mutating canonical content or introducing a cross-reader cache. The
three-dot menu contains Bookmarks and Refresh; the settings sheet owns the prefetch toggle. Disabling
images removes their figures, captions, placeholders, and image requests rather than merely hiding pixels.

The Vue frontend owns presentation and interaction: shelf filtering/sorting/restoration, TOC filtering/ordering/current positioning, keyboard/tap navigation, wake lock, typography, overlays, responsive behavior, and modality-specific rendering. It never crawls or evaluates source rules and does not reconstruct provider resource locations.

## Publication removal

The common delete route dispatches to the owning lifecycle. TXT removal first hides the library
item and cascades its bookmarks, then deletes owned bytes/index/receipt. Cleanup failure returns
`status: removed` with warning `txt_cleanup_pending`; it is not a claim of complete byte deletion.
Book Detail retains a focused result screen and retry control rather than navigating away and
losing the warning. Retrying the same book ID reaches the retained removal record even when the
shelf row is gone. Pending unpublished acquisitions cannot be discarded through this route.
Unrecognized providers remain explicitly unsupported; unrelated files are never swept.

## Failure model

- BookSource failures are typed by workflow and remain distinct from storage/not-found failures.
- Parser/transport failures never become successful empty results.
- Opaque `data:` payloads remain backend-only and are redacted from user-visible crawl errors.
- External source/gateway failure may leave a shelf book without a ready catalog; the book remains valid metadata with explicit retry and source-recovery actions.

## Logical-book identity contract

BookSource shelf merging and frontend source-recovery matching follow backend `NormalizeBookIdentity`: lowercase individual Unicode characters, trim Unicode whitespace, remove the exact author prefixes/suffixes in that implementation, then retain letters and numbers. Whitespace before the author-label colon is not accepted as a prefix. Browser locale and contextual casing must not change this identity. Discovery's looser result grouping remains a separate contract.

Both implementations run the shared synthetic cases in `testdata/book-identity.json`. The backend's persisted identity behavior is unchanged.

## Narrow chapter lookups

Chapter content reads fetch the exact chapter and its immediate catalog successor (including volume headings), together with the owning BookSource binding and library revision in one SQLite read transaction. The transaction ends before network work. Progress/bookmark validation uses that bounded snapshot to obtain the readable chapter and title.
TXT catalog and location lookups join only published receipts with indexed sections, with no file
read during progress/bookmark validation. The native queries use their chapter/section indexes. Full-catalog reads remain for catalog display and workflows that need the complete list. Missing indices are not silently advanced; shared progress CAS validates content revision and state version.
