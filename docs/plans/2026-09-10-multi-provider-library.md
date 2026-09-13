---
status: active
updated: 2026-09-14
---

# Multi-Provider Library and Imported Books

## Resume Here

This is the canonical handoff for this workstream; conversation memory and older plan revisions are not needed to resume.

| Need | Read / action |
|---|---|
| What the user has decided | [Confirmed product decisions](#confirmed-product-decisions-and-delivery-order), together with [prior preferences](#previously-confirmed-preferences). Do not restart that questionnaire. |
| Architecture and implementation authority | Methodical implementation is authorized; see [delivery steps](#delivery-steps). The [refined design](#refined-design-recommendation--proposed) still does not fix unexamined schemas or lifecycle mechanisms. |
| What exists and what to inspect | [Existing foundations](#existing-foundations-and-evidence), then [current state](#current-state) and [verification](#verification). Check Git before assuming the branch is unchanged. |
| What to do next | [Next action](#next-action) and [delivery steps](#delivery-steps). Continue the current bounded step; do not repeat the broad architecture review or start coding from a historical blueprint. |

The confirmed product choices supersede earlier alternatives in this document and Git history. Proposed architecture is not an extra feature checklist. If new code evidence conflicts with a requirement, report the specific conflict; do not silently change the requirement or implement an increasingly complex workaround.

## Goal

Add practical, responsive batch TXT import and reading, with a clean path to EPUB, shared library/reader behavior, understandable non-wasteful storage, portable backups, and complete removal. Preserve existing BookSource behavior without building an unnecessarily elaborate framework.

This document replaces the previous architectural blueprint with a requirements-led baseline. The user subsequently authorized methodical implementation under the quality constraints below. This does not adopt every proposed mechanism wholesale. The earlier proposal is available in Git (notably `f27c4f1`) as a second opinion, not an implementation constraint.

## Scope

The workstream covers TXT acquisition, interpretation, review, library admission, reading, backup, removal, and safe later reparsing. The core-before-advanced delivery order is confirmed below; listing a requirement here does not put every UI control into the first release.

EPUB is a future extension to consider when evaluating the design, not an implementation requirement for the first TXT slice. Existing BookSource search, source bindings, catalog synchronization, source switching/recovery, caching, and reading must remain supported.

## Requirements

### Library and reading

- Readers can find, manage, and resume BookSource and imported books through a coherent library experience.
- Common library and reading features should not be reimplemented per origin. Reuse progress, bookmarks, navigation, settings, and prose presentation where their contracts genuinely apply. Future organization features should have a clear owner, not become new scope merely because they are mentioned here.
- Origin-specific management remains available without burdening unrelated books with irrelevant actions.
- Content acquisition and reading modality are distinct concerns: importing a different container does not by itself justify another prose renderer.
- Do not silently merge different imported editions or combine them with BookSource books solely because title and author match.
- Preserve existing BookSource admission and reading behavior; TXT readiness rules must not become new prerequisites for adding a BookSource book.

### Acquisition and filesystem use

- Support browser uploads and reader-specific bind-mounted directory ingestion.
- Support batches of hundreds of TXT files, with consistent downstream behavior regardless of ingestion method.
- Users can drag/copy files into an understandable ingress directory. The application must explain the location and whether successful ingestion consumes originals.
- Account for partially copied files, retries, duplicate discovery, unreadable inputs, and inboxes on a different filesystem.
- Keep managed filenames/directories recognizable and practical to inspect or copy. Define clearly whether renaming, editing, or replacing managed files externally is supported.
- Avoid routine persistent full-content duplication. Temporary copies needed for safe transfer are a different cost and must be bounded and cleaned up.
- Storage and backups must be portable between deployments without depending on absolute host paths.
- Reader accounts must remain isolated; file operations and restored references must not escape the owning reader's storage.

### TXT interpretation

- Handle supported text encodings explicitly; expose uncertainty and unsupported input rather than silently corrupting text.
- Analyze chapter structure before ordinary reading, with a preview of headings and representative content.
- Let users correct unsuitable encoding or chapter interpretation and analyze again.
- Provide a usable path for text without recognizable chapter headings. That fallback must still respect reading resource limits.
- Distinguish technical failure from an ambiguous but potentially usable interpretation.
- Keep published interpretations stable across application/parser updates; do not silently reparse books on upgrade or normal opening.
- Support later reparse with preview and explicit application. A failed or cancelled attempt must not damage the currently readable book.
- Preserve progress/bookmarks where a reliable mapping exists; explain uncertainty rather than silently relocate them incorrectly. Follow the confirmed conservative reparse policy below; precise text anchors are not part of that delivery.

### Batch review and admission

- Provide timely per-file progress and actionable errors; users should not wait for the whole batch before inspecting or using completed results.
- Support bulk acceptance of suitable results instead of requiring hundreds of individual confirmations.
- Keep ambiguous and failed results from being silently auto-published.
- Allow individual review of encoding, chapter structure, sample content, and proposed title/author, with corrections before admission.
- Report partial success accurately. Failed items remain understandable and recoverable or removable.
- Pending imports must be distinguishable from ordinary readable library items and remain manageable across interruption/restart.
- Cancellation must stop further requested work safely and explain what has already completed; it is not an implicit rollback of an entire batch.

### User workflow reference

`Upload or scan completed files → receive durably → analyze asynchronously → preview/correct or bulk accept → library → ordinary reading`.

Acquired files can leave the inbox before becoming library items; acquisition, analysis, and admission are distinct outcomes. Ready results can be added while other files continue processing. Needs-review results expose an actionable interpretation choice; technical failures expose retry/remove, not a false success. Closing the screen does not cancel accepted server work, but an unfinished browser transfer is not yet durably acquired.

Later published-book reparse uses the same analysis/preview behavior: `choose method → analyze candidate → review impact → apply or discard`. Until Apply, the active interpretation remains readable. Exact screens and labels are not fixed by this reference.

### Responsiveness and scale

- Upload/import feedback remains responsive for large batches. Completed files can advance independently.
- Background processing has bounded concurrency and memory use and must not starve ordinary reading or interactive review.
- Expected deployment size is roughly 10–100 users per host. Account/browser-session count is distinct from simultaneously active server work; no throughput or latency guarantee is implied without a representative workload and host budget.
- Opening one section must not enumerate the library, scan all publication directories, or decode the entire novel.
- Large libraries must not require loading/rendering all books just to browse a page of results.
- Reading work must also be bounded for exceptionally long chapters or files with no detected headings; a file-range read is not inherently small.
- The intent of “1,000 books should read like two” is independence from total library size, not identical wall-clock latency under arbitrary hardware, file sizes, or concurrent load.
- Establish representative workloads and practical limits before claiming performance or scaling guarantees. Avoid speculative infrastructure or exhaustive benchmarking.

### Backup, removal, and recovery

- Integrate with the existing reader-home backup/restore system; do not create a parallel TXT backup product.
- A backup must contain a consistent database and the durable files it references, including application-owned pending imports.
- Define the ownership boundary for unclaimed inbox files and exclude disposable processing work from portable backups.
- Restored content must be usable without host-specific path rewriting and must preserve reader isolation.
- Removing an imported book or discarding a pending import must clean its owned files and related database state, without harming other books or shared account resources.
- Failures and interruption must not leave permanently unreachable content accumulating on disk. Cleanup must remain identifiable and retryable.
- Do not report complete deletion while required cleanup remains unfinished.
- Recovery must not delete the sole surviving original simply because its database reference was not committed before a crash.

## Engineering Criteria

Evaluate alternatives against these criteria rather than rewarding architectural symmetry:

- **Maintainability and readability:** a future reader can understand a change locally.
- **Cohesion and low coupling:** parsing, remote source behavior, library organization, and rendering each have clear ownership; shared workflows do not interpret provider-native mechanics.
- **Extensibility:** adding EPUB or another real capability should reuse meaningful contracts without forcing irrelevant methods onto every origin.
- **Simplicity:** use the smallest complete solution. No speculative plugin registry, generic storage framework, untyped universal provider payload, or layers whose only job is forwarding calls.
- **Correctness:** fix actual ownership/data-flow causes, not special-case known files or layer workarounds over broken invariants.
- **Efficiency:** avoid routine duplicate content, unbounded queues, whole-library work on reading paths, and processing that blocks interaction unnecessarily.
- **Proportional verification:** deterministic tests for real risks and shared contracts, not exhaustive combinations or architectural implementation details.
- **Controlled scope:** preserve unrelated behavior and stop for consequential tradeoffs. No broad cleanup or dependency changes merely to modernize code.

### Effort and abstraction guardrail

The user explicitly prioritizes balance: over-abstraction is as harmful as missing abstraction. Choose patterns per real problem, not from a preferred catalog. Use a function/conditional until variation, shared invariants, or lifecycle complexity earns a stronger structure; the ownership table does not mandate a package, interface, or class for every row.

The failure scenarios below bound normal supported behavior, not an invitation to design for every possible failure combination. Cover ordinary interrupted I/O, retries, and account lifecycle interactions. Do not broaden the contract to concurrent external editing, distributed writers, perfect encoding detection, elaborate relocation, or self-healing storage. Preserve data integrity and reader isolation without turning this into a general resilience project.

Use the fewest deterministic tests that cover the changed contract: normal behavior, a meaningful boundary/failure, and a regression where needed. Add an integration test when it proves a real transaction/file boundary, not one test per internal helper or one suite per scenario combination. Stop when that evidence is sufficient; do not repeatedly audit unrelated code or delegate routine work to subagents.

## Previously Confirmed Preferences

These preserve the discussion so the user does not have to reconstruct it. They are the default product direction, but supporting mechanisms must be revalidated. Discuss any material change rather than silently overriding a preference.

| Topic | Previous preference |
|---|---|
| Navigation | One library with origin filters rather than independent online/local shelves; final labels remain open. |
| Pending results | Reviewable imports separate from ordinary readable shelf items. |
| Discovery | Scan explicitly and when opening Import; no filesystem watcher or periodic scan initially. |
| Inbox ownership | Consume successfully claimed files; clearly warn users to retain a separate external copy if desired. |
| Analysis timing | Asynchronous after acquisition, never full-file TOC parsing during ordinary book opening. |
| Admission | Bulk-ready confirmation, plus optional automatic addition of suitable results. |
| Durable storage | Avoid a second routine full-content copy; prefer stable, human-readable managed names. |
| Backup | Include managed pending and published content; exclude temporary work and unclaimed ingress. |
| Schema compatibility | Disposable pre-public reader homes may be recreated if necessary; no speculative migration machinery. This is permission, not a requirement to replace the schema. |

Review controls discussed previously include result filters/search/sorting, bulk method changes, beginning/middle/end TOC samples, custom heading patterns, and single-section fallback. The confirmed decisions below establish generated-section handling and core/advanced sequencing; exact UI controls and engineering limits remain to be designed. Parsing methods do not require a parser framework.

## Existing Foundations and Evidence

The earlier code inspection identified the following relevant starting points; recheck affected paths when designing changes rather than repeat a repository-wide survey:

- `backend/internal/book/store.go` combines BookSource shelf identity, source state, chapters, and progress. Logical title/author merging is specifically a BookSource behavior.
- `backend/internal/api/reader_handlers.go` and `bookmarks.go` couple ordinary reading/progress/bookmark paths to BookSource state. The frontend reader and API types reflect that contract.
- Existing Prose Documents and the prose renderer offer a reuse point; no second TXT renderer has been justified.
- `backend/internal/readerstore/home.go` and `backup.go` already own reader-home file access and portable snapshot/replacement. Database/file consistency for imported content needs explicit design, not an assumption that sequential copying is sufficient.
- [Reading documents and resources](../decisions/0002-reading-documents-and-resources.md) is relevant existing guidance. It does not establish the exact new tables, package layout, or TXT interface.

These observations identify coupling and reuse opportunities; they do not prove that a wholesale shared-storage rewrite is necessary.

## Confirmed Product Decisions and Delivery Order

The user selected these after the refined-design discussion. Do not reopen them without a material new constraint:

1. **Initial encodings:** UTF-8, BOM-marked UTF-16, GB18030, and Big5. Recognize encodings where reliable and permit explicit selection when uncertain; no promise of perfect automatic detection or arbitrary encoding support.
2. **Oversized/no-heading text:** generate bounded reading sections, preferring paragraph boundaries. Preserve ordinary detected chapters; split oversized ones into clearly labeled parts, and offer generated sections in review when headings are absent. Disclose generated divisions in the preview. These are index ranges, not rewritten/copied content. The implementation must also bound unusually long paragraphs using valid character boundaries; exact thresholds are engineering parameters to check with representative inputs.
3. **Inbox completion:** users finish copying before opening Imports or pressing Scan. Opening Imports continues to trigger scanning. Ignore temporary suffixes; recommend temporary-name then rename for automated producers, but do not require that protocol for ordinary manual use. In-progress writes are outside the supported scanning contract; inexpensive change checks are useful but are not proof of completion. No watcher or elaborate completion-detection system.
4. **First reparse behavior:** preserve demonstrably reliable mappings, flag uncertainty, let users select a new resume section, and retain unresolved bookmarks visibly. Do not add precise text-anchor tracking or an approximate relocation engine merely for this feature. Initial-import reinterpretation has no saved locations to migrate.
5. **Delivery order:** first deliver browser/inbox batch acquisition, bounded asynchronous processing, automatic/preset analysis, encoding selection, basic preview/bulk acceptance, unified library/prose reading, progress/bookmarks, complete removal, and backup/restore integration. Follow with advanced custom patterns, published-book reparse, optional auto-add, and richer bulk-review controls. EPUB remains later. This sequencing does not authorize implementing the still-proposed architecture or dropping integrity work from the first delivery.

## Accepted Approach

The confirmed product decisions and methodical implementation are accepted. Begin with the isolated TXT interpretation step below; consequential shared-state and durable-lifecycle design remains an explicit checkpoint:

1. Use the alternatives already compared below; revisit them only if the bounded implementation design exposes a material new constraint.
2. Prefer reuse and a complete, bounded TXT path over an abstract foundation rewrite performed solely in anticipation of future providers.
3. Explain consequential tradeoffs and obtain agreement before committing to interfaces, schemas, or lifecycle mechanisms.
4. Deliver small working steps with focused verification. Resolve the shared-state and lifecycle boundaries before their implementation; do not mistake progress on an isolated step for acceptance of the remaining schema or runtime design.

Do not inherit the previous proposal's table split, shared section persistence, provider/publication ID scheme, revision counters, original-byte indexing, lock topology, directory hierarchy, deletion quarantine, or foundation-first sequence as requirements.

## Refined Design Recommendation — Proposed

This section records the systematic design review and subsequent discussion. Implementation authority and concrete progress are tracked in Delivery Steps and Current State, not inferred from this proposal. Product behavior and core/advanced scope are confirmed above; unimplemented signatures, table layout, and coordination mechanisms still require the relevant checkpoint.

### Ownership and appropriate abstraction

| Concern | Recommended owner and shape | Avoid |
|---|---|---|
| Library identity, display metadata, listing/organization | One shared library model/store; server-side bounded queries. User-edited metadata has one authoritative home; provider-returned metadata is acquisition data, not a competing display record. | Fetching every provider's books and merging/paginating in memory; per-card provider queries; growing nullable format-specific columns. |
| Ordinary reading and reader state | Small reading operations coordinating authorization, section lookup, document opening, progress/bookmarks, and normalized failures. Reuse the existing prose semantics and renderer. | Separate TXT/EPUB readers or a pass-through service with no invariant of its own. |
| Native catalog/content | BookSource owns source bindings/catalog/cache; TXT owns encoding and chapter index. Share the section/document contract; start with provider-owned catalog persistence unless common storage earns its cost. | Assuming a shared interface requires shared section rows plus provider-extension rows. |
| Provider variation | Consumer-owned interfaces only for real shared operations. Explicit selection in application composition; focused origin-specific management remains explicit. | Giant Provider interface, plugin registry, reflection, universal provider payload, scattered provider checks in the reader. |
| TXT analysis | One concrete analyzer with ordinary functions and a typed method choice: automatic, preset, custom where approved. | Class per pattern, parser plugins, generic workflow engine. |
| Durable bytes | Extend existing reader-home rooted storage only for required streaming, ownership, backup, and cleanup operations. TXT owns interpretation, not a separate file-lifecycle system. | Whole-file ReadFile for novels; cloud/virtual filesystem abstraction; encoding-aware storage infrastructure. |
| Cross-owner mutations | One explicit use-case operation owns commit/rollback, using narrow transaction-aware store operations. | Module-internal SQL from another module, separate commits stitched together by an HTTP handler, a general unit-of-work framework. |
| Background analysis | Recoverable SQLite work records plus bounded workers integrated with reader lifetime. | One goroutine per file, unbounded per-reader pools, external broker, event bus, retaining whole novels in memory. |

Dependencies should flow from application composition/reading operations toward the shared model and origin implementations. The library must not import origin modules. Shared document values must not depend on HTTP handlers or BookSource-native processor state. Provider implementations can adapt to those values without duplicating them. Physical package splits should follow these responsibilities only when they materially improve locality.

Keep BookSource catalog synchronization and TXT reparse as distinct use cases; share location validation/commit mechanics only where their contracts match. The existing BookSource switch transaction is a preservation requirement, not permission to make all origins implement source switching.

### Identity, changes, and reading locations

- Give every library item a stable identity. Title/author merge remains BookSource-specific; pending acquisitions need not be visible library items. The shared cutover's concrete row/key layout is recorded in [the exact contract below](#exact-shared-state-cutover-contract).
- Distinguish active content/catalog revision from optimistic concurrency on progress. Scrolling must not invalidate a prepared TOC solely because a progress counter changed. Ordinary progress writes must still reject stale structure and conflicting writes; retain existing ordering/conflict protections.
- Section references are opaque outside the owning origin and qualified by the relevant revision. Do not assume ordinals survive replacement or that random IDs create correspondence between different catalogs.
- The current prose location is section/chapter plus normalized progression. A scroll fraction is NOT an original-byte location, especially after presentation transforms. Do not invent exact TXT relocation by multiplying that fraction by a byte range. Follow the confirmed conservative reparse policy rather than introduce exact anchors or a speculative universal audio/image location model.
- When applying a new interpretation, read current progress/bookmarks and validate them inside the coordinated operation. If concurrent changes invalidate an impact preview, recompute or return a conflict rather than overwrite newer state. Expensive analysis stays outside the transaction.
- At the shared reading seam, requests/responses and client caches must identify the relevant interpretation. Old document/resource responses and queued progress writes must not attach to a newer TOC, removed item, replaced reader home, or another account. Backend lookup must likewise avoid pairing a section from one revision with content from another.

### TXT preparation and switching

- Receive durably, analyze asynchronously, accept, then read. Ordinary opening loads the prepared index and selected content; it never triggers full parsing or full-file hashing.
- Prefer one immutable original file and byte-range indexing for explicitly supported seek-decodable encodings. Do not promise every encoding or create a routine normalized full-content duplicate. A bounded sample proposes encoding; the complete analysis detects later decode errors. Resolved encoding/rule and parser version are stored with the result.
- Analyzer input is a readable original and validated interpretation options; output is a bounded index, resolved interpretation, review disposition, and diagnostics. It does not publish books or manage transactions. One streaming pass can evaluate a small bounded set of built-in patterns; enforce limits on lines, headings, candidates, and total work. Preserve leading text and handle EOF/decoding failures explicitly.
- Separate **advisory diagnostics**, **review-required ambiguity**, and **technical errors**. Ready is not defined as “no messages.” Explicit user choice can resolve ambiguity but cannot bypass invalid decoding, unsafe ranges, or resource limits. Custom patterns use the existing language's safe regex facilities and input limits, not an invented timeout subsystem.
- Prefer one active index and at most one durable replacement candidate per publication. Transient bounded alternative evidence may support Automatic review; it need not become permanent version history. A newer analysis request supersedes older work; cancellation/supersession is checked when saving a result, not only during parsing.
- Initial import and later reparse use the same analyzer and preview behavior. Acceptance is an idempotent database operation bound to the candidate and content revision, not another parse or file copy. Repeated clicks/retries must not publish duplicate library items. Independently re-uploading the same bytes is a different product question, not automatic deduplication.
- Normal reads should remain bounded by the requested unit. Use the confirmed generated-section policy for oversized/no-heading text; neither a whole-book fallback nor an unbounded HTML/JSON response satisfies this requirement.

### Durable content and operation lifetime

- Proposed file layout adds `files/inbox/`, `files/work/`, and `files/publications/<readable-name>--<unique-id>/<readable-original-name>`. No pending/ready/failed directory hierarchy or required format subfolders. Workflow state is in SQLite; pending and published imports share a stable managed file. Use portable sanitized names and stored relative paths, not names reconstructed from editable titles.
- Managed originals are immutable through the application; external in-place edits/renames are initially unsupported. Detect encountered missing/changed content explicitly; do not silently rebuild an index or hash the entire file on every open. File replacement is not implied by reparsing and needs its own future policy.
- Acquisition is not merely a rename followed by an INSERT. Record recoverable intent before destructive inbox consumption, retain evidence of managed ownership, and acknowledge durable acquisition only when the storage contract is met. Same-filesystem and cross-filesystem steps may differ. Failed database commits can be ambiguous; recovery must not delete potentially referenced bytes.
- Follow the confirmed completed-copy inbox contract; do not try to support arbitrary concurrent external writers through a complex protocol. An open external writer can survive rename, so heuristic stability is not proof of completion. Failed claims must not silently consume a different file; if managed acquisition succeeds but inbox cleanup fails, retain retryable identity/state and report partial completion without claiming/publishing it again.
- Deletion uses an identifiable, retryable lifecycle: stop new work, drain/cancel conflicting work, remove owned bytes, finalize related rows. Do not require a quarantine directory without a concrete need. Unreferenced files from interrupted acquisition are recovery inputs, not automatically disposable garbage.
- Workers must respect process-wide resource bounds as well as reader/account fairness. They hold a valid reader-home lifetime while running, not an HTTP request context; shutdown, account removal, and restore stop admission and cancel/drain work before closing/replacing storage. Do not pin a runtime for an entire idle batch or let stale workers write into a restored home. This is lifecycle integration, not a new distributed scheduler.
- Backup uses an explicit durable-file policy, not a walk that accidentally includes inbox/work. Pair the database snapshot with its referenced durable content and prevent deletion/replacement of those bytes until copied; include existing managed assets in the same consistency analysis. Parsing and ordinary reads should not require a long exclusive backup lock.
- Backup/restore must account for partially acquired or deleting publications. Choose whether to settle such operations or export a safe recoverable representation before snapshot; do not export database references whose only required bytes live in excluded work/inbox. A portable restore must NEVER replay deletion of an external inbox original from the source installation. Keep operational recovery references distinct from portable publication state.

### Extension and failure checks

| Scenario | Expected locality / invariant |
|---|---|
| New TXT heading family | TXT analyzer and focused fixtures; shared reader unchanged. |
| New BookSource login/recovery feature | BookSource implementation and its management UI; TXT unaffected. |
| Collections or common display changes | Shared library and UI; no format parser edits. |
| EPUB addition | Reuse durable file lifecycle, library, prose session, and authorized resources; implement package/spine interpretation inside EPUB. No placeholder adapter now. |
| EPUB-specific footnotes/richer navigation | Extend document semantics only for real behavior; do not force EPUB navigation hierarchy to equal a flat spine or expose archive paths/raw executable HTML. Exact EPUB design is deferred. |
| Reparse while reading or double-clicking Apply | No lost newer progress, stale candidate publication, duplicate admission, or old document attached to new sections. |
| Interrupted claim/deletion or backup during acquisition | No lost sole original, silently incomplete cleanup, or unusable portable snapshot. |
| Restart/account restore during batch | Work recovers without request lifetime dependence or stale writes to a replaced home; no destructive external inbox replay. |

### Alternatives and delivery discipline

- **One nullable universal book row:** superficially smallest patch, but leaves shared reading dependent on origin-specific state; not recommended.
- **Universal shared catalog schema/provider framework first:** centralizes tables but front-loads joins, coordinated ownership, and hypothetical capabilities; not justified by present evidence.
- **Small shared library/reading contract with origin-owned preparation:** recommended balance. Its real cost is explicit transaction composition and section validation without assumed shared-table foreign keys. Verify those seams rather than hiding the cost.

Implement a narrow complete TXT path and extract the minimum common state needed for both origins in that same workstream. “Vertical slice first” is not permission to create fake BookSources, duplicate reader state temporarily, or defer safe removal/backup. Establish identity, revision checks, and commit ownership before schema edits; avoid a standalone broad foundation rewrite. Follow the confirmed core-before-advanced delivery order when defining implementation steps. Keep it schema-coherent with existing exact-schema validation; any foreign-key cleanup design must enable enforcement on every reader connection rather than assume the current configuration does so.

## Remaining Engineering Design

These are bounded implementation-design tasks, not another product questionnaire. Choose routine reversible defaults from existing conventions; ask only when evidence requires a consequential change to the confirmed behavior, scope, data contract, or architecture:

1. **Implementation slices:** how can the confirmed core delivery be divided into complete working steps without temporary duplicate reader state or a broad framework rewrite?
2. **Shared contract and ownership:** what must the library/reader know, what stays origin-owned, and does shared section persistence simplify real workflows or merely duplicate provider indexes?
3. **Identity and reading state:** how do section references, content changes, source switches, progress, and bookmarks remain coherent without conflating every progress write with a content revision?
4. **Encoding and bounded access:** verify original-byte boundary handling for the confirmed encoding set and choose practical generated-section limits. These are focused implementation checks, not a request for an encoding framework or exhaustive benchmark suite.
5. **TOC readiness:** define concrete structural checks separating ready, review-required, and failed analysis for the confirmed methods. Ready enables bulk selection in the core delivery; it does not imply automatic admission, which is deferred.
6. **Ownership transfer:** under the confirmed completed-copy convention, what minimal recovery records handle interrupted same/cross-filesystem claims, cleanup failure, and duplicate discovery without losing originals?
7. **Durable consistency:** what is the smallest coordination/recovery mechanism that makes backup, ingestion, removal, and reading safe together? File placement before database insertion alone does not establish crash-safe ownership recovery.
8. **Limits and compatibility:** what upload/section/batch limits and representative performance checks are appropriate, and what coordinated schema/frontend transition is actually necessary?

## Non-Goals

- EPUB implementation in the initial TXT delivery, or new audio/image/fixed-layout support.
- Runtime third-party provider/parser plugins, arbitrary import scripts, or a universal interface for search, import, recovery, and reading.
- Automatic title/author merging across origins or a new cross-provider edition-linking system.
- Content-addressed deduplication, cloud storage drivers, distributed coordination, or filesystem watchers without a demonstrated requirement.
- A full manual TOC editor, parser marketplace, or full-text editing suite.
- Unrelated BookSource compatibility fixes, repository-wide refactoring, or expanded test/tooling infrastructure.

## Shared-State and File-Lifecycle Checkpoint

**Confirmed operational choices:** file-changing operations may wait during local snapshot copying; ordinary reading and progress remain available. After interrupted acquisition, uncertain inbox cleanup requires confirmation. Confirmation removes the reviewed leftover inbox original, never the managed publication; if the input changes after review, the old confirmation does not authorize removal. Normal uninterrupted claims still consume their input automatically. Keep the acquisition record and expose cleanup status rather than silently re-importing or forgetting the leftover. Restore must not replay external inbox deletion.

**Checkpoint evidence before this substep:** `readerstore.SnapshotHome` snapshotted SQLite before walking files with no composite-mutation coordination. `fontstore.Add/Delete/Cleanup` already coordinate font metadata and retirement through SQLite, but not against that snapshot. `readerRuntimeManager.quiesce` waits for references before closing; future workers must be cancelled before waiting on references they hold. `book.Store.UpdateProgress` and `SaveCatalog` then shared one counter, and `SwitchSource` updates catalog and bookmarks in one transaction. Preserve that atomicity while separating content revision from reading-state writes.

**Completed implementation: snapshot/file-mutation boundary (High-Risk storage coordination, no schema change).**
- Put one cancellable gate on the existing reader-home entry, shared by every lease/FileStore for that home. Acquire it before a composite file mutation's SQLite transaction, release after commit/cleanup. Snapshot acquires the same gate before `VACUUM INTO` and holds it through local file copying/validation; archive streaming remains outside the gate. Do not hold the manager-wide mutex while waiting.
- Use the gate in existing font add/delete/cleanup. Ungated private cleanup avoids nested acquisition. Ordinary reads and progress do not acquire it; future analysis and upload streaming stay outside it, with only durable finalization entering it.
- Check cancellation while waiting and during file copying so a cancelled backup releases the gate promptly between I/O operations. This does not promise interrupting a stuck filesystem syscall.
- Verify same-reader exclusion, other-reader independence, progress/read availability, cancellation, and existing font/backup behavior with focused tests. Rollback is a code revert: this step changes neither schemas nor persisted formats. Allowlist/reference validation and acquisition recovery are still separate unfinished work; the gate alone does not make future imports backup-safe.

**Shared-state cutover boundary (accepted after final verification):** put library identity, origin discriminator, display metadata, current prose location, catalog summary, content revision, reading-state version and timestamps in the authoritative library record. Common bookmarks belong with that state. Keep BookSource logical-identity merging, bindings, catalogs and cache origin-owned; TXT interpretation/index data remains TXT-owned. A published provider record uses its library item's ID; pending acquisition identity exists before shelf admission. Section references are qualified by content revision, without a shared section table or invented correspondence across catalogs.

Keep content revision separate from the state version protecting progress/bookmark mutations and apply previews. Cross-owner use cases own one SQLite transaction and call transaction-aware store operations, not another module's SQL. The HTTP/frontend change must replace mandatory source identity in common reading requests with the relevant revision/state checks; source-specific management remains explicit. Do not maintain parallel shared/BookSource progress records, fake BookSource fields for TXT, or a dual-read compatibility layer. The exact-schema cutover uses the confirmed disposable-home policy and must include recreation/rollback instructions before changing the epoch.

### Exact shared-state cutover contract

This is the implemented shared-state cutover contract. Durable TXT acquisition/publication remains a separate pending increment.

- **Schema epoch:** advance the complete reader schema from **9 to 10**, leaving the credentials epoch unchanged. `library.ReaderSchema()` contributes `library_items` and `bookmarks`; `book.ReaderSchema()` contributes only BookSource-owned tables. Compose those contributions once in the existing application/test schema lists. No migration, compatibility view/trigger, dual reads, or temporary duplicate shared columns.
- **Authoritative library row:** `library_items.id` is the existing stable book ID (TEXT primary key); `provider` is the origin discriminator (`booksource` for this cutover). Common display columns are `name`, `author`, `cover_url`, `intro`, `kind`, `last_chapter`, `update_time`, and `word_count`. Common reading/summary columns are `dur_chapter_index`, `dur_chapter_pos`, `total_chapter_num`, `current_chapter_title`, `content_revision`, and `state_version`; timestamps remain `created_at` and `updated_at`. Keep existing public camelCase field names and add `provider`/`contentRevision`. Admission starts at content revision 0 and reading-state version 0; no TOC is required for BookSource admission. The shared admission operation never merges title/author.
- **BookSource persistence:** keep `books` as the actual provider record keyed by the library ID, referencing `library_items(id) ON DELETE CASCADE`. It owns `identity_name`/`identity_author` (the existing normalized unique merge index), `source_id`, `source_url`, `book_url`, `toc_url`, `origin` (the existing source-name label, NOT the new provider discriminator), `variable_map`, and `alternate_sources`. Remove all shared display/progress/summary/timestamp columns from this table. `chapters` stays provider-owned and references `books(id) ON DELETE CASCADE`; source/URL-bound `chapter_cache` stays BookSource-owned. BookSource acquisition/reading-context DTOs can still combine these records in their current shape, but they are projections, not duplicate storage. Native source bindings and shared metadata must not be inferred from each other's presence.
- **Bookmarks:** move the existing `bookmarks` ownership to library, add `content_revision`, and declare `book_id REFERENCES library_items(id) ON DELETE CASCADE`. Keep numeric `chapter_index`, snapshot `chapter_title`, normalized `position`, `note`, `orphaned`, and `created_at`; no shared section rows or invented anchors. Successful add/delete mutations advance `state_version` in the same transaction. An exact repeated add ID/payload is not a second mutation; a conflicting reuse of an ID remains a conflict. Mapping a bookmark in a replacement transaction updates its revision; unresolved marks retain their old revision and remain visibly orphaned.
- **Connection enforcement:** put SQLite `foreign_keys(ON)` in the reader connection DSN so every pooled connection enforces declared relationships; use enforcement in applicable schema initialization and synthetic fixture connections too. Fix fixtures that insert child rows without their real parent rather than disabling enforcement. Schema validation continues to compare composed authoritative DDL exactly.
- **Revision rules:** catalog publication/replacement and source promotion increment `content_revision` atomically with native catalog state and shared catalog summary. Progress writes never change content revision. Catalog crawls capture active source identity plus content revision, perform network work outside SQLite transactions, and compare that captured interpretation on publication; changes to `state_version` alone do not reject a crawl. Progress and bookmark mutations compare both `content_revision` and `state_version`. Source switch/apply compares both captured versions because it relocates reading state; it retains existing title/nearest-index progress mapping and title/orphan bookmark handling in one transaction, and advances reading-state version with that relocation.
- **Module seam:** library imports neither `book` nor `txt`. Library owns numeric location checks, shared CAS, metadata, bookmarks and shared-row removal. BookSource owns readable-chapter/title lookup and catalog interpretation; request/use-case code validates a real readable chapter before shared writes, with the final revision CAS rejecting a concurrent catalog change. Cross-owner transactions call narrow transaction-aware library operations, never another module's internal SQL. Source switching still deliberately regenerates chapter IDs. No long transaction wraps network work.
- **Library read boundary (confirmed):** `/books` and `/books/:id` return provider-neutral library metadata/state and optional display enrichment, without native bindings. `/books/:id/booksource` returns the existing combined BookSource-context projection for source-specific consumers. Keep source metadata separate rather than adding a nested provider payload to every common detail response. Shared list reads and native cover/source-label enrichment use a bounded number of queries; native data is not a second authority for shared fields.
- **Common HTTP writes:** retain `/books/:id/progress` and bookmark paths. Progress/add-bookmark payloads replace mandatory `sourceId` with required nonnegative `contentRevision`, retaining required nonnegative `stateVersion` and current location fields. Bookmark deletion also supplies both versions. Mutation responses expose the resulting `stateVersion`; add keeps the bookmark's existing public fields (plus content revision) and supplies the resulting state version, delete retains `status` plus that version. Missing/malformed guards are 400, missing records are 404, and stale interpretation/state is 409 `state_changed`. Generic library list/detail data comes from library-owned rows and requires no source fields; source-specific management DTOs/endpoints remain explicit.
- **Catalog/document lifetime:** a ready catalog response carries its `contentRevision` alongside chapters; content requests carry that revision and content responses repeat it alongside the existing version-1 prose document. Read native section and owning interpretation coherently before network work. Before cache admission, compare the captured revision and current source/section URL inside the admission transaction; source ID alone is insufficient when switching away and back. Opaque image resource URLs also carry interpretation revision so old documents cannot resolve new images. Do not hold a transaction during the crawl. Clients key section reuse/conversion/prefetch by book plus content revision and reject superseded results, while preserving existing account/home lifetime and cancellation guards.
- **Frontend state ordering:** progress and bookmark mutations share each book's existing serialized write ordering/version ownership, so a successful bookmark mutation cannot leave the next queued progress write on the previous version. A content change invalidates old queued locations rather than rebasing them onto the new revision. Load the new catalog revision before opening content; do not pair an initially loaded revision-0 book with a newly published revision-1 catalog. Keep the existing prose renderer and source recovery workflow; no visual redesign.
- **Bounded shelf reads:** generic listing uses a fixed number of queries, not per-item provider materialization. Fetch any BookSource-specific summary augmentation in one batched lookup for that result set; this does not add pagination or silently cap the existing shelf response. Preserve the existing batched source-revision cover enrichment and avoid per-card source/catalog reads. Current chapter title is the shared display summary maintained by accepted progress/catalog mutations, not a full-catalog shelf lookup.

**Compatibility, recreation, rollback:** backend/frontend must deploy together. Epoch-9 reader homes **and epoch-9 Reader Data backups cannot be opened/restored by epoch 10**; backups are not a migration path. Follow the canonical [development reset and complete cold-copy runbook](../runbooks/development-data-reset.md), preserving the complete stopped `DATA_DIR` before choosing fresh disposable data. Do not delete isolated reader homes: account authority belongs to the same deployment. Never delete or recreate an actual developer/user home automatically. Rollback requires the matching old backend/frontend plus the retained complete epoch-9 `DATA_DIR`; do not open an epoch-10 root with old code. Existing backup/restore exact-schema checks must enforce this incompatibility. No data-root operation is authorized or performed by this implementation task.

**Completed preparatory increment:** `replaceChaptersTx` no longer commits its caller's transaction. `SaveCatalog`, `replaceChapters`, and admission own commit/rollback; admission reuses the catalog helper rather than duplicate its writing loop. Source-switch ID/mapping behavior is unchanged. Real SQLite tests verify catalog/count changes plus additional caller-owned state commit or roll back together, and invalid admission leaves neither a shelf row nor partial chapters. This removes a concrete composition obstacle; it is not the shared-schema cutover itself.

## Accepted TXT Lifecycle

The user accepted the simplified lifecycle and clarified that “one managed copy” does not mean retaining an inbox duplicate. Managed originals are immutable outside the application; edited/replaced files are re-imported. Browser originals stay on the user's computer; cross-filesystem transfers may briefly need two server-side copies, with inbox consumption only after durable finalization.

- One TXT-owned file record survives acquisition, review, publication and removal; ordered section indexes belong to TXT. Link to a library item only on acceptance. Do not move or duplicate bytes when accepting a reviewed file.
- Persist acquisition intent before moving an inbox original. Stream uploads/cross-filesystem copies into disposable work outside the snapshot payload; finalize under the existing home mutation gate. An interrupted complete managed file remains recoverable; incomplete transfers are explicit retries, not readable publications.
- Keep a removal record until owned-file cleanup succeeds. Failed cleanup remains retryable. Reuse the confirmed conservative inbox cleanup policy; restore never replays external inbox deletion.
- Add only the TXT-specific reference validation/portable-record preparation needed at the existing backup boundary, not another backup or job framework.

**Completed backend interpretation/publication increment:** TXT analysis and original-byte section indexes are persisted; qualify pending previews with a version so changed options cannot race acceptance. Analysis runs outside the file gate; its result commits only if the claimed pending version still owns the receipt. Acceptance inserts shared library metadata and links the existing TXT record atomically, without moving bytes or merging names/authors. Published analysis is immutable in this increment. Section reads select one stored range within a consistent metadata snapshot, use the shared library interface for revision checks, and do not load the full index or reparse. Publication removal atomically hides the library item and retains its cleanup record until file/index retirement finishes. This was first verified as an isolated component; production composition is now described below. Public intake and reading routes remain pending.

**Completed backend component increment:** managed receipt persistence and streaming upload acquisition/recovery/discard in `backend/internal/txtstore`, with synthetic tests. Keep pending originals at a stable `files/txt/<readable-name>--<id>/original.txt`; reserve `files/.work/txt/` for disposable transfer work and exclude that reserved work root from snapshots/restores. The original receipt increment exposed no intake routes and was tested before production registration. Interpretation/publication, portable validation, inbox claims and worker integration have since been added; public intake remains pending.

## Accepted TXT Worker Ownership

Use a small in-process TXT worker pool independent of API runtimes, not a worker pinned to every reader runtime. Start with two active files globally, at most one per reader, and rotate readers between files. Queue durable work in TXT records; keep only deduplicated reader wake-ups in memory. Waiting readers hold no home lease, decoded text, interpretation index, or per-file goroutine. Release each active home lease and result before taking the next file; remove idle scheduling entries. No external queue, generic job framework, or retained job history.

Production composition budgets 32 API-runtime homes plus two active worker homes. Moving goroutines alone would leave contention at the storage pool. Preserve explicit bounds rather than claim 100 simultaneous heavy operations. Validate foreground access during saturated import work before claiming the deployment target is met.

Restore/deletion must stop new reader requests and TXT admission, cancel/drain that reader's work, then replace/remove its home. Other readers continue. Shutdown joins workers before closing reader storage. Recovery runs only with intake quiescent, never before each analysis job: doing so could mistake a concurrent live upload for an interrupted transfer. Restored/reopened data must be queried afresh rather than replaying old file jobs or cleanup authority.

The analysis pool and its quiesce/resume/forget/close interface were first verified as an unregistered component. Production composition now follows the integration contract below; upload/inbox scheduling and HTTP intake remain pending. Verify claimed-version cancellation/recovery, fairness/global and per-reader bounds, lost-wake avoidance, idle-state cleanup, and release of home leases on errors/cancellation/shutdown. Do not turn this into an exhaustive scheduling framework or a performance guarantee.

### Accepted production lifecycle integration

This High-Risk increment composes the existing TXT schema and worker pool; it does not expose intake or provider reading routes.

- Advance reader epoch **10 → 11** when registering `txtstore.ReaderSchema()` in production. Credentials and archive format stay unchanged. Epoch-10 homes/archives are incompatible; preserve a complete stopped `DATA_DIR` and use the [existing reset/cold-copy runbook](../runbooks/development-data-reset.md). Rollback requires the prior application and its matching complete epoch-10 data, not editing markers or restoring old archives into epoch 11. No actual user/developer data is modified by implementation.
- Recover retained, non-deleting account homes before serving requests or waking workers. New accounts start empty. Recover again only after successful restore publication, inside the reader's quiescent window; ordinary runtime opens/jobs never recover live intake. Per-home recovery problems are logged without blocking unrelated readers.
- **Confirmed restore outcome:** successful replacement remains `restored: true` if TXT reconciliation fails; return a machine-readable warning and display it persistently in the backup UI. Keep failed work available for retry, log diagnostic details server-side, and do not misreport an already-published replacement as a failed restore. Invalid/incompatible backups still fail before publication. No staged TXT-recovery framework is needed.
- Compose runtime and worker drain/resume in the API owner. Deletion forgets worker barriers only after successful home removal. Shutdown joins workers even if another service's cleanup fails.
- Budget the existing 32 API runtimes plus two active worker homes in production. Capacity waits remain cancellable, not arbitrarily abandoned after ten seconds. Later upload intake needs its own bound and must not borrow API runtime slots; that admission/routing change is not exposed here.
- Verify recovery before admission, post-publication warnings and resume ordering, deletion cleanup/retry, shutdown, exact-epoch composition and affected capacity/race regressions. Existing portable validation remains authoritative; no live fixtures or deployment claims.

## Accepted TXT Reading and Removal Integration

Implement the complete reading path through the existing HTTP routes and prose reader, before intake exposure. A small application-level reading module selects the concrete BookSource/TXT implementation, normalizes catalogs/documents, and validates revision-qualified locations before library-owned progress/bookmark writes. Catalog responses contain reader-facing indices/titles/heading flags, not native URLs, paths, or fabricated BookSource identities. Keep source recovery, synchronization, and image fetching BookSource-owned; no plugin registry or universal provider interface.

TXT library removal uses its existing hide-first, recoverable byte-cleanup lifecycle. Report removal with a visible **cleanup pending** warning when hiding succeeded but cleanup failed; retain the record and allow an idempotent retry. Do not claim complete deletion or silently discard an error. Pending unpublished receipts are not removable through a library-book operation.

Scope: backend reading/removal routing, shared contracts, and the existing frontend consumers. No intake UI, reparse, schema change, migration layer, or real data-root modification. Verify a synthetic TXT HTTP journey, invalid/stale locations and cleanup retry, existing BookSource reading regressions, and focused frontend integration. This is a public-interface/destructive-operation increment: keep backend/frontend changes together; rollback is the preceding code revision with the unchanged epoch-11 home schema (deleted test/application data is not recreated by rollback).

## Accepted TXT Intake Admission

Use a small, fair, process-owned admission queue before transferring file bytes. This replaces the
initial client-retry recommendation: server-owned ordering avoids retry competition as concurrent
import use grows, while clients retain only file references/metadata and one pending transfer.
Registered-account count is not a throughput guarantee; increase measured host capacity rather
than promising short waits under arbitrary demand.

- Keep one ticket per reader and rotate at file boundaries: a new file rejoins the tail. Start with
  two granted/active transfers globally and a bounded 1,024-ticket map. Waiting/granted tickets hold
  no file contents, home leases, or per-file goroutines. Ordinary reading and analysis retain their
  separate capacity budgets.
- Admission is metadata-only. Waiting clients refresh their ticket; inactive waiting tickets expire
  after two minutes and unused grants after 30 seconds. One-use grants become cancellable active
  transfers with a 30-minute ceiling. Expire/promote on queue access or completion, not with a new
  scheduler thread. These are reversible engineering defaults, not deployment performance claims.
- Tickets belong to the authenticated reader and process lifetime. They are not durable acquisition
  receipts or inbox-cleanup approvals. Restart/restore invalidates admission; no bytes may be consumed
  until a grant is claimed. Acquired files continue through the existing durable analysis workers.
- Integrate admission cancellation/draining with restore, deletion, and shutdown. Release an active
  ticket only after its home lease and I/O have ended. HTTP transfer adapters must interrupt blocked
  body reads when the admission context is cancelled, not merely check cancellation between reads.
- First checkpoint: the admission owner, separate home-capacity allowance, lifecycle integration and
  focused queue/lifecycle tests. No new schema, migration, intake HTTP route, or file consumption in
  this checkpoint. Next wire acquisition/review APIs together, including server-retained inbox proofs,
  before exposing intake. Rollback is a code revert with the same epoch-11 reader homes.

## Delivery Steps

1. **TXT interpretation boundary (complete; Standard change).** Implement a concrete analyzer in `backend/internal/txt`, using existing `x/text` support, and direct bounded section decoding. Done means synthetic originals in the confirmed encodings produce lossless original-byte ranges, automatic/preset heading recognition and bounded generated sections; ambiguity/errors are explicit and cancellation is respected. No storage writes, worker framework, schema, HTTP, or frontend changes. This validates the original-byte strategy before wiring persistence. Engineering defaults are local resource bounds, not a finalized upload policy or performance guarantee.
2. **Shared-state and durable-lifecycle checkpoint (complete).** Shared-library ownership, snapshot coordination, TXT acquisition/interpretation/publication/removal, portable validation, and production worker/recovery lifecycle are implemented and verified. Continue through the concrete HTTP/user workflow rather than adding another foundation layer.
3. **Durable acquisition through reading.** Integrate browser/inbox intake, bounded asynchronous work, persisted candidates, acceptance, shared reader/progress/bookmarks, and complete cleanup/backup in working increments under the accepted checkpoint. Define concrete increments there, not speculative tables now.
4. **Core user workflow and verification.** Connect basic preview, encoding/preset corrections and bulk acceptance to the unified library. Verify representative batch responsiveness and bounded opening plus focused BookSource regressions. The core is not complete until removal and portable restore work.
5. **Advanced follow-up.** Only after core completion: the already deferred controls and reparse, then separately scoped EPUB work.

## Current State

- Branch: `feat/multi-provider-library`, created from `main` at `0702469`.
- TXT interpretation is implemented in `backend/internal/txt` and connected to common reading routes through the persisted publication index. Intake routes remain pending.
- Per-home snapshot/file-mutation coordination and caller-owned BookSource catalog transactions are complete.
- The epoch-10 shared-library cutover is implemented and verified across storage, runtime, HTTP and frontend. `library` owns common metadata/progress/bookmarks; BookSource owns bindings/catalog/cache. Catalog/content and reading-state revisions are separate. Source switching retains atomic mapping and BookSource identity merging is preserved.
- Common list/detail responses contain library fields and display enrichment; `/books/:id/booksource` supplies native context separately. Shelf enrichment is batched. Content/caches/images and client loading are revision-qualified; progress and bookmarks share one queue. Orphan deletion guards current state while retaining old location identity.
- `backend/internal/txtstore` now implements managed upload receipts: persisted intent, bounded streaming into disposable work, gated finalization, quiescent recovery and retryable discard. Synthetic reader-home tests cover portable pending originals and reader isolation. It is registered in production at epoch 11, with startup/restore recovery. Upload/intake HTTP routes remain unexposed.
- The isolated TXT component now persists requested/resolved interpretation settings, review reasons, parser version and indexed original-byte ranges. Pending analysis versions guard acceptance; admission is atomic and idempotent, equal names/authors remain independent publications, and published re-analysis is rejected. Shared progress/bookmarks and indexed reads work without BookSource tables; removal hides shared state before retryable physical cleanup.
- Feature-owned portable-reference validation is implemented and tested against copied reader homes: required managed originals must exist as regular files with matching sizes, published receipts and TXT library items must correspond in both directions, and incomplete acquisition/removal may lack an original. Failed/removing receipts can retain damaged bytes for explicit cleanup. Validation neither reparses nor sweeps files; size checks rely on the accepted immutable-managed-file policy, not content hashing.
- Single-file inbox acquisition is implemented as a backend component: same-filesystem rename, cross-device streaming fallback, atomic receipt/journal intent, guarded consumption and unresolved-name deduplication. Recovery never deletes inbox leftovers; export/import strips the operational journal from copies only. Inboxes are outside replaceable reader homes and remain unclaimed external input on home removal.
- Backend leftover review/confirmation and explicit release are tested. Review proofs are scoped to the current database lifetime; confirmation checks identity/content and a matching surviving managed copy. Release preserves files and permits normal later acquisition with a fresh receipt; it neither silently retries nor discards an existing receipt.
- Reader runtime initialization now has one owner per reader, including during quiesce/shutdown and capacity accounting. Targeted concurrency regressions and the API package pass.
- The independent `backend/internal/txtimport` pool and persisted analysis scheduling are production-wired. Analysis claims load original metadata/options atomically, complete their short claim independently of cancellation, and return interrupted attempts to `received` without overwriting a later version. Indexed selection takes one pending file; active homes/results are released between fair reader turns. Idle scheduling entries and completed job state are not retained.
- Epoch-11 TXT schema registration, pre-serving recovery of retained account homes (including disabled accounts), post-restore recovery, combined intake/runtime/worker drain/resume, deletion barrier retirement and joined shutdown are implemented. Startup per-home errors do not block unrelated homes. Permanent storage errors are logged and pending work remains durable for a later wake/restart; no automatic retry engine was added.
- Restore returns `restored: true` with a safe warning if TXT reconciliation fails after publication. The frontend clears pre-restore reader state and keeps a translated warning visible. Corrupt/incompatible archives remain rejected before replacement. The common deletion route now uses TXT's recoverable removal lifecycle; unrecognized providers remain explicitly unsupported.
- `backend/internal/reading` now owns common catalog/prose adaptation and location validation over concrete BookSource/TXT implementations. Catalog entries expose only index/title/heading flags. TXT uses one indexed original-byte read and literal paragraphs; native processing/cache/resource behavior stays BookSource-specific. Progress and bookmarks commit through the library's shared CAS operations.
- TXT removal reports `txt_cleanup_pending` after successful hiding but incomplete cleanup. Book Detail keeps a focused warning/retry result, retries by the stable publication ID, and does not silently restore a shelf row or allow unpublished acquisitions to be removed as books. Existing-reader integration and failure recovery require no source context for TXT.
- TXT reading/removal verification passed the full backend suite, affected race tests, frontend build and 36 focused frontend tests. The legacy standalone SQLite opener's ignored driver options were corrected to match the actual driver's pragma syntax after concurrent catalog reads exposed lock errors; per-connection WAL/busy-timeout coverage passes.
- `txtimport.Admission` now implements the accepted intake scheduling contract, independently of file storage, parsing and HTTP. The server owns its restore/deletion/shutdown lifecycle and budgets separate transfer homes. Queued/granted tickets hold no home leases; cancelled active transfers retain capacity until their caller releases after cleanup. Queue access/completion drives expiry and FIFO promotion, with no additional scheduler goroutine.
- Admission component/production-composition tests and targeted API lifecycle tests pass; race runs of the complete `txtimport`, `api` and `cmd/server` packages pass. AFT inspection timed out; compiler/tests and scoped diff review are the authority. No schema change, migration, existing-data reset or deployment occurred. Admission/acquisition HTTP adapters, server-retained inbox proof controls and import/review UI remain unimplemented.

## Next Action

The intake-admission component and lifecycle checkpoint are complete and verified; no intake route is exposed. Next wire metadata-only admission plus browser/inbox acquisition and review HTTP adapters, followed by the core import/review UI. Transfers must claim a reader-bound grant before opening a home **outside API runtime slots**, interrupt blocked body reads on cancellation, and release admission only after I/O/home cleanup. Reuse receipt/analysis/acceptance operations and worker wake-ups; no new job framework, readiness layer or migration system. Retain server-issued inbox review proofs (bounded lifetime/ownership) before exposing cleanup confirmation; do not reconstruct approval from client JSON. Remaining files in a browser batch stay client-owned until acquired; only an explicit pre-acquisition rejection is safe for automatic retry.

Keep the confirmed intake constraints: finish copying before Imports/Scan; rename-first with streaming cross-device fallback; one immutable managed original; no silent consumption of unresolved inbox leftovers. HTTP review/confirmation must retain the server-issued proof scoped to the current reader/database lifetime, not reconstruct authorization from displayed fields. Reuse the existing journal and operations; no generic import or backup framework.

Do not restart the shared-state cutover, repeat broad architecture review, or make advanced parsing/reparse controls prerequisites. Record consequential choices before production edits; the core scope and operational choices remain authorized.

## Verification

- Intake admission: `go test ./internal/txtimport ./cmd/server -count=1`, targeted API runtime/backup/restore/TXT-worker/shutdown tests, and `go test -race ./internal/txtimport ./internal/api ./cmd/server -count=1` pass. Five deterministic admission tests cover reader-bound single-use grants, deduplication, FIFO file turns, the 1,024-ticket bound, expiry/refresh, cancellation versus release, isolated quiescence and joined shutdown. Existing composed tests now include both occupied transfer leases and blocked analysis workers while one foreground reader stays active and 100 identities cycle through another runtime slot, plus restore invalidation and server shutdown. This verifies ownership/capacity—not throughput, memory profiles, 1,000-user load, or actual HTTP uploads. No frontend code changed or frontend tests rerun. AFT inspection timed out; `git diff --check` passes.

The entries below record verification at earlier component checkpoints, not today's exposure status.

- TXT reading/removal: full backend `go test ./...` and `go test -race ./internal/database ./internal/reading ./internal/txtstore ./internal/library ./internal/api -count=1` pass. Two synthetic HTTP journeys cover neutral catalogs, literal prose, persisted progress/bookmarks, revision and location failures, complete/idempotent removal, warning-bearing cleanup retry and protection of pending originals. Existing BookSource HTTP/content/cache/image tests pass. The superseded readable-existence helper was removed; final book/API tests and a one-iteration `publication_snapshot` benchmark smoke test pass (not a performance claim). Frontend typecheck/build and nine focused test files (**36 tests**) pass; the final focus/control/translation edits were reverified with the affected three files (**14 tests**) and build. Coverage includes reading without native source context, cleanup warnings surviving failed retries, keyboard focus on the removal result, and shared catalog consumers. No browser visual run, deployment, or end-user intake workflow is claimed. AFT inspection timed out.

- Production lifecycle: full backend `go test ./...` and `go test -race ./internal/txtimport ./internal/backup ./internal/readerstore ./internal/auth ./internal/api -count=1` pass. Tests cover recovery before analysis, complete versus partial transfers, per-home cleanup failure without blocking good work, fake-time capacity waits beyond ten seconds, retained/disabled/deleting account discovery, recovery of replaced data before resume, HTTP success with a TXT warning and retained unowned bytes, deletion barrier cleanup, and shutdown after another service fails. Production schema composition and existing adjacent-epoch/portable checks pass. Frontend typecheck/build and four focused test files (**12 tests**) pass, covering warning visibility, failed commits, cache invalidation without logout, and translation parity. AFT inspection timed out; compiler/tests and diff review are the authority. Browser visual testing, deployment, and end-user TXT intake/reading are not claimed. A late regression test reproduced unrouted provider deletion returning success; the capability gate fixes that boundary pending TXT management routing; final `go test -race ./internal/api -run 'Test(Library|Restore)' -count=1` also passes.

- Independent TXT analysis pool: `go test ./internal/txtstore ./internal/txtimport -count=1`, both packages under `-race`, and `go test -race ./internal/api -run 'Test(TXTWorkers|.*Runtime)' -count=1` pass. Deterministic tests cover reader fairness, fixed global/per-reader concurrency, duplicate and late wake-ups, interrupted claims/options, per-file failure, isolated quiesce, resumed work, deletion-barrier cleanup, idle-entry retirement and joined shutdown. The composed storage/API test holds both real TXT workers at the database boundary while one reader remains active and 100 other identities cycle through another runtime slot, then verifies analysis completion and released home leases. This is an ownership/capacity regression, not a throughput benchmark, heap-profile result, or proof of 100 simultaneous expensive operations. No production worker wiring or epoch change is claimed. AFT inspection timed out; compiler/tests are the verification authority.

- Runtime initialization prerequisite: new deterministic tests first reproduced duplicate initialization, excess in-flight capacity and premature quiesce/shutdown completion. After the fix, `go test ./internal/api -count=1` and `go test -race ./internal/api -run 'Test.*(Runtime|Backup|Restore)' -count=1` passed. Tests cover failed initialization releasing its reservation and successful-but-rejected initialization releasing its home lease. AFT inspection timed out. No TXT worker or schema registration was added.

- Leftover resolution: `go test ./internal/txtstore -count=1` and its `-race` run passed. Four focused regressions cover scoped/idempotent confirmation with preserved publication, changed content despite preserved size/mtime, discarded managed originals, and absent/reappearing inbox entries. Explicit release followed by normal acquisition is covered; no second retry engine was introduced. Windows TXT-store tests also cross-compile (not runtime-tested). AFT inspection timed out. No runtime/HTTP/UI or new schema epoch is claimed.

- Rename-first inbox component: `go test ./internal/txtstore ./internal/readerstore ./internal/backup -count=1`, the corresponding `-race` run, and API tests matching `Test.*(Backup|Restore)` passed. Synthetic tests verify actual same-filesystem inode-preserving movement, reader isolation, copy fallback/cancellation, retained claims after discard, interruption without deletion/re-import, and export/import journal stripping without changing live intent. Cross-device error classification is tested; fallback execution is tested directly, not with a privileged bind mount. Windows TXT-store tests cross-compile; no Windows runtime or power-loss verification is claimed. AFT inspection timed out. Cleanup confirmation, batch scheduling, runtime exposure and production schema registration are still pending.

- Portable-reference boundary: `go test ./internal/txtstore ./internal/readerstore ./internal/backup -count=1`, the corresponding `-race` run, and API tests matching `Test.*(Backup|Restore)` passed. Synthetic tests cover published restore/read, missing original rejection during export/staging/publication, intact live homes after rejected staging, broken ownership in both directions, invalid paths/sizes and recovery of portable unfinished receipts. AFT inspection timed out; tests and diff review supply verification. No production TXT registration, inbox consumption, semantic revalidation of stored indexes, or content-hash verification is claimed.

- Persisted TXT interpretation/publication: `go test ./internal/txtstore ./internal/library ./internal/txt -count=1` and the corresponding `-race` run passed. Nine TXT-store tests now cover receipt lifecycle plus persisted preview/restore, original reconstruction through indexed reads, stale preview/result rejection, failed/restarted analysis, independent equal-name admission, rollback on conflicting IDs, immutable published interpretations, shared progress/bookmarks and publication-removal cleanup. The receipt snapshot test now also verifies saved pending interpretation portability. No production TXT schema/routes/workers or full portable-reference validation is claimed for this component increment. AFT inspection timed out; focused tests and diff review are the verification evidence.

- Managed receipt component: `go test ./internal/txtstore ./internal/txt ./internal/readerstore ./internal/backup -count=1` and `go test -race ./internal/txtstore ./internal/readerstore ./internal/backup ./internal/fontstore -count=1` passed. Five new synthetic receipt tests cover unchanged single-original storage, pending-file snapshot/restore isolation, interrupted complete/incomplete transfer recovery, cancellation, foreign persisted path rejection, and retryable failed cleanup. Unreferenced managed files are never swept. AFT inspection timed out; production TXT intake, inbox consumption, analysis/publication integration, full-file reference validation and power-loss fault injection are not claimed.

- Full backend `go test ./...` passed after the final cutover edits.
- Race verification passed for `internal/library`, `internal/book`, `internal/api`, `internal/readerstore` and `internal/backup`. Initial new-test expectation mistakes (bookmark creation status and the existing delete route) were corrected; affected API race tests and the full backend suite then passed.
- `npm run build` and the full frontend suite passed: **203 tests in 57 files**.
- Focused regressions cover shared-state CAS and caller-owned transactions, catalog/source-switch behavior, late cache admission, in-flight catalog replacement, obsolete image links, revision-qualified requests, bookmark ordering/orphan deletion, generic library/native-context separation, pooled foreign-key enforcement and complete shared/native deletion cascades.
- Existing reader-home tests reject both preceding and succeeding epochs without mutating the home. Portable-backup coverage rejects an old archive epoch, releases the failed preparation reservation, and then restores a compatible archive across readers.
- One bounded independent review of transaction/revision ownership found no actionable correctness issues. It was read-only and did not duplicate frontend or compatibility verification.
- Authoritative ownership/revision docs and the upgrade/reset guidance were updated. AFT inspections timed out; compilation, tests and diff review—not AFT—are the verification evidence. Hosted CI, deployment, live-source compatibility and TXT import/restore integration are not claimed.

### Implemented Interpretation Boundaries

- Automatic encoding selection recognizes BOMs or a valid UTF-8 sample followed by full streaming validation. It does not guess GB18030 versus Big5; `ErrEncodingRequired` requests a manual choice. Explicit legacy decoding rejects substitutions rather than silently corrupting the index. BOM-marked UTF-16 resolves byte order for subsequent range reads.
- Automatic Chinese/English heading candidates share one decoding pass; explicit presets and generated sections use the same analyzer. Heading scanning currently uses LF/CRLF lines. Returned sections include headings/whitespace and exclude only the initial BOM. Few/no headings or competing families produce review reasons, not a publish decision; admission remains outside the analyzer.
- Original-byte offsets are TXT index data, not shared reading-location identity. `Options`, resolved encoding/preset, and parser version are available for candidate persistence; revision/hash ownership belongs to the next integration step.
- Resource bounds live in `analysis.go`: decoded section/source-range limits, preferred paragraph target, input/index bounds, and heading/sample bounds. These are initial engineering defaults, not finalized upload policy. Before persisting indexes, define how later limit/parser changes preserve readability of existing interpretations. Generated sections remain character-unit aligned and preserve original bytes; a long line is fragmented rather than buffered without limit.
- `ReadSection` needs only `io.ReaderAt`, the resolved encoding and a validated indexed range. It does not inspect a library or publication directory. Its caller must provide the matching immutable original and handle resource/revision lifetimes. Shared Reading Document conversion and acquisition/backup/delete behavior are not implemented.
