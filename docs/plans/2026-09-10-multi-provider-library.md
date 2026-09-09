---
status: active
updated: 2026-09-10
---

# Multi-Provider Library and Imported Books Foundation

## Goal

Evolve NovelReader from a BookSource-only shelf into one unified library that can hold BookSource publications and imported publications without leaking provider-specific behavior across the library, reading workflow, or frontend.

The first concrete non-BookSource path will be TXT import with encoding handling, previewable TOC parsing, persisted sections, and reuse of the existing prose reading experience. The design must leave a direct path for EPUB and later providers without introducing a speculative plugin framework.

Done means:

- BookSource and TXT publications coexist as separate items in one library;
- shared library and prose-reading features are implemented once rather than per provider;
- BookSource-specific source bindings, catalog synchronization, recovery, rules, and sessions remain cohesive;
- TXT-specific file ownership, encoding, TOC analysis, indexing, and reparsing remain cohesive;
- the frontend receives provider-neutral reading sections/documents and does not interpret local files or BookSource rules;
- existing BookSource shelf and reading behavior remains supported and verified.

## Scope

Included:

- provider-neutral shelf-item identity and common library metadata;
- one shelf experience with origin filtering rather than independent shelf implementations;
- a small application-level seam for ordered sections, prose documents, and authorized resources once BookSource and TXT provide real variation;
- browser upload and reader-specific bind-mounted inbox ingestion, bounded TXT validation/encoding detection, asynchronous TOC analysis and preview, optional automatic addition of structurally ready results, reading, and later bounded reparsing;
- provider-owned persistence and lifecycle cleanup;
- shared progress, bookmarks, collections, reader presentation, and common shelf behavior where their contracts genuinely apply to every publication;
- focused compatibility and migration work required to preserve current BookSource behavior.

Excluded unless separately accepted:

- EPUB implementation in this workstream's first delivery slice;
- runtime third-party provider plugins or dynamic provider registration;
- one comprehensive provider interface covering Search, Explore, import, source recovery, and reading;
- automatic cross-provider edition merging by title and author;
- fixed-layout, image-sequence, or audio expansion;
- arbitrary JVM emulation, source-specific compatibility patches, or frontend content parsing;
- a separate top-level local bookshelf with duplicated library behavior.

## Accepted Approach

Follow [Normalize Reading into Documents, Resources, and Modality Renderers](../decisions/0002-reading-documents-and-resources.md), especially its Future Provider Adoption Reference.

1. Keep one provider-neutral library surface. Users navigate one shelf and may filter it by origin, initially `All`, `Source Books`, and `Imported Books` (final localized wording remains a UX implementation decision).
2. Separate provider kind from reading modality. BookSource, TXT, and EPUB are origins/containers; prose, image sequence, and audio describe reading behavior.
3. Keep shared shelf and reader state above providers. Provider modules supply and interpret content but do not each reimplement collections, progress, bookmarks, reading history, or common reader chrome.
4. Keep provider-native state in provider-owned storage. Do not add an expanding set of nullable TXT/EPUB/BookSource columns or use an untyped provider-data blob as the primary model.
5. Introduce only small consumer-owned capability interfaces demonstrated by the second real adapter. The first expected shared reading capabilities are ordered-section enumeration and opening a normalized Reading Document; opaque resource resolution remains a separate capability where real variation requires it.
6. Keep TXT TOC parsing inside a cohesive TXT module. The shared reading workflow consumes the published section index and never knows whether sections came from source rules, heading analysis, or a future EPUB navigation document.
7. Preserve exactly one durable full-content file for each imported TXT publication. Keep original bytes, detect and persist encoding, and index valid byte ranges in that file; do not create a second normalized full-file copy or extracted chapter files without measured need.
8. Publish parsing/index changes atomically in database state. A failed import or reparse must not replace an existing readable index, and confirmation must not copy or move content.
9. Treat backup, publication-file replacement, reparse application, and deletion as coordinated reader-home writes. A portable backup must pair its `reader.db` snapshot with the exact durable publication-file generation referenced by that snapshot.
10. Prefer an explicit compile-time provider dispatch switch initially. Introduce a registry only if real registration needs emerge.
11. Refactor existing BookSource code only where ownership must change for the first TXT vertical slice; do not reorganize the subsystem for symmetry.
12. Place future features by the narrowest domain that owns their invariant: library-wide organization in the library module; origin-neutral reading coordination in the reading workflow; typography/page/audio behavior in the modality document/renderer; and acquisition, parsing, synchronization, replacement, or recovery in the owning provider. When a feature is shared by only some providers, extract a small capability only after the second real implementation demonstrates the common contract.

