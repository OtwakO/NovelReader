---
status: completed
updated: 2026-09-02
---

# Reading Document Foundation

## Goal

Establish the smallest concrete reading foundation needed for maintainable BookSource prose and inline-image reading now, while preserving clean seams for future TXT/EPUB providers, image-sequence reading, and audio without implementing speculative provider or media frameworks.

Done means:

- the backend exposes an explicit versioned prose-document contract;
- prose blocks preserve ordered paragraphs and inline images;
- image origins and source behavior remain backend-only behind opaque NovelReader resource references;
- the Vue Reader delegates prose presentation to a focused renderer;
- inline images are centered and available alt text appears beneath them as a centered caption;
- existing BookSource reading, chapter caching, source recovery, conversion, and progress behavior remain operational;
- the architecture does not introduce unused provider, manga, audio, or progress abstractions.

## Scope

Included:

- formalize the existing processed chapter response as a `Prose Document` rather than a universal content model;
- define a versioned, discriminated wire contract for prose;
- represent inline images with opaque NovelReader-controlled resource references;
- preserve source-aware image loading, private upstream URLs, headers, cookies, sessions, request options, and portable decoding on the backend;
- retain ordered text/image content and capture useful image alternative text when available;
- extract prose markup and styles from `ReaderView.vue` into a cohesive `ProseRenderer.vue`;
- render images responsively and centered using semantic figure/image/caption markup;
- show non-empty image alternative text beneath the image as a centered caption;
- provide a localized, non-destructive image failure state;
- prevent image interaction from accidentally triggering Reader tap navigation where required;
- update current architecture documentation after implementation makes the new design current.

Excluded:

- TXT, EPUB, CBZ, manga, or audio providers;
- image-sequence, fixed-layout, or audio document implementations;
- automatic manga classification;
- a provider registry or plugin framework;
- generalized provider capability interfaces before a second provider exists;
- cross-provider edition linking;
- cross-section infinite scrolling;
- offline image-byte storage or a new resource database;
- Android bitmap API emulation;
- migration to structured modality-specific progress locations;
- paginated prose or manga layout settings.

## Accepted Approach

Follow [the reading document and resource decision](../decisions/0002-reading-documents-and-resources.md) incrementally.

The immediate production path remains BookSource-specific behind the backend seam:

```text
BookSource content execution
  → prose processing
    → versioned Prose Document
      → opaque image resource references
        → authenticated source-aware image delivery

ReaderView reading session
  → ProseRenderer
    → paragraph or centered figure/caption
```

### Initial contract direction

The exact Go and TypeScript names may follow local conventions, but the wire semantics should be equivalent to:

```ts
interface ProseDocumentEnvelopeV1 {
  version: 1;
  document: {
    kind: 'prose';
    title: string;
    blocks: ProseBlock[];
  };
  offlineCopy: boolean;
}

type ProseBlock =
  | { kind: 'paragraph'; text: string }
  | {
      kind: 'image';
      resource: { href: string; mediaType?: string };
      alt?: string;
    };
```

The backend constructs `resource.href`. The frontend does not construct provider or storage references from an image index and never receives the upstream URL.

Compatibility for existing cached processed chapters should use the smallest safe strategy supported by the current cache shape. Do not add general migration machinery unless inspection proves it is necessary.

### Inline-image presentation

- Use semantic `<figure>`, `<img>`, and conditional `<figcaption>` elements.
- Center the image within the prose column.
- Constrain it responsively to the available prose width and viewport height without cropping or stretching.
- When non-empty alternative text is available, use it as the image `alt` value and display the same reader-facing text beneath the image as a centered caption.
- When no meaningful alternative text is available, use the agreed localized generic accessible alternative and omit the visible caption rather than displaying placeholder text beneath the image.
- Image failure remains local to the figure and must not replace otherwise readable chapter content.

## Decisions

### Normalize current content as prose

**Decision:** The current chapter content is modeled as a Prose Document containing prose blocks, not as a future-complete universal media tree.

**Why:** It accurately represents the active BookSource use case and creates a real renderer seam without forcing manga and audio into prose semantics.

**Alternatives:** Keep generic `ContentBlock`; introduce all future document kinds immediately.

**Revisit when:** The first non-prose modality is selected for implementation.

### Backend-issued resource references

**Decision:** Image blocks carry an opaque NovelReader-controlled resource reference produced by the backend.

**Why:** This hides BookSource URLs and behavior, keeps Vue provider-agnostic, and gives future local/archive resources the same frontend contract.

**Alternatives:** Continue exposing only an image index and let Vue construct the endpoint; expose upstream image URLs.

**Revisit when:** Offline packages or durable resource identity require a persisted resource catalog.

### Separate prose rendering from the reading session

**Decision:** Extract a focused prose renderer while retaining loading, navigation, source recovery, common chrome, and session coordination in the Reader shell.

**Why:** This reduces the current `ReaderView.vue` responsibility and provides the natural dispatch seam for future modality renderers.

