---
status: in-progress
updated: 2026-09-17
---

# EPUB support: structured novel reading

## Goal

Import a DRM-free reflowable EPUB, review it, add it to the existing shelf, read its text and illustrations with meaningful formatting/navigation, resume and bookmark it, and back it up or remove it through the existing reader-owned lifecycle.

This is a structural milestone, not a second reader application. Done means a reliable, explicitly bounded novel-reading profile, not universal EPUB reading-system conformance.

## Accepted direction and authorization

The user selected structured reflowable EPUB 2/3 reading and both browser and server-inbox import. After reviewing the proposal and reference research, the user authorized implementation, starting with the bounded contract proof. Keep existing reader homes/backups untouched. Schema cutover remains a separate gate before storage changes; checkpoint 1 does not change the schema.

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

### Reader references and performance

The user clarified that original publisher layout is unnecessary and efficiency/performance matter. [Legado/web-legado/Readest findings](../notes/2026-09-17-epub-reader-reference-lessons.md) support app-owned responsive typography while preserving semantics. Readest's polished reading layout does not require adopting its Foliate browser-rendering stack; Readium Go was evaluated separately as a package-parsing candidate.

- Prepare and persist semantic sections once, processing one section at a time. Ordinary reading must not download, inflate, parse or mount the whole book.
- Keep images separate and lazy; preserve validated dimensions where available to reduce layout shift. Reader style changes reflow local content, not trigger reimport.
- Reuse the shared chapter loader's request deduplication and bounded recent cache/prefetch. Check whether rich-section byte limits or byte-weighted retention are needed; do not create an EPUB-only cache owner.
- Include a large single-spine document in the first proof. A section can contain much more than a conventional chapter; do not assume five cached sections means small memory, or add speculative splitting/virtualization before measuring.
- Preserve the current reading mode. Use fluid single-column phone text and comfortable desktop measure; no new full-book pagination, dual original/parsed modes or unrelated reader redesign in this milestone. Borrow Readest's quiet controls and contextual navigation, not its entire feature set.
- Record import time/peak memory, prepared size, cold/warm section payload/load and image layout movement on a small normal fixture and a large-section/image-heavy fixture. These focused observations inform limits and parser choice; no benchmark framework or unmeasured speed claims.

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

## Accepted architecture

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

- `internal/epub`: bounded container/package inspection, URL resolution and semantic normalization; no HTTP, library mutations, home paths or worker ownership. **Selected after the build proof:** focused EPUB rules over Go ZIP/XML and existing HTML/text primitives, with no new dependency. Readium's package-level coupling brings unrelated cloud/gRPC/telemetry dependencies even through its narrower exported parsing functions. The user chose the focused Go approach after reviewing that measured cost. Do not fork Readium, maintain parallel parsers or expose archive references in HTTP/storage-facing prose documents. Format-rule ownership and conformance tests are the cost of this choice.
- `internal/epubstore`: original, durable import state, prepared content/index, publication and provider-owned removal/recovery/portable validation. Library alone owns display metadata, progress, bookmarks and revisions; parsed metadata before admission is import evidence, not a second authoritative shelf record.
- Evolve `txtimport` into a small local-import lifecycle owner as EPUB lands. One shared process budget and reader quiescence barrier; concrete TXT/EPUB dispatch with fair selection so continuous TXT work cannot starve EPUB. No dynamic registry, generic task engine or extra independent worker pool.
- Share confined transfer/finalization and inbox claim mechanics only where both formats have the same invariant. Keep encoding/regex/reparse and archive normalization in their respective owners. Do not rename/rebuild every TXT table as a prerequisite.
- HTTP owns authentication, DTOs and safe response headers. Reading service selects the provider. Renderer receives semantic content, never ZIP paths, OPF metadata or provider parsing instructions.

### Sections, TOC and footnotes

1. Main sequence follows the package spine, not ZIP order, manifest order or TOC order. One supported spine content document is one section initially; no heuristic splitting by headings.
2. Navigation is distinct: preserve hierarchy and map entries to section plus opaque anchor. Several entries may target one section. Extend catalog output with optional navigation rather than duplicating sections or overloading `isVolume`.
3. Normalize document IDs to opaque anchor keys during preparation; links carry validated publication-local targets qualified by the same content revision. No archive href reaches the client.
4. Auxiliary/non-linear and referenced note documents remain reachable but do not become ordinary next/previous or prefetch steps. Proposed representation: indexed auxiliary sections plus explicit main-sequence membership. Keep section ordinals stable within the published interpretation.
5. Use ordinary note navigation with a visible return action before considering popovers. The Reading Session retains the return location; do not create a second independent reader for notes. Auxiliary views must not overwrite the main resume/last-read position. Accepted: allow auxiliary bookmarks and reopening them, while auxiliary reads and bookmark operations must not overwrite main resume/last-read. Test that behavior in the first checkpoint rather than accidentally accepting indices that shared validation cannot restore.
6. Keep normalized progression for ordinary resumes/bookmarks. An anchor jump resolves after document display, then ordinary main-section progress can be saved. No claim of exact text-position restoration after typography changes. A persisted anchor locator is a separate decision only if the checkpoint demonstrates a real failure of this contract.
7. Missing optional TOC: generate a clearly labeled section list from the spine with review warning. Broken individual targets remain visible as unavailable or produce a review diagnostic; never silently redirect to another chapter. Missing primary readable content is a failure, not a successful partial book.