## Decisions

### Use one library with origin filters

**Decision:** BookSource and imported publications share one shelf/library. Origin-specific views are filters and management contexts, not separate implementations.

**Why:** Readers need one place to find and continue books, while shared sorting, collections, progress, card presentation, and bulk behavior should not be duplicated.

**Alternatives:** Two independent bookshelves would make origin prominent but would duplicate common workflows and make mixed-library navigation harder.

**Revisit when:** Imported publications demonstrably require a fundamentally separate document-management product rather than a reading-library workflow.

### Keep provider and modality independent

**Decision:** Provider selection determines how content is obtained and interpreted; document kind determines how it is rendered and located.

**Why:** BookSource, TXT, and reflowable EPUB can all produce prose, while one provider may later produce more than one reading modality.

**Alternatives:** Rendering by file/provider type is initially direct but would spread origin checks through the frontend and duplicate prose behavior.

**Revisit when:** A real publication requires simultaneous primary modalities that cannot be represented by modality-specific sections, as described in decision 0002.

### Use capability-specific seams only after real variation

**Decision:** Do not create a universal provider/plugin interface. Extract the smallest reading capabilities when the TXT adapter and current BookSource implementation establish the second real case.

**Why:** This keeps interfaces deep, avoids unsupported placeholder methods, and lets Search, Explore, source recovery, TXT parsing, and file replacement remain with their actual owners.

**Alternatives:** A comprehensive provider interface offers apparent uniformity but couples every provider to irrelevant capabilities and capability flags.

**Revisit when:** Several implemented providers repeatedly require the same cohesive contract and explicit registration provides real operational value.

### Keep imported publications independently identified

**Decision:** Each imported file becomes its own provider-owned publication and shelf item by default. It is not automatically merged with BookSource books by normalized title and author.

**Why:** Files may be different editions, translations, volumes, revisions, or parsing configurations despite matching metadata.

**Alternatives:** Universal title/author merging reduces apparent duplicates but can silently combine unrelated editions and complicate file replacement and progress ownership.

**Revisit when:** Readers need an explicit cross-provider edition-linking workflow with defined progress and metadata rules.

### Keep one durable TXT content file

**Decision:** Preserve the uploaded TXT bytes as the publication's only durable full-content file. Persist the selected encoding and parser configuration, and store section boundaries as validated byte ranges in that original file.

**Why:** Opening one section remains a direct indexed lookup plus bounded seek/read/decode, independent of library size, while storage does not automatically duplicate every novel. Parsing establishes boundaries at valid decoder positions for supported encodings.

**Alternatives:** A second normalized UTF-8 file simplifies arbitrary seeking but approximately duplicates durable text storage. Extracted chapter files multiply filesystem objects and complicate reparse and cleanup. Decoding the whole book on every open makes latency depend on file size.

**Revisit when:** A demanded stateful encoding cannot safely support bounded decoding, or measurements show that selective derived data is required. Any exception should be format/encoding-specific rather than duplicating all imports.

### Keep durable files human-recognizable but machine-identified

**Decision:** Store TXT content under `files/publications/txt/<sanitized-original-name>--<short-id>/<sanitized-original-filename>`. Preserve the exact original filename in SQLite. The immutable full publication ID remains authoritative; filesystem names do not follow later metadata edits.

**Why:** Operators can recognize and copy publication files, while duplicate names, case-insensitive filesystems, unsafe characters, and mutable titles cannot break identity.

**Alternatives:** Opaque ID-only paths are simpler for machines but unnecessarily hostile to filesystem users. Title-only paths collide and turn metadata edits into risky filesystem operations.

**Revisit when:** A future storage backend cannot provide hierarchical human-readable paths; maintain the same publication identity and relative-reference contract.

### Analyze after claim; never during ordinary open

**Decision:** Browser uploads and inbox scans claim each file into its final durable publication path, return promptly, and run the default Automatic TOC analysis asynchronously with bounded workers. Ordinary book opening always consumes a published index and never performs a full-file TOC parse.

**Why:** Batch ingestion remains responsive, analysis resource use stays bounded, errors are discovered before reading, and opening book 1,000 has the same indexed/direct-file algorithm as opening book 2.

