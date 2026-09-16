---
status: planning
updated: 2026-09-17
---

# EPUB support: structured novel reading

## Goal

Import a DRM-free reflowable EPUB, review it, add it to the existing shelf, read its text and illustrations with meaningful formatting/navigation, resume and bookmark it, and back it up or remove it through the existing reader-owned lifecycle.

This is a structural milestone, not a second reader application. Done means a reliable, explicitly bounded novel-reading profile, not universal EPUB reading-system conformance.

## Accepted direction and authorization

The user selected structured reflowable EPUB 2/3 reading and both browser and server-inbox import. They requested further scrutiny of the milestone before implementation. Planning is authorized; production implementation and the detailed proposal below are not yet approved.

Follow [reading documents/resources decision](../decisions/0002-reading-documents-and-resources.md). Reuse the library, reading session and prose modality. Do not create a plugin framework or an EPUB-specific reading stack.

## Scope and why

### Include

- Browser uploads and explicit server-inbox selection within Local Import; mixed TXT/EPUB selections retain independent outcomes.
- Package metadata, embedded cover, ordered reading sections, EPUB 3 navigation and EPUB 2 NCX navigation.
- Paragraphs, headings, emphasis, explicit line breaks, lists, quotations, illustrations/alternative text, internal links and footnotes. Include ruby annotations and simple semantic tables in the content-contract checkpoint: these carry meaning in novels and should not be silently concatenated into plain text. No CSS layout emulation.
- Existing typography, image visibility, Chinese conversion, progress, last-read tracking, bookmarks and revision-qualified navigation.
- Default-on review preference; disabling it may automatically accept only an eligible warning-free initial import from current client work, never a recovered receipt or a degraded book without approval.
- Review metadata, contents and a bounded sample; clear unsupported-content warnings; explicit acceptance, retry/discard and persisted history.
- Reader isolation, crash recovery, bounded resources, portable backup/restore and complete removal/cleanup retry.

### Exclude

- DRM decryption, fixed-layout/mixed fixed-layout publications, publisher CSS/fonts, scripts, forms, audio/video/media overlays, full SVG/MathML rendering, vertical-layout fidelity and exact printed pagination.
- EPUB editing/export, replacing a published original, published reparse, version history, cross-provider merging, content-hash deduplication, full-text search and downloadable offline packages.
- EPUB CFI or a universal location framework. Existing section/progression persistence remains the default; exact internal-link targets are a separate navigation concern.
- Automatic remote resource retrieval, archive extraction trees, a rendering browser service or client-side archive parsing.

These exclusions narrow fidelity, not data ownership: preserve the original unchanged. State the supported profile before upload/review; do not advertise complete EPUB compatibility.

### Direction assessment

**Why not text-only conversion?** It would be cheaper initially, but lose emphasis, reading order distinctions, notes and intra-document navigation. Repairing that after publication could change locations and require reinterpretation. It does not satisfy the selected scope.

**Why not embed a full EPUB browser reader?** It can preserve publisher layout better, but adds CSS/DOM isolation, archive/resource delivery and a separate navigation/location lifecycle. That is justified for publisher fidelity, not this novel-focused milestone.

**Why both intake channels?** They are two ways to acquire the same durable original, not two parsers. Browser-first is a useful implementation checkpoint, but server-inbox parity belongs in milestone completion. Reuse existing inbox safety and explicit cleanup proof; do not implement a second independent intake scheduler.

**Main cost:** semantic documents and navigation, plus lifecycle integration. ZIP/XML decoding alone is not the milestone. The first checkpoint must exercise these costs before committing to detailed persistence.

## Evidence from the current code

- `backend/internal/reading/service.go`: a private provider interface already separates catalog/open/location lookup; `resolve` chooses BookSource or TXT. Add one concrete EPUB adapter here, not registration machinery.
- `backend/internal/reading/document.go` and `frontend/src/api/reader.ts`: version 1 supports only paragraph/image blocks; the frontend rejects unknown versions/kinds. Rich content requires an explicit contract change.
- `frontend/src/features/reader/ProseRenderer.vue`: currently treats the non-paragraph branch as an image. Make rendering exhaustive when extending prose; do not append format checks to that fallback.
- `frontend/src/features/reader/chinese-conversion.ts`: conversion currently visits paragraph text and image alt only. Structured text traversal must preserve identifiers/targets and leave canonical documents unchanged.
- `backend/internal/library/schema.go`: shared progress/bookmarks use chapter index, normalized position and content revision. No evidence yet requires replacing that persistence model.
- `backend/internal/txtimport/pool.go` and `work.go`: scheduling already has a narrow processing seam, but construction, errors and recovery are TXT-bound. Generalize the real scheduling owner, not TXT's parser/state tables.
- `backend/internal/txtstore/receive.go`, `portable.go`: acquisition is distinct from publication; originals and durable intent participate in the reader-home mutation/snapshot lifecycle. Portable validation does not rebuild content.
- `frontend/src/features/imports/import-queue.ts`: current orchestration and DTOs are TXT-specific. Share queue lifecycle with typed format-specific operations rather than copying the queue or making every receipt carry TXT options.