### Semantic prose and assets

- Define a finite prose schema: supported block types and a small inline vocabulary (text, emphasis, links, ruby, line breaks). Lists/tables have bounded semantic structure, not arbitrary tags/attributes/styles.
- Proposed wire compatibility: add version 2 for richer documents; retain version 1 BookSource/TXT parsing while both are served. Do not silently send new blocks under version 1. Frontend parsing and renderer cases are exhaustive; unsupported versions fail explicitly.
- Preserve language/direction where needed for correct text semantics. Publisher font metrics, generated CSS content and visual placement are not preserved. Unsupported meaningful content yields diagnostics, not silent disappearance; meaningful fallback text remains readable where available.
- Render Vue-owned elements and text; no raw publisher HTML via `v-html`. Conversion visits text leaves/labels without changing opaque IDs or target references. Links/buttons must not trigger reader tap-navigation accidentally.
- Issue opaque publication/revision-bound image and cover resources. Authorization and removal/restore lifetime checks remain shared; archive lookup is EPUB-owned. Validate media type and byte/dimension bounds; never serve arbitrary archive contents as same-origin active content.
- Start with supported raster images. Do not ship a general SVG sanitizer/rasterizer. Common SVG wrappers containing a supported image can normalize to that image only through a narrow parsed rule; unsupported standalone vector art is reported. A cover failure need not make readable prose unusable.
- No remote fetch for referenced publication resources. Accepted: retain HTTP/HTTPS external links for deliberate opening in a separate tab with opener isolation; never fetch them in preparation or prefetch. Other schemes remain inactive. Transport/renderer integration must enforce this policy.

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

1. **Contract proof, before broad plumbing — in progress.** Dependency gate complete: Readium compiled successfully, but the user selected focused Go parsing after its actual package graph was measured (see Current State). Bounded archive/XML, package inventory/navigation inspection, package support checks and isolated section normalization are implemented; the complete parser/section design is not yet accepted as proven. Measure resource use and include a large single-spine document before accepting that design. Build minimal synthetic EPUB 2/3 fixtures containing a main sequence, multiple TOC anchors in one file, cross-document notes and illustrations. Validate parser/semantic document/renderer/navigation together. Freeze the version-2 schema, section/TOC/auxiliary rules, semantic support matrix, limits and exact failure categories. Check EPUB 2 primary references. If this requires a fundamentally different location or renderer model, stop and revisit scope rather than stacking exceptions.
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
- No universal EPUB conformance suite, benchmark project or large random corpus unless specific failures justify them. Optional legally supplied local EPUBs belong in repository-root `test-epubs/`, excluded from Git and Docker build contexts; they are compatibility evidence, never clean-checkout or CI dependencies. The user-supplied development example is `test-epubs/全职高手 (蝴蝶蓝).epub`; see Verification for the limited inspection performed. Real-book checks must be explicit local runs; default tests use synthetic archives. Do not commit extracted book content, images or generated reports containing substantial book text.

## Current State

Checkpoint 1 is partially implemented; no EPUB routes or publication capability are exposed.