**Alternatives:** Keep adding block and modality branches directly to `ReaderView.vue`; create a universal renderer.

**Revisit when:** Concrete renderer responsibilities show that some supposedly common session behavior is modality-specific.

### Delay provider interfaces and progress migration

**Decision:** Do not add provider capability interfaces or structured modality locations during this workstream.

**Why:** BookSource and prose are currently the only real implementations. The second provider and second modality will supply the constraints needed to design those interfaces correctly.

**Alternatives:** Build a complete provider/plugin and generalized progress framework now.

**Revisit when:** TXT/EPUB or image-sequence/audio work is accepted.

## Progress

- [x] Agree on provider/document/resource/renderer architectural direction.
- [x] Record the durable cross-cutting decision.
- [x] Record the initial inline-image presentation requirement.
- [x] Inspect the existing processor, cache, API, Reader, conversion, progress, and image tests against the accepted contract.
- [x] Finalize the smallest compatibility strategy for current chapter responses and cached blocks.
- [x] Implement the backend prose-document and opaque-resource response seam.
- [x] Extract and integrate the prose renderer with centered figures and captions.
- [x] Add focused regressions for the contract, private resource delivery, ordering, caption behavior, and image-local failure.
- [x] Run affected backend and frontend verification.
- [x] Update current architecture documentation and mark this plan complete.

## Current State

The repository already supports an initial image path:

- `backend/internal/processor/processor.go` emits ordered text/image blocks;
- `backend/internal/book/chapter_cache.go` persists processed content plus private image URLs;
- `backend/internal/api/chapter_cache.go` serializes block indices;
- `backend/internal/api/chapter_image.go` resolves stored image URLs through the active BookSource and authenticated Reader Account;
- `backend/internal/book/image.go` applies source-aware fetching and portable `imageDecode` behavior;
- `frontend/src/api/reader.ts` defines the current block response and constructs indexed image URLs;
- `frontend/src/features/reader/ReaderView.vue` directly renders paragraphs and images.

This is a useful implementation skeleton, but the current names and response shape appear universal despite being prose-specific, resource URL construction leaks backend endpoint knowledge into the Reader API client, and prose rendering remains embedded in the already broad Reader view.

The foundation is implemented. Chapter responses now expose a versioned Prose Document with backend-issued resource references; the processor captures image alternative text and emits explicit paragraph/image blocks; old text-only cached chapters are adapted at the response seam; and the frontend uses a focused prose renderer with centered responsive figures, conditional centered captions, fallback accessible alternatives, and image-local failure states.

A live/local reproduction from `凡人修仙传@QQ阅读` in the aggregated source exposed inline `data:image/svg+xml;base64,...` chapter resources. The resource loader now decodes bounded inline image data locally, tolerates the source's malformed trailing `,{` option fragment after complete base64, and hydrates the reader-owned source session before all image resolution. Ordinary remote resources retain the existing source-aware HTTP path.

Reader typography preferences now include a persisted image-visibility control. Disabling images removes image figures from the prose render tree entirely, so they reserve no layout space and initiate no image requests.

Compatibility remains at the response seam: text-only old caches synthesize paragraph blocks, while old image-bearing caches translate their legacy `text` block kind to `paragraph`. No cache migration is required.

No provider registry, future modality implementation, progress migration, or offline image-byte storage was introduced.

## Next Action

This workstream is complete. Select the next reading/provider capability from demonstrated product need. Provider capability interfaces should wait for the first non-BookSource provider; image-sequence documents and structured locations should wait for manga/image-sequence work.

## Verification

Verified:

- complete backend Go suite after inline-data and legacy-cache fixes: `go test ./...`;
- complete frontend suite: 48 files, 161 tests passed;
- frontend TypeScript check: `npm run typecheck`;
- frontend production build: `npm run build` (278 modules transformed);
- processor regression covers ordered paragraphs/images and captured source alt text;
- API regressions cover the versioned prose contract, opaque image references, cache fallback, and source-aware image retrieval;
- renderer regression covers semantic figures, source captions, fallback alternatives, image-local failure, and complete image-block omission when images are disabled;
- cache regression covers legacy `text` blocks in image-bearing cached chapters;
- scoped static inspection found no diagnostics, import cycles, or duplicated lines in the changed reading modules; Go diagnostics were verified by the complete Go test suite because `gopls` is unavailable.

Verification limit:

- the failing local cached `凡人修仙传@QQ阅读` resource was inspected and minimized into a deterministic backend regression; the actual endpoint still requires restarting the user's running server with this build and refreshing the chapter to verify in-browser;
- responsive centering and caption layout are covered structurally and by the production build, not by screenshot comparison.

## Open Questions

None for this completed scope. Additional lazy-image attributes should be added only from demonstrated BookSource evidence. The accepted cache strategy is response-seam translation for old text-only entries, and missing source alt text uses a localized generic accessible alternative without a visible caption.