**Alternatives:** Synchronous upload parsing produces long requests and poor partial results. First-open parsing makes reading latency unpredictable and mixes provider processing into the common reader path.

**Revisit when:** Measurements show that eager analysis consumes unacceptable resources for a supported deployment; preserve the invariant that explicit preparation completes before an item is presented as normally readable.

### Support multiple TXT TOC methods behind one analyzer

**Decision:** TXT owns a typed, bounded set of TOC methods: Automatic, demonstrated heading presets, validated custom line regex, and a safe single-section fallback. Automatic may evaluate built-in detectors in one streaming pass. Persist the resolved method/configuration, parser version, and active index version.

**Why:** Users can correct diverse files without exposing TXT rules to shared library/reader code. Existing published indexes remain deterministic across parser upgrades.

**Alternatives:** One global rule cannot handle real TXT variation. Arbitrary scripts or a runtime parser-plugin system add security and maintenance cost. Manual chapter-by-chapter editing is disproportionate before real parser failures require it.

**Revisit when:** Real files cannot be represented by method selection or custom line matching and demonstrate a bounded need for manual structural overrides.

### Keep import review separate from library publication

**Decision:** Claimed files and analysis records may be pending, ready, needs-review, or failed without becoming library items. A batch can either review results before adding or explicitly enable `Automatically add ready books`; ambiguous and failed imports never auto-publish. Ready items can be bulk-added with one confirmation rather than confirmed individually.

**Why:** Large batches do not require repetitive approval, while parser confidence cannot silently publish questionable interpretations. Confirmation is a short per-publication database transaction and no file move.

**Alternatives:** Immediate processing shelf items pollute ordinary library states. Per-file confirmation is tedious. Blind automatic publication hides ambiguous TOCs.

**Revisit when:** Users require unattended recurring inbox ingestion; add an explicit account policy rather than changing the safe batch default invisibly.

### Treat the inbox as a consume queue

**Decision:** Each reader has a bind-mountable `files/inbox/`. Opening Import or explicitly scanning claims eligible stable regular files. Same-filesystem claims use atomic rename; cross-filesystem claims stream once through `files/work/*.part`, then atomically rename locally. A successful claim removes the inbox original. Temporary-suffix files are ignored.

**Why:** The user can drag in hundreds of recognizable files, while NovelReader gains stable ownership and avoids rediscovery. No filesystem watcher or periodic scanner is needed initially.

**Alternatives:** Leaving claimed files in the inbox requires durable dedup/acknowledgement semantics. Watching bind mounts is less portable and adds partial-copy races.

**Revisit when:** Read-only inboxes or non-consuming watched folders become an explicit product requirement with defined duplicate and acknowledgement behavior.

### Make managed publications portable backup state

**Decision:** Durable claimed files live under `files/publications/` and are included with their database records in portable reader backups. `files/work/` and the external-ingress `files/inbox/` are excluded. SQLite stores only rooted relative publication paths.

**Why:** A reader home remains movable and inspectable without host-specific paths. Pending claimed imports are protected; reconstructible work and potentially in-progress external copies are not archived.

**Alternatives:** Copying the entire file tree can archive unstable inbox files and disposable work. Excluding pending claimed files can lose NovelReader-owned uploads.

**Revisit when:** The backup product adds an explicit option to include unclaimed ingress files, with a defined stable-copy contract.

### Deletion owns complete publication cleanup

**Decision:** Removing an imported library item is one resumable lifecycle operation. Mark the item/provider state `deleting` with its owned relative directory, hide it from normal library/reading paths, drain reads/analysis, atomically retire the directory under `work/delete-<operation-id>`, remove the bytes, then transactionally delete common state, TXT state, section/index state, and the deletion record. Startup resumes incomplete deletion before serving that item. Failure is explicit and retryable—no successful deletion response while owned bytes or database ownership remain.

**Why:** Imported content is large durable state and cannot be orphaned silently. The short durable deletion state bridges the fact that filesystem operations and SQLite cannot share one atomic transaction. One publication owns one directory, so cleanup remains cohesive without global reference counting or general garbage collection.

**Alternatives:** Database-first deletion risks unreachable files. Unrecorded file-first deletion risks rows that point to missing content after failure. A permanent user-visible trash hierarchy or content-addressed object store adds lifecycle complexity before demonstrated need.

**Revisit when:** Exact-byte deduplication or shared resources are introduced; ownership and reference counting would then require a separate accepted design.