- Dependency gate: isolated Readium v0.16.0 (`e36e46c88ad82c2ff9bfabc5657d988bd8c99cd4`) build succeeded on Go 1.27.0 linux/amd64. A caller doing only `epub.NewParser(nil)` had 790 compiled packages, 562 non-standard, 69 distinct modules including the proof module, and 310 cloud/AWS/gRPC/telemetry packages. Unstripped executable: 48,146,598 bytes. These are build-footprint observations, not runtime parsing benchmarks. No PDF-native dependency was observed in this import graph. The user explicitly selected focused Go implementation instead; no bounded Readium fetcher was built after this decision.
- Reproduction remains isolated from the application: create a temporary module, `go get github.com/readium/go-toolkit/pkg/parser/epub@v0.16.0`, compile `package main; import "github.com/readium/go-toolkit/pkg/parser/epub"; func main() { _ = epub.NewParser(nil) }`, then inspect `go list -deps -json .`. The local experiment and graph are in ignored `reference/epub-parser-proof/`; no upstream code or dependency was added to the application.
- `backend/internal/epub/` now owns bounded ZIP indexing/metadata reads, local URI resolution, bounded cancellable XML token decoding (including UTF-16 BOM and fixed HTML entities), and container/OPF inventory inspection. It preserves spine order, linear/non-linear membership, navigation IDs/properties and fallback IDs without treating TOC order as reading order. `Inspect` explicitly does **not** return a prepared/publishable book.
- Navigation inspection now reads EPUB2 spine-declared NCX `navMap/navPoint` or EPUB3 manifest-declared nav `epub:type="toc"` lists. It preserves document-order hierarchy, grouping headings, mixed inline labels and distinct path/fragment targets independently of spine. EPUB3 nav and legacy NCX are never merged; missing/invalid EPUB3 nav is diagnosed rather than silently substituted with NCX. XML namespaces are checked; base overrides are diagnosed as invalid optional navigation instead of ignored or fetched.
- Navigation uses the existing metadata/XML/context limits plus a 10,000-entry limit. Cancellation/limits and archive read failures remain fatal. Missing, malformed, empty or ambiguous optional navigation produces code-only diagnostics. Nonlocal, escaping, absent or nonmanifest targets retain label/hierarchy but no actionable target and are marked unavailable; missing labels have their own diagnostic. No target content or external DTD is fetched. Fragment existence, target media support and prepared-anchor mapping remain pending; empty navigation still needs a labeled spine-derived list during preparation.
- Package support checks now reject declared fixed layout (EPUB 3 rendition vocabulary, including aliases and spine overrides; legacy fixed-layout metadata and Apple/Kobo display options). Encryption declarations permit only known IDPF/Adobe font obfuscation on declared font resources, which remain unused; encrypted non-font resources and other encryption are unsupported. No decryption or publisher-font rendering is implemented.
- Every spine item, including non-linear items, resolves through declared media fallbacks to an existing XHTML resource. Missing references/files, cycles and unsupported terminal media fail instead of silently dropping content. `SpineItem.ContentID` records the selected item while retaining original identity, order and linearity; fallback use has a code-only diagnostic. Multiple manifest IDs may alias one path when their media declarations agree; conflicting media declarations are rejected. The local example exposed why rejecting all aliases was unnecessarily strict; the fix applies to resource identity generally, not that book's IDs.
- `NormalizeSection` now converts one bounded XHTML document to a finite semantic tree: headings, paragraphs/groups, emphasis/strong/strike, sub/superscripts, code/preformatted text, lists, tables, ruby, line breaks, note roles and image/link bindings. It preserves language/direction and whitespace semantics. The shared bounded XML tree now has its own owner (`xml_tree.go`) rather than duplicating navigation parsing. Styles/scripts and arbitrary publisher attributes are not emitted; unsupported meaningful content retains fallback text plus code-only diagnostics.
- Normalized anchors are opaque and separate from original IDs; ambiguous original IDs cannot resolve. Internal links and images use section-local opaque binding keys with private source-reference maps. HTTP/HTTPS external links are explicit-action candidates only; other schemes and remote images are inactive. A narrow SVG wrapper containing one image can yield a pending image binding; this is not SVG rendering or asset validation. Base overrides fail rather than silently redirect content. Empty readable content fails.
- This is an **internal preparation model**, not the frozen version-2 wire contract. It remains under `internal/epub` until the renderer proof establishes the final shared shape; existing version-1 endpoints are unchanged. No unvalidated binding is exposed for reading. Publication-wide link/anchor resolution, image-byte validation, readability after binding resolution and staged/streamed persistence remain next.
- Section input is capped at 16 MiB and shares the XML depth/token bounds. Archive reads enforce actual bytes cumulatively (up to 1 GiB for content), rather than trusting declared sizes. No section segmentation/virtualization was added. Ordered-list starts retain signed 32-bit semantics; table spans outside the initial 1–1000 range are diagnosed and omitted. These preparation limits/profile still need the renderer/cache-size proof before freezing.
- Initial inspection limits: 256 MiB compressed archive, 20,000 ZIP entries, 1 GiB declared expanded size, 4 MiB per metadata read, 16 MiB cumulative metadata read budget, XML depth 128 / 200,000 tokens, 10,000 manifest items and 5,000 spine items. These are internal limits, not settings. Content/image normalization needs its own actual-byte budgets; current inspection does not decompress chapters or images.
- The optional local example passes inventory/navigation inspection (16,778,888 compressed bytes, 1,760 manifest items, 1,737 spine entries, 1,736 navigation entries, no navigation or support diagnostics). The explicit normalization check also processed all 1,737 spine sections, one at a time: 18,359,861 XHTML input bytes, 25,493,822 semantic-tree JSON bytes, 3,472 pending image bindings, zero local link bindings/anchors, and no normalization diagnostics in 1.62 seconds locally. This book therefore does not exercise internal-link/anchor compatibility; synthetic tests do. Cover/image bytes and target fragments remain unvalidated; do not call this full-book compatibility.
- No application dependencies, reader schema, existing homes/backups or frontend code changed. Untracked frontend prototypes remain untouched.

