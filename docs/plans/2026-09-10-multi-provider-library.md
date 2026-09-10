---
status: requirements-review
updated: 2026-09-10
---

# Multi-Provider Library and Imported Books

## Goal

Add practical, responsive batch TXT import and reading, with a clean path to EPUB, shared library/reader behavior, understandable non-wasteful storage, portable backups, and complete removal. Preserve existing BookSource behavior without building an unnecessarily elaborate framework.

This document replaces the previous architectural blueprint with a requirements-led baseline. The user authorized this redraft, not implementation of a replacement architecture. The earlier proposal is available in Git (notably `f27c4f1`) as a second opinion, not an implementation constraint.

## Scope

The workstream covers TXT acquisition, interpretation, review, library admission, reading, backup, removal, and safe later reparsing. A complete first delivery slice and subsequent milestones remain to be agreed; listing a requirement here does not put every UI control into the first release.

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
- Preserve progress/bookmarks where a reliable mapping exists; explain uncertainty rather than silently relocate them incorrectly. Exact anchors and relocation policy remain design questions.

### Batch review and admission

- Provide timely per-file progress and actionable errors; users should not wait for the whole batch before inspecting or using completed results.
- Support bulk acceptance of suitable results instead of requiring hundreds of individual confirmations.
- Keep ambiguous and failed results from being silently auto-published.
- Allow individual review of encoding, chapter structure, sample content, and proposed title/author, with corrections before admission.
- Report partial success accurately. Failed items remain understandable and recoverable or removable.
- Pending imports must be distinguishable from ordinary readable library items and remain manageable across interruption/restart.
- Cancellation must stop further requested work safely and explain what has already completed; it is not an implicit rollback of an entire batch.

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

Possible review controls discussed previously include result filters/search/sorting, bulk method changes, beginning/middle/end TOC samples, custom heading patterns, and single-section fallback. Their exact selection, safety limits, and delivery priority are not yet approved. Automatic/preset/custom parsing are candidate approaches, not a required parser framework.

## Existing Foundations and Evidence

The earlier code inspection identified the following relevant starting points; recheck affected paths when designing changes rather than repeat a repository-wide survey:

- `backend/internal/book/store.go` combines BookSource shelf identity, source state, chapters, and progress. Logical title/author merging is specifically a BookSource behavior.
- `backend/internal/api/reader_handlers.go` and `bookmarks.go` couple ordinary reading/progress/bookmark paths to BookSource state. The frontend reader and API types reflect that contract.
- Existing Prose Documents and the prose renderer offer a reuse point; no second TXT renderer has been justified.
- `backend/internal/readerstore/home.go` and `backup.go` already own reader-home file access and portable snapshot/replacement. Database/file consistency for imported content needs explicit design, not an assumption that sequential copying is sufficient.
- [Reading documents and resources](../decisions/0002-reading-documents-and-resources.md) is relevant existing guidance. It does not establish the exact new tables, package layout, or TXT interface.

These observations identify coupling and reuse opportunities; they do not prove that a wholesale shared-storage rewrite is necessary.

## Accepted Approach

Only the requirements-led process is accepted at this checkpoint:

1. Compare the smallest credible architectures against the actual TXT and BookSource workflows.
2. Prefer reuse and a complete, bounded TXT path over an abstract foundation rewrite performed solely in anticipation of future providers.
3. Explain consequential tradeoffs and obtain agreement before committing to interfaces, schemas, or lifecycle mechanisms.
4. Add concrete delivery steps and verification gates here after the approach is accepted.

Do not inherit the previous proposal's table split, shared section persistence, provider/publication ID scheme, revision counters, original-byte indexing, lock topology, directory hierarchy, deletion quarantine, or foundation-first sequence as requirements.

## Open Design Questions

Resolve these progressively, not through one large speculative blueprint:

1. **Smallest delivery:** what complete TXT import/review/read/remove/backup path establishes the second real origin, and which advanced review/reparse controls follow later?
2. **Shared contract and ownership:** what must the library/reader know, what stays origin-owned, and does shared section persistence simplify real workflows or merely duplicate provider indexes?
3. **Identity and reading state:** how do section references, content changes, source switches, progress, and bookmarks remain coherent without conflating every progress write with a content revision?
4. **Encoding and bounded access:** which encodings are initially supported, how are safe read boundaries established, and how do long sections/no-heading files remain responsive without routine content duplication?
5. **TOC acceptance:** what evidence supports automatic admission, what requires review, and which correction methods solve the actual expected inputs?
6. **Ownership transfer:** how are incomplete writes, same/cross-filesystem claims, restart, cleanup failure, and duplicate discovery handled without losing originals? Rename does not stop an external writer with an already-open file handle; observed stability alone is not proof of completion.
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
- Requirements and prior preferences are recovered; replacement architecture and implementation sequence are not yet accepted. Production implementation remains paused.

## Next Action

Present a bounded comparison of credible architectural alternatives, grounded in the affected existing code and the requirements above. Recommend the simplest complete approach, identify consequential unresolved tradeoffs, and discuss it with the user before adding a concrete implementation plan or changing production code.

## Verification

- Branch comparison with `main` confirmed that only `PLAN.md` and this plan differ for the workstream.
- This redraft is documentation-only; no runtime tests or builds are claimed or needed to validate it.
- Before implementation, define focused checks for preserved BookSource behavior, TXT interpretation/read bounds, stale reading state, partial batch failure, interrupted acquisition/deletion, and backup/restore consistency. Use synthetic deterministic fixtures and fault injection where justified; do not multiply tests for equivalent cases.
- Performance limits and supported encodings remain unmeasured/unselected; no scalability guarantee is claimed.
