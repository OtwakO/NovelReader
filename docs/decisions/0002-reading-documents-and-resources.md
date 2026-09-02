---
status: accepted
---

# Normalize Reading into Documents, Resources, and Modality Renderers

## Context

NovelReader currently reads BookSource chapters as mostly prose with occasional inline images. The backend extracts and processes chapter content, retains private image origins in the chapter cache, and serves images through authenticated source-aware endpoints. The Vue Reader displays ordered text and image blocks.

Future work may add local TXT or EPUB publications, manga/image-sequence reading, source-provided or server-generated audio, and other content forms. These additions must not make the frontend interpret BookSource behavior, force unrelated providers through one oversized interface, or turn the Reader into one universal renderer full of format-specific branches.

The foundation must improve the current prose-and-image path now while allowing abstractions to emerge only when a second real provider or reading modality requires them.

## Decision

NovelReader separates four concerns:

1. A **Content Provider** obtains and interprets content from a particular origin, such as a BookSource, EPUB file, or TXT file.
2. A provider opens a **Reading Section** as a modality-specific **Reading Document**.
3. A document refers to binary material through opaque, reader-authorized **Content Resources**.
4. The frontend **Reading Session** delegates document presentation and location tracking to a modality-specific renderer.

```text
Content Provider
  → Reading Section
    → Reading Document
      → Content Resource

Reading Session
  → modality renderer
    → Reading Location
```

Provider kind, reading modality, container format, and resource kind remain separate concepts. EPUB, TXT, and BookSource describe origins or containers; prose, image sequence, and audio describe reading behavior.

### Reading documents

The reading interface uses a discriminated family rather than one universal media-block tree:

- **Prose Document** — flowing text with semantic prose blocks, initially paragraphs and inline images.
- **Image-Sequence Document** — ordered page images whose order, progression, fitting, preloading, and page location are primary reading behavior.
- **Audio Document** — ordered playable resources and audio location; introduced only when concrete audio work begins.

Inline illustrations and manga pages share the same image-resource delivery foundation, but they do not share one renderer or one location model. An inline image participates in prose flow; a manga page is a primary reading unit.

New document kinds and block kinds are added only for demonstrated behavior. Placeholder media types are not added in advance.

### Content resources

Documents expose only NovelReader-controlled opaque resource references. The frontend never receives or reconstructs upstream source URLs, local paths, archive paths, source credentials, request options, cookies, decoding scripts, or provider-native identifiers.

The backend resource path owns:

- Reader Account authorization and item/section association;
- provider-specific resource resolution;
- source sessions, headers, cookies, referers, and URL options;
- source-defined portable decoding;
- archive or local-file access when those providers exist;
- bounded time, size, media-type validation, and cache policy;
- streaming where the resource does not require whole-body transformation.

A resource endpoint must not become a general URL proxy. References are server-issued and resolve only within reader-owned publication state.

### Backend reading seam

Application-level reading orchestration belongs above provider-specific implementations. It owns reader authorization, shelf-item and section lookup, provider selection, normalized document construction, resource-reference issuance, cache fallback, stable errors, and reading-state coordination.

Provider implementations own provider-native retrieval and interpretation. BookSource Search, Explore, source switching, rules, sessions, and source interactions remain BookSource-specific capabilities rather than requirements of every future provider.

Provider interfaces are capability-specific, not one giant interface. Interfaces for sections, documents, or resources are extracted when a second real provider creates actual variation. Until then, current BookSource implementations remain concrete behind the normalized document/resource seam.

### Frontend reading seam

The Vue frontend depends on NovelReader reading-domain values and a cohesive HTTP adapter, not on provider behavior or endpoint construction spread through renderers.

The Reading Session owns common concerns:

- active shelf item and section;
- loading, failure, and route coordination;
- previous/next section navigation;
- common Reader chrome;
- source recovery when available;
- progress persistence coordination;
- renderer selection by document kind.

Each renderer owns modality-specific presentation and interaction:

- prose typography, paragraphs, inline images, selection, conversion, and prose progression;
- image-sequence flow, page progression, fitting, zoom, preloading, and page location;
- audio playback, buffering, seeking, playback rate, and time location.

Shared presentation vocabulary does not require shared implementation. For example, both prose and image sequences may support continuous or paged flow, while each renderer implements that behavior according to its modality.

### Reading locations

Reading progress is modality-specific:

- prose uses a section plus normalized progression or a later prose locator;
- image sequence uses a section plus page index and optional in-page progression;
- audio uses a section/track plus time offset.

The current prose progress schema remains authoritative until a second modality creates a real persistence requirement. Structured, versioned locations are introduced and migrated at that point rather than speculatively now.

### Shelf identity

Local imports remain separate shelf items from BookSource books by default. Provider-specific identity and storage remain provider-owned. Cross-provider edition linking is not part of this decision and may be designed later if readers need it.

## Rationale

This design shares infrastructure where the invariants are genuinely common—authorization, section opening, resource delivery, errors, and reading-session coordination—while preserving cohesion for behavior that differs substantially by modality.

It gives the frontend a stable provider-agnostic reading interface without pretending that prose, manga, and audio have the same layout, navigation, loading, or progress semantics. It also avoids speculative provider/plugin machinery: abstractions are introduced at the normalized document and resource seams now, and provider capability interfaces appear only with a second implementation.

## Alternatives

### One universal content-block tree and renderer

Rejected because manga pages and audio tracks are not merely additional inline blocks. Their layout, controls, loading, navigation, and locations would accumulate conditional behavior in one low-cohesion renderer.

### One complete reading stack per provider

Rejected because BookSource, EPUB, and TXT prose should reuse the same prose document and renderer, while all providers should reuse reader authorization and resource delivery. Separate stacks would duplicate behavior and couple presentation to origin.

### A full provider/plugin framework now

Rejected because BookSource is currently the only real provider. Registration frameworks, placeholder adapters, generic provider metadata, and speculative persistence would add interfaces without proven variation.

### Frontend content classification

Rejected because counting images or inspecting blocks is unreliable and would let clients disagree about reading behavior. Providers or backend normalization select an explicit document kind; conservative detection and a user override may be added later for genuinely ambiguous sources.

## Consequences

- The immediate content contract becomes explicitly prose-oriented instead of being presented as a universal chapter model.
- Inline images and future page images use one opaque resource concept.
- `ReaderView.vue` should become a common Reading Session shell and delegate prose presentation to a focused renderer before additional modalities are added.
- Manga adds an image-sequence document and renderer rather than branches throughout the prose renderer.
- Audio adds an audio document and renderer only from concrete requirements.
- TXT or EPUB introduces the first real provider abstraction and provider-neutral section workflow.
- Existing BookSource discovery, source binding, and source recovery remain provider-specific where appropriate.
- Current progress storage is not migrated during prose/image foundation work.

## Revisit When

Revisit this decision if:

- one real publication requires multiple simultaneous primary modalities that cannot be represented as modality-specific sections;
- cross-provider edition linking becomes a product requirement;
- frontend clients need offline document packages rather than authenticated resource retrieval;
- demonstrated fixed-layout EPUB behavior cannot fit the image-sequence or a separately justified fixed-layout document model;
- provider variation shows that the capability interfaces need a different seam.
