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
- TXT upload, validation, encoding normalization, TOC analysis and preview, confirmed index publication, reading, and later bounded reparsing;
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
7. Preserve uploaded source material and publish parsing/index changes atomically. A failed import or reparse must not replace an existing readable index.
8. Prefer an explicit compile-time provider dispatch switch initially. Introduce a registry only if real registration needs emerge.
9. Refactor existing BookSource code only where ownership must change for the first TXT vertical slice; do not reorganize the subsystem for symmetry.
10. Place future features by the narrowest domain that owns their invariant: library-wide organization in the library module; origin-neutral reading coordination in the reading workflow; typography/page/audio behavior in the modality document/renderer; and acquisition, parsing, synchronization, replacement, or recovery in the owning provider. When a feature is shared by only some providers, extract a small capability only after the second real implementation demonstrates the common contract.

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

### Normalize TXT before indexing

**Decision:** The TXT design will choose and document one canonical text representation before persisting section offsets; encoding and parser configuration remain TXT-owned.

**Why:** Stable offsets and deterministic reopening require one explicit coordinate system. Per-encoding or ambiguous character offsets would make reading and reparsing fragile.

**Alternatives:** Persisting copied chapter bodies simplifies reads but duplicates full content and complicates reparsing and source-file preservation.

**Revisit when:** Measurements show bounded section materialization is necessary for performance or transformations that cannot be served from indexed canonical text.

## Progress

- [x] Confirm the unified-library, provider-owned implementation direction.
- [x] Create the dedicated feature branch and durable implementation plan.
- [ ] Map the current BookSource shelf, catalog, document, resource, progress, and deletion ownership against the intended seams.
- [ ] Define the smallest provider-neutral identity and persistence transition, including compatibility and rollback constraints.
- [ ] Define the first TXT vertical-slice contract and import states with realistic file/encoding/TOC failure cases.
- [ ] Implement the provider-neutral library/storage foundation with BookSource regressions.
- [ ] Implement TXT upload, normalization, TOC preview/confirmation, and atomic index publication.
- [ ] Connect TXT sections to the existing Prose Document and Reading Session path.
- [ ] Add unified shelf origin filtering and focused TXT management UI.
- [ ] Add bounded TXT reparsing and progress/bookmark relocation only after the basic import/read path is stable.
- [ ] Update current architecture documentation with the concrete implemented interfaces and storage ownership.

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

NovelReader currently has a BookSource-oriented `book` domain with normalized title/author shelf identity, active and alternate source bindings, catalog synchronization, prose documents, opaque image resources, progress, and bookmarks. The accepted reading decision already defines the provider/document/resource/modality separation and states that TXT or EPUB should establish the first real provider abstraction.

No production code, schema, HTTP interface, or frontend behavior has changed for this workstream yet. The architectural direction is accepted, but concrete identity, storage transition, interface signatures, import limits, and migration/rollback details still require design against the current code before implementation.

## Next Action

Inspect the directly affected BookSource shelf and reader paths, schema ownership, API contracts, frontend shelf/reader state, and focused tests. Produce a bounded implementation design for the first foundation slice: provider-neutral shelf identity plus BookSource preservation, followed by the smallest complete TXT import-to-reading vertical slice. Confirm any destructive schema, compatibility, or UX decision before editing production code.

## Verification

Verified:

- `git status --short --branch` showed clean `main` tracking `origin/main` before branching.
- `git switch -c feat/multi-provider-library` created the feature branch successfully on 2026-09-10.
- The accepted approach was checked against `docs/decisions/0002-reading-documents-and-resources.md` and its Future Provider Adoption Reference.

Still needed:

- No code tests or builds have been run because this checkpoint changes only planning and project routing.
- Current schema and caller impact have not yet been mapped.
- No data migration or rollback mechanism has been accepted.
- TXT file size, accepted encodings, normalization representation, parser configuration, confidence/preview behavior, and resource limits remain to be designed from concrete requirements.
- Hosted CI, release images, and browser workflows are outside this planning checkpoint.

## Open Questions

- Should incomplete TXT imports appear in a separate import queue until confirmed, or as explicit processing items within the unified library?
- Which canonical normalized TXT representation and offset/location strategy best preserves deterministic reads and bounded reparsing?
- What is the smallest compatible transition from the current title/author logical-book identity to provider-neutral shelf identity without unnecessary pre-public migration machinery?
- Which progress and bookmark anchors are required in the first TXT delivery, and which relocation behavior can wait for the reparse milestone?
