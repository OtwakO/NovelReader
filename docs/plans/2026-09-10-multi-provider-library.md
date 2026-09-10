---
status: design-review
updated: 2026-09-10
---

# Multi-Provider Library and Imported Books

## Resume Here

This is the canonical handoff for this workstream; conversation memory and older plan revisions are not needed to resume.

| Need | Read / action |
|---|---|
| What the user has decided | [Confirmed product decisions](#confirmed-product-decisions-and-delivery-order), together with [prior preferences](#previously-confirmed-preferences). Do not restart that questionnaire. |
| Architectural recommendation, not yet implementation approval | [Refined design](#refined-design-recommendation--proposed). Ownership and invariants matter more than illustrative signatures or suggested storage shapes. |
| What exists and what to inspect | [Existing foundations](#existing-foundations-and-evidence), then [current state](#current-state) and [verification](#verification). Check Git before assuming the branch is unchanged. |
| What to do next | [Next action](#next-action). Prepare one bounded implementation proposal; do not repeat the broad architecture review or start coding from a historical blueprint. |

The confirmed product choices supersede earlier alternatives in this document and Git history. Proposed architecture is not an extra feature checklist. If new code evidence conflicts with a requirement, report the specific conflict; do not silently change the requirement or implement an increasingly complex workaround.

## Goal

Add practical, responsive batch TXT import and reading, with a clean path to EPUB, shared library/reader behavior, understandable non-wasteful storage, portable backups, and complete removal. Preserve existing BookSource behavior without building an unnecessarily elaborate framework.

This document replaces the previous architectural blueprint with a requirements-led baseline. The user authorized this redraft, not implementation of a replacement architecture. The earlier proposal is available in Git (notably `f27c4f1`) as a second opinion, not an implementation constraint.

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

The confirmed product decisions above and the following requirements-led process are accepted. Concrete architecture remains proposed:

1. Use the alternatives already compared below; revisit them only if the bounded implementation design exposes a material new constraint.
2. Prefer reuse and a complete, bounded TXT path over an abstract foundation rewrite performed solely in anticipation of future providers.
3. Explain consequential tradeoffs and obtain agreement before committing to interfaces, schemas, or lifecycle mechanisms.
4. Turn the confirmed delivery order into concrete implementation steps and verification gates after the architecture is accepted.

Do not inherit the previous proposal's table split, shared section persistence, provider/publication ID scheme, revision counters, original-byte indexing, lock topology, directory hierarchy, deletion quarantine, or foundation-first sequence as requirements.

## Refined Design Recommendation — Proposed

This section records the systematic design review and subsequent discussion, not authorization to implement. Product behavior and core/advanced scope are confirmed above. Concrete signatures, table layout, coordination mechanisms, and engineering limits remain to be specified.

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

- Give every library item a stable identity. Title/author merge remains BookSource-specific; pending acquisitions need not be visible library items. Exact row/key layout is not chosen here.
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

## Current State

- Branch: `feat/multi-provider-library`, created from `main` at `0702469`.
- The workstream has changed documentation only. No production code, schema, frontend behavior, or HTTP interface was implemented; nothing in those areas required reverting.
- The previous blueprint has been replaced in this stable plan path. Git preserves its history.
- Requirements, the initial encoding set, generated-section behavior, completed-copy inbox convention, conservative reparse handling, and core-before-advanced delivery order are confirmed. The proposed refined design records ownership, appropriate patterns, lifecycle invariants, and extension checks. Concrete architecture still awaits acceptance; production implementation remains paused.

## Next Action

Prepare one bounded core-delivery implementation proposal using the confirmed choices and proposed ownership model. It should identify: (1) minimum shared state and origin-owned state, (2) section/document and progress contracts with transaction ownership, (3) acquisition/deletion/backup coordination integrated with existing runtime lifetime, and (4) small working delivery steps and targeted verification. Resolve only the relevant engineering questions above; no exhaustive schema catalog, pattern survey, or speculative framework is needed.

Present that proposal for acceptance before production edits. After acceptance, update this plan's proposed/accepted status, concrete delivery steps, Current State, and Verification together so a later session can act without asking for the same approval again. During implementation, record meaningful stopping points and verification limits here rather than create per-session or per-agent plans. Do not reopen settled product choices or turn deferred advanced features into core prerequisites.

## Verification

- Branch comparison with `main` confirmed that only `PLAN.md` and this plan differ for the workstream.
- This work is documentation-only; no runtime tests or builds are claimed. Focused source inspection rechecked `book/store.go:UpdateProgress`, `book/bookmark.go:AddBookmark`, `book/source_switch.go:SwitchSource`, frontend `reader/progress-writer.ts`, `readerstore/home.go`, `readerstore/backup.go:SnapshotHome/copyDurableFiles`, `readerstore/database.go`, and API reader runtime lifecycle. These confirm progress/source coupling, transactional source switching, whole-file file helpers, whole-tree backup copying, and the need for job/runtime lifetime integration. This is design evidence, not a completed implementation or concurrency test.
- Before implementation, define focused checks for preserved BookSource behavior, TXT interpretation/read bounds, stale reading state, partial batch failure, interrupted acquisition/deletion, and backup/restore consistency. Use synthetic deterministic fixtures and fault injection where justified; do not multiply tests for equivalent cases.
- The initial encoding set is selected but not implemented/verified. Exact performance limits remain unmeasured; no scalability guarantee is claimed.