Format checks against [EPUB 3.3](https://www.w3.org/TR/epub-33/): [spine itemref](https://www.w3.org/TR/epub-33/#sec-itemref-elem) distinguishes primary and auxiliary reading; [TOC navigation](https://www.w3.org/TR/epub-33/#sec-nav-toc) is a hierarchy of targets, not one entry per spine item; [font obfuscation](https://www.w3.org/TR/epub-33/#sec-font-obfuscation) is distinct from DRM. EPUB 2 package/NCX rules still need focused primary-source checking during the contract checkpoint.

## Proposed architecture

### Ownership and flow

```text
Local Import (browser / inbox)
  → shared bounded intake and worker lifecycle
  → EPUB-owned durable receipt + unchanged original
  → bounded EPUB parser/normalizer
  → saved sections, navigation, resource/anchor mappings and diagnostics
  → explicit library publication transaction
  → existing reading service + EPUB adapter
  → versioned semantic prose + authorized opaque resources
  → shared Reading Session / Prose Renderer
```

- `internal/epub`: bounded container/package inspection, URL resolution and semantic normalization; no HTTP, library mutations, home paths or worker ownership. [Go library research](../notes/2026-09-17-go-epub-libraries.md) identifies Readium as the strongest package/navigation reuse candidate. Evaluate it behind bounded local resource access before choosing custom EPUB glue over Go ZIP/XML and existing HTML primitives. Do not adopt its default archive pipeline unchanged, fork several incomplete readers, expose dependency types to storage/HTTP, or build parallel production parsers. Dependency selection remains proposed until the contract proof demonstrates net maintenance benefit.
- `internal/epubstore`: original, durable import state, prepared content/index, publication and provider-owned removal/recovery/portable validation. Library alone owns display metadata, progress, bookmarks and revisions; parsed metadata before admission is import evidence, not a second authoritative shelf record.
- Evolve `txtimport` into a small local-import lifecycle owner as EPUB lands. One shared process budget and reader quiescence barrier; concrete TXT/EPUB dispatch with fair selection so continuous TXT work cannot starve EPUB. No dynamic registry, generic task engine or extra independent worker pool.
- Share confined transfer/finalization and inbox claim mechanics only where both formats have the same invariant. Keep encoding/regex/reparse and archive normalization in their respective owners. Do not rename/rebuild every TXT table as a prerequisite.
- HTTP owns authentication, DTOs and safe response headers. Reading service selects the provider. Renderer receives semantic content, never ZIP paths, OPF metadata or provider parsing instructions.

### Sections, TOC and footnotes

1. Main sequence follows the package spine, not ZIP order, manifest order or TOC order. One supported spine content document is one section initially; no heuristic splitting by headings.
2. Navigation is distinct: preserve hierarchy and map entries to section plus opaque anchor. Several entries may target one section. Extend catalog output with optional navigation rather than duplicating sections or overloading `isVolume`.
3. Normalize document IDs to opaque anchor keys during preparation; links carry validated publication-local targets qualified by the same content revision. No archive href reaches the client.
4. Auxiliary/non-linear and referenced note documents remain reachable but do not become ordinary next/previous or prefetch steps. Proposed representation: indexed auxiliary sections plus explicit main-sequence membership. Keep section ordinals stable within the published interpretation.
5. Use ordinary note navigation with a visible return action before considering popovers. The Reading Session retains the return location; do not create a second independent reader for notes. Auxiliary views must not overwrite the main resume/last-read position. Decide and test auxiliary bookmark semantics in the first checkpoint rather than accidentally accepting indices that shared validation cannot restore.
6. Keep normalized progression for ordinary resumes/bookmarks. An anchor jump resolves after document display, then ordinary main-section progress can be saved. No claim of exact text-position restoration after typography changes. A persisted anchor locator is a separate decision only if the checkpoint demonstrates a real failure of this contract.
7. Missing optional TOC: generate a clearly labeled section list from the spine with review warning. Broken individual targets remain visible as unavailable or produce a review diagnostic; never silently redirect to another chapter. Missing primary readable content is a failure, not a successful partial book.

### Semantic prose and assets

- Define a finite prose schema: supported block types and a small inline vocabulary (text, emphasis, links, ruby, line breaks). Lists/tables have bounded semantic structure, not arbitrary tags/attributes/styles.
- Proposed wire compatibility: add version 2 for richer documents; retain version 1 BookSource/TXT parsing while both are served. Do not silently send new blocks under version 1. Frontend parsing and renderer cases are exhaustive; unsupported versions fail explicitly.
- Preserve language/direction where needed for correct text semantics. Publisher font metrics, generated CSS content and visual placement are not preserved. Unsupported meaningful content yields diagnostics, not silent disappearance; meaningful fallback text remains readable where available.
- Render Vue-owned elements and text; no raw publisher HTML via `v-html`. Conversion visits text leaves/labels without changing opaque IDs or target references. Links/buttons must not trigger reader tap-navigation accidentally.
- Issue opaque publication/revision-bound image and cover resources. Authorization and removal/restore lifetime checks remain shared; archive lookup is EPUB-owned. Validate media type and byte/dimension bounds; never serve arbitrary archive contents as same-origin active content.
- Start with supported raster images. Do not ship a general SVG sanitizer/rasterizer. Common SVG wrappers containing a supported image can normalize to that image only through a narrow parsed rule; unsupported standalone vector art is reported. A cover failure need not make readable prose unusable.
- No remote fetch for referenced publication resources. External web hyperlinks, if retained, must be explicit safe user actions, not background loads; finalize their small URL policy with the content contract.

### Storage and lifecycle

- Keep one immutable `original.epub` under the reader's managed files and disposable intake work separately. Read validated ZIP entries directly; do not extract arbitrary archive paths into a directory tree.
- Persist normalized section documents plus navigation/anchor/resource mappings during preparation. This makes published semantics stable across restarts/parser updates and avoids whole-book parsing on each chapter request. Original image bytes stay in the archive; do not duplicate all binary assets.
- EPUB owns a small receipt/result model, not TXT's active/candidate reparse model. Allocate an attempt generation before work; a completed index and diagnostics become visible atomically. Publication uses the exact reviewed attempt and is idempotent. Published content is immutable in this milestone.
- Receiving, queued/processing, ready/review-required, failed, published and removing are lifecycle concepts; exact tables/constraints and legal states are to be written at the storage checkpoint, not copied mechanically from TXT projections.
- Long archive parsing happens outside SQLite write transactions and mutation gates. Finalization/publication use short guarded transactions; canceled or superseded work cannot install results. Incomplete prepared rows/files remain hidden and have explicit cleanup/recovery ownership.
- Recovery requeues interrupted unpublished work only while quiescent. Removal hides the publication and retires work before deleting owned bytes; incomplete cleanup is observable/retryable. Restore, deletion and shutdown drain transfers/work/resources under existing reader-home ownership.
- Backups include original and authoritative prepared content/mappings consistently. Validate bounded stored documents/references and required originals, without re-normalizing an EPUB or running content during restore. Do not export inbox cleanup authority. Pending/failed/removing states need their own legal missing-file rules.
- A new schema epoch is expected (14 if 13 remains current). **Cutover requires confirmation before production edits:** retain old homes/backups untouched, verify fresh isolated homes, no migration or automatic reset. Rollback uses the prior application with its compatible old home; never open a new epoch with the old app. Check the current epoch at implementation time.

### Practical input protections

EPUB is an untrusted archive, so these are necessary boundary checks, not a general security overhaul:

- Bound compressed input, central-directory entry count, total declared and actually read uncompressed bytes, per-entry bytes, document/nesting/node counts, sections and resources; enforce cancellation/deadlines while decompressing/parsing, not just ZIP header claims.
- Reject ambiguous duplicate normalized names, escaping/absolute paths, unsupported encryption/compression and broken required references. Resolve URLs relative to their owning package/document; allow legitimate internal `../` references only when the resolved path stays within the archive. Decode consistently before lookup; fragments are not entry names.
- Do not resolve external XML entities/DTDs or fetch schemas. Handle normal EPUB 2 declarations using a bounded parser policy, not network access.
- Do not classify every `encryption.xml` as DRM: ignored obfuscated fonts do not require decryption. Encrypted required reading content is unsupported; expose a safe category.
- Validate the package's fixed-layout declarations and spine overrides, not only the filename. Unsupported primary content fails or requires a supported declared fallback; do not silently skip primary sections.
- Stable client error categories and bounded diagnostics; raw paths/parser details stay out of public responses. Numerical limits are named constants chosen at the first checkpoint, not new knobs for every theoretical case.

## Delivery checkpoints

1. **Contract proof, before broad plumbing.** Evaluate Readium's EPUB parser through its existing resource seam (or narrower exported package/navigation functions): enforce local-only bounded/cancellable reads, preserve fatal errors that optional parsing may suppress, and check the actual build/dependency graph. Prefer standard-library EPUB glue if reliable integration requires a substantial fork or duplicates the reused logic. No dependency adoption merely on README claims. Build minimal synthetic EPUB 2/3 fixtures containing a main sequence, multiple TOC anchors in one file, cross-document notes and illustrations. Validate parser/semantic document/renderer/navigation together. Freeze the version-2 schema, section/TOC/auxiliary rules, semantic support matrix, limits and exact failure categories. Check EPUB 2 primary references. If this requires a fundamentally different location or renderer model, stop and revisit scope rather than stacking exceptions.
2. **Durable provider and lifecycle.** Write exact DDL/legal-state table after cutover approval. Implement original acquisition, preparation, generation guards, publication, reading/resources, portable validation and removal/recovery. Extract the shared import scheduler/transfer seam with TXT behavior preserved. Verify at storage/reading boundaries before exposing unfinished imports.
3. **Product integration.** Browser and inbox EPUB routes, one Local Import queue/history/review flow, cover/detail/shelf capability wiring, semantic reader/TOC/note integration and existing preference behavior. Preserve TXT-specific controls only for TXT. Do not expose EPUB reparse controls.
4. **Composed verification and documentation.** Fresh isolated real-server upload and inbox journeys, read/link/note/resume/bookmark, backup/restore/removal, plus focused TXT/BookSource regressions. Update current usage/architecture docs when functionality actually lands. Complete the plan only at the recorded verification scope.

Each checkpoint is a cohesive tested commit or small sequence, not permission to expose a half-working EPUB workflow. Keep this plan's Current State/Next Action/Verification current at meaningful stopping points.

## Verification strategy

Use a few generated synthetic archives rather than a large copyrighted corpus. Fixtures cover format contracts, not special-case book names.

- Parser: EPUB 2/3 normal case plus one feature-dense semantic/navigation fixture; table-driven hostile archive/reference/limit cases exercising real boundary protections.
- Storage/lifecycle: exact reviewed publication, canceled/interrupted work, cleanup retry, cross-reader denial, portable round trip and quiescent restore. Reuse shared intake tests; add only EPUB-specific and mixed-format scheduling cases, not a copy of every TXT test.
- Reading/frontend: rich rendering and conversion preserve targets; main sequence vs TOC; note return and no auxiliary progress overwrite; image toggle performs no image requests; stale revisions reject reads/writes/resources; TXT/BookSource version-1 content remains readable.
- Race tests for changed scheduling/store/restore boundaries; focused normal tests first. Frontend typecheck/build and relevant component tests. One composed real-server journey for each intake channel, desktop/mobile reader inspection, no deployment/load claim.
- No universal EPUB conformance suite, benchmark project or large random corpus unless specific failures justify them. Optional legally supplied local EPUBs are compatibility evidence, never clean-checkout dependencies.

## Current State

Planning only. User accepted the product direction and requested deeper consideration. Current provider, prose, import-worker, portable-state and frontend seams were inspected; EPUB 3.3 spine/navigation/font-obfuscation requirements were checked. Four Go EPUB readers were compared against commit-pinned source; Readium is the proposed reuse candidate, not yet an adopted dependency. See the linked research note for tradeoffs and fallback criteria. No EPUB code, dependencies, schema or tests have been added. Existing untracked frontend prototypes are unrelated and remain untouched.

## Next Action

Review/accept the bounded proposal and confirm the fresh-data schema cutover policy. Then authorize implementation beginning with checkpoint 1, including the bounded Readium reuse proof before settling parser ownership. Resolve auxiliary bookmark behavior, exact semantic schema and EPUB 2 parser compatibility in that checkpoint before storage DDL is finalized; do not turn these implementation details into a speculative universal model.

## Verification

Performed: read-only code/document inspection, targeted EPUB 3.3 primary-reference checks and commit-pinned Go library research. Readium's parser error handling and archive fetcher's context handling were independently rechecked against source. Documentation whitespace/path checks are the only intended checks in this planning change.

Not performed: EPUB parsing experiments, code tests, browser validation, performance measurements, EPUB 2 primary-reference verification or real-book compatibility checks. Architectural feasibility is reasoned from current seams, not yet demonstrated by a working EPUB implementation.