## Progress

- [x] Confirm the unified-library, provider-owned implementation direction.
- [x] Create the dedicated feature branch and durable implementation plan.
- [x] Map the current BookSource shelf, catalog, document, resource, progress, and deletion ownership against the intended seams.
- [x] Confirm the provider-neutral persistence seam, clean schema epoch, staged import lifecycle, bind-mounted inbox, asynchronous analysis, and batch-review direction.
- [ ] Define the smallest provider-neutral identity and persistence transition, including compatibility and rollback constraints.
- [ ] Define the concrete publication-file lifecycle, backup barrier/validation changes, and complete deletion contract.
- [ ] Define the first TXT vertical-slice contract and import states with realistic file/encoding/TOC failure cases.
- [ ] Implement the provider-neutral library/storage foundation with BookSource regressions.
- [ ] Implement TXT upload, normalization, TOC preview/confirmation, and atomic index publication.
- [ ] Connect TXT sections to the existing Prose Document and Reading Session path.
- [ ] Add unified shelf origin filtering and focused TXT management UI.
- [ ] Add bounded TXT reparsing and progress/bookmark relocation only after the basic import/read path is stable.
- [ ] Update current architecture documentation with the concrete implemented interfaces and storage ownership.

## Ownership Mapping Checkpoint

The current implementation is cohesive for one BookSource provider but its shared-looking types and routes encode BookSource invariants:

- `backend/internal/book/store.go` owns the `books`, `chapters`, `bookmarks`, and `chapter_cache` tables in one schema module. `Book` combines shelf metadata, progress, active BookSource binding, alternate bindings, and source-returned metadata. `Chapter` combines ordered-section fields with BookSource URL/rule fields.
- Logical title/author uniqueness is enforced directly on `books`. Search/Explore shelf annotation correctly depends on this BookSource identity and must not start matching imported publications implicitly.
- `book.Catalogs` is specifically a remote BookSource catalog synchronizer even though `/api/books/{id}/chapters` presents it as the only section path. An existing stored catalog bypasses synchronization; a missing one always requires a source and Searcher.
- Chapter opening, cover retrieval, image resources, offline cache, source switching, and source recovery directly load `SourceID` and BookSource state in `readerAPI` handlers.
- Progress and bookmark optimistic concurrency require `sourceId + stateVersion`; this is an active-source revision guard, not a provider-neutral reading-state contract.
- Deletion is database-transactional for BookSource rows but has no provider lifecycle/owned-file cleanup seam. TXT deletion will require preserving a cleanup reference until durable file removal succeeds or remains explicitly retryable.
- The frontend `Book` shape and `ReaderView` keep `sourceId`, source recovery, catalog polling, and progress writes in the common path. The existing Prose Document response and `ProseRenderer` are already suitable normalized reuse points.
- `readerstore.Home.Files()` is the correct reader-owned file seam and backups already copy all safe files beneath it. A dedicated imports subdirectory must be added through `readerstore`; feature code must not reconstruct reader-home paths.
- Reader schema initialization is an exact current-schema epoch, currently version 9. The project policy treats pre-public reader data as disposable, so an accepted clean schema replacement can use a version bump and explicit recreate guidance rather than speculative migration machinery.

### Proposed foundation correction — pending confirmation

Do not add TXT fields or a generic provider payload to the current `books` row. Split persistence by ownership while preserving the external `/api/books/{id}` identity and routes where useful:

1. A provider-neutral library record owns item ID, provider kind/reference, display metadata, common progress/bookmark state, counts, and timestamps.
2. A shared ordered-section record owns only provider-neutral section identity/order/title/readability and the provider-owned opaque section reference needed for dispatch. BookSource URL/rule context remains in BookSource-owned storage; TXT byte ranges remain in TXT-owned storage.
3. A BookSource publication record owns the current source binding, alternate bindings, BookSource metadata, source revision, and remote-cache identity. Existing Search/Explore logical merge continues to query only BookSource publications.
4. A TXT publication record owns canonical-file identity, encoding/normalization metadata, parser configuration, parse/index version, and lifecycle state. TXT section ranges remain TXT-owned.
5. Application-level library/reading workflow loads the library item, dispatches by provider kind, and exposes sections/documents/resources. BookSource catalog synchronization and TXT index publication are adapters behind that workflow, not branches in generic handlers.
6. Shared reading-state concurrency uses a provider-neutral `readingStateVersion`. Provider content revisions remain provider-owned and can invalidate/remap reading state through an explicit workflow. The frontend no longer submits `sourceId` for ordinary progress or bookmarks.
7. Keep the existing JSON `Book` response compatible during the transition only if doing so does not preserve source fields as mandatory common state. Prefer an additive origin discriminator and origin-specific nested details over an ever-growing flat structure.