## Next Action

Continue checkpoint 1 with publication-wide anchor/link and image-resource validation, then the version-2 prose contract/renderer proof. Isolated XHTML semantic normalization is implemented, but does not resolve targets or prove usable image resources. Orchestration must reuse the archive index, process sections incrementally and retain only bounded indexes/staged results, not a whole-book content tree. Hierarchical NCX/nav inspection is implemented but is not a prepared catalog. The inventory deliberately is not a complete validity/admission check. Then exercise the version-2 prose contract/renderer with links, notes and image resources; test the accepted auxiliary-bookmark/main-resume isolation before storage DDL. Measure full preparation and section costs, not just metadata inspection. Confirm fresh-data schema cutover before checkpoint 2; do not reopen dependency selection without new evidence.

## Verification

Passed from `backend/`:

- `go test ./internal/epub -count=1` and `go test -race ./internal/epub -count=1`: synthetic package/order/reference cases, archive and XML boundaries, cancellation, EPUB 2 declarations/entities/UTF-16, NCX/nav hierarchy and selection, mixed labels/group headings, repeated-document fragments, spine independence/non-linear targets, unavailable targets, optional-navigation diagnostics, base overrides, fatal navigation byte/depth/entry/cumulative limits, fixed-layout declarations, font obfuscation versus encryption, spine fallback/cycle/missing-file handling, consistent versus conflicting resource aliases, semantic structure/whitespace, opaque/private reference separation, safe/inactive links/images, ambiguous anchors, SVG raster wrappers and bounded section normalization. Default runs skip the explicitly opt-in local fixture.
- `NOVELREADER_EPUB_FIXTURE="$(pwd)/../test-epubs/全职高手 (蝴蝶蓝).epub" go test ./internal/epub -run '^TestLocalPackageInspection$' -count=1 -v`: package support/inventory/navigation only; logs counts and diagnostic codes, not title/text/hrefs/assets. Use an absolute fixture path because Go tests run in their package directory; an earlier relative-path invocation failed before reading the book.
- `NOVELREADER_EPUB_FIXTURE="$(pwd)/../test-epubs/全职高手 (蝴蝶蓝).epub" go test ./internal/epub -run '^TestLocalSectionNormalization$' -count=1 -v`: full spine section normalization only; logs counts/diagnostic codes, not book content or original references. Sections are not retained together.
- `go test ./internal/epub -run '^$' -bench '^BenchmarkNormalizeSection$' -benchmem -count=1`: normal synthetic section 18,551 ns/op, 17,968 B/op; 1.1 MB / 10,000-paragraph single document 31,480,811 ns/op, 28,496,645 B/op, on the same Go/CPU below. Construction excluded; allocations are not peak memory, and this excludes image decoding, target binding, persistence and rendering.
- `go test ./internal/epub -run '^$' -bench '^BenchmarkInspect$' -benchmem -count=1`: synthetic metadata inspection with an approximately 19 MiB single-spine body took 46,485 ns/op, 26,295 B/op, 376 allocations/op on AMD Ryzen 9 5900X, Go 1.27.0 linux/amd64. Fixture construction is excluded; chapter bytes are not decoded. This is not peak memory, full preparation, rendering performance or a comparison with Readium.

Navigation implementation consulted the EPUB 2 OPF NCX requirements at <https://idpf.org/epub/20/spec/OPF_2.0.1_draft.htm> and EPUB 3.3 navigation restrictions/TOC definition at <https://www.w3.org/TR/epub-33/>. Earlier reference-reader research is linked above. The latest AFT inspection timed out during its rescan; it also has no authoritative Go diagnostics because `gopls` is unavailable. Focused Go tests/race compilation are the authority. The metadata benchmark above predates navigation/support parsing and was not rerun for this slice.

Not performed: publication-wide target-fragment/media-byte validation and staged preparation, peak-memory measurement, browser validation, EPUB backup/restore, schema cutover, complete local-book compatibility, container build or hosted CI. Checkpoint 1 remains unfinished.