This is a meaningful schema correction, but it removes the root coupling rather than stacking TXT branches on a BookSource-shaped table. Because the project is pre-public and reader schemas are exact-version validated, the proposed rollback/compatibility policy is a reader schema epoch bump with explicit disposable-home recreation, unless preserving current development data is now a product requirement.

### Proposed first delivery slices — pending confirmation

1. **Provider-neutral reading foundation:** storage ownership split, BookSource adapter, common section/document/progress/bookmark workflows, existing BookSource behavior retained through focused regressions. No TXT UI yet.
2. **TXT import and read:** upload to a staged reader-owned import area; bounded validation and UTF-8 normalization; deterministic TOC analysis preview; confirmation atomically publishes TXT publication, shared library item, and section index; existing Prose Renderer reads normalized section text.
3. **Unified library UX:** origin filters and origin-specific actions; BookSource recovery appears only for BookSource items. Incomplete imports remain import sessions rather than half-readable shelf items.
4. **TXT reparse:** candidate index, explicit preview, atomic replacement, and bounded progress/bookmark relocation after the basic vertical slice is stable.

## Accepted File and Import Lifecycle

```text
files/
├── inbox/                         user-managed, bind-mountable ingress
├── work/                          internal temporary operations; disposable
└── publications/
    └── txt/
        └── <readable-name>--<short-id>/
            └── <readable-original-filename>.txt
```

The immutable publication ID and database reference are authoritative; readable filesystem names are diagnostic and user-friendly. The exact original filename, content hash, byte size, selected encoding, TOC method/configuration, parser version, and index version live in TXT-owned database state. The content file remains at one stable relative path through analysis, review, publication, and reading.

### File movement

- **Browser upload:** stream once to `work/upload-<id>.part` while enforcing limits and hashing; close/validate; atomically rename to the final publication path; then create the pending import record.
- **Same-filesystem inbox:** atomically rename directly from `inbox/` to the final publication path; then create the pending record.
- **Cross-filesystem inbox:** stream once to `work/claim-<id>.part`, hash/validate, atomically rename locally to the final path, create the pending record, then remove the inbox original. Cleanup failure is reported and duplicate detection prevents silent re-import.
- **Analysis/review/confirmation/reading:** no file movement and no full-content copy.
- **Reparse:** read the same content file and build a candidate index; applying it changes versioned database state only.
- **Deletion:** durably mark the item `deleting`, drain publication work, atomically rename the owned directory into `work/delete-<operation-id>`, remove the retired bytes, then delete all database ownership and the deletion record. Startup resumes any interrupted operation before that item can be read or backed up.

The invariant is **file first, reference second** for creation: a crash may leave an unreferenced directory that reconciliation can safely remove, but must not create a database record pointing at absent content. Confirmation never moves the file.

### Backup integration

The current `readerstore.Manager.SnapshotHome` takes a SQLite `VACUUM INTO` snapshot and then copies all files. Imported publications require a coordinated generation so the snapshot cannot reference content that is concurrently replaced or deleted during file copying.

The implementation design must use the existing per-reader home lifecycle/lease mechanism rather than a provider-specific backup hook:

1. Block publication-file mutations for the target reader during the snapshot-and-durable-file-copy window; ordinary read-only reading may continue only if it cannot hold or observe a file generation being replaced/deleted. Prefer one reader-home durable-state read/write coordination seam rather than separate backup, import, and deletion locks.
2. Snapshot `reader.db`.
3. Copy only durable managed files, including pending claimed and published `publications/`; exclude `inbox/` and `work/`.
4. Validate that every publication path referenced by the database snapshot exists as a regular rooted file in the staged home and that no path escapes or traverses through a symlink.
5. Restore uses the existing staged-home validation and atomic reader-home replacement; imported content requires no absolute-path rewrite because references are relative.

Do not create a second backup system inside TXT. The reader-home backup owns the database-plus-files consistency contract for all future file-backed providers.

## Feature Placement Guide

Use this routing rule to prevent shared behavior from being duplicated or provider details from leaking outward:

| Feature kind | Canonical owner | Examples |
|---|---|---|
| Shared shelf/library behavior | Library | collections, favorites, sorting, filtering, reading history, display overrides |
| Shared reading-session behavior | Reading workflow | authorization, section navigation, progress, bookmarks, common failures, prefetch coordination |
| Reading-modality behavior | Document and renderer | prose typography and selection, image-page fitting and zoom, audio playback and time location |
| BookSource-only behavior | BookSource module | Search, Explore, source bindings, catalog fetch, source recovery, rules, sessions, login |
| TXT-only behavior | TXT module | upload validation, encoding, TOC parsing, text indexing, reparse, original-file handling |
| Shared by some providers | Small capability after the second real case | original-file export or file replacement if TXT and EPUB demonstrate one cohesive contract |

Provider modules normalize native behavior into Reading Sections, Reading Documents, Content Resources, and Reading Locations. Shared layers must not know TXT offsets, archive paths, BookSource URLs, source rules, cookies, or provider-native identifiers. The frontend selects rendering by document kind, not provider kind; provider-specific management remains in focused feature UI.

## Current State

The branch `feat/multi-provider-library` was created from clean `main` at `0702469`.

The current ownership mapping is complete and recorded above. It confirms that the Prose Document/renderer and reader-owned file store are reusable seams, while the current shelf, sections, progress, bookmarks, catalog route, and Reader orchestration encode mandatory BookSource identity. Adding TXT by flags and nullable fields would preserve the wrong ownership.

No production code, schema, HTTP interface, or frontend behavior has changed for this workstream yet. The proposed correction is to separate provider-neutral library/reading state from provider-owned BookSource and TXT state, retain one external item identity, and deliver the work in bounded foundation/import/UX/reparse slices. The user accepted a clean reader-schema epoch with disposable pre-public home recreation, separate pending import sessions, explicit/import-screen inbox scans, a consuming bind-mounted inbox, asynchronous default TOC analysis, bulk-ready review/optional automatic addition, one durable TXT content file, and complete owned-file cleanup. Concrete schemas/interfaces and the reader-home backup/delete coordination are the next design checkpoint before production edits.

## Next Action

Specify the concrete tables, consumer-owned interfaces, reader-home rooted streaming/file operations, backup generation barrier and staged validation, complete deletion workflow, HTTP transition, schema recreation instructions, and focused regression matrix for the provider-neutral foundation slice. Pause for confirmation if that design exposes another consequential ownership or portability tradeoff before editing production code.

## Verification

Verified:

- `git status --short --branch` showed clean `main` tracking `origin/main` before branching.
- `git switch -c feat/multi-provider-library` created the feature branch successfully on 2026-09-10.
- The accepted approach was checked against `docs/decisions/0002-reading-documents-and-resources.md` and its Future Provider Adoption Reference.

Still needed:

- No code tests or builds have been run because this checkpoint changes only planning and project routing.
- Current schema and caller impact were mapped across `backend/internal/book`, reader API/runtime/routes, `readerstore`, frontend shelf/API/Reader state, and focused storage/API tests. No code was changed and no test result is claimed from this read-only mapping.
- The user accepted an exact reader-schema epoch bump with explicit recreation of disposable pre-public reader homes; concrete version/recreation instructions remain to be implemented and verified.
- The existing portable backup path was inspected: `SnapshotHome` uses `VACUUM INTO` and then recursively copies the whole current `files/` tree. Imported files require a durable-file allowlist/exclusion policy, a reader-level mutation barrier, and staged reference validation; none is implemented yet.
- TXT file size, initially supported encodings, exact automatic/preset parser set, confidence evidence thresholds, custom-regex limits, and upload/analysis resource limits remain to be designed and measured.
- Hosted CI, release images, and browser workflows are outside this planning checkpoint.

## Open Questions

- Which exact imported-file mutations must share the reader-home backup write barrier, and can current reading safely remain concurrent while publication files are copied?
- Which initially supported encodings permit safe byte-range boundaries without a normalized duplicate, and what explicit behavior should unsupported/stateful encodings receive?
- What concrete structural evidence makes an Automatic TOC candidate eligible for `Automatically add ready books` without turning a heuristic score into an unexplained policy?
- What is the smallest compatible transition from the current title/author logical-book identity to provider-neutral shelf identity without unnecessary pre-public migration machinery?
- Which progress and bookmark anchors are required in the first TXT delivery, and which relocation behavior can wait for the reparse milestone?
