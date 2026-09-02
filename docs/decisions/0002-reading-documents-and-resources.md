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

## Future Provider Adoption Reference

Use this section when the first non-BookSource provider—most likely local TXT or EPUB import—becomes accepted implementation work. It defines the extension path, not interface signatures or storage schemas; those must be designed from the concrete provider requirements available at that time.

### Start the workstream

1. Create a dedicated implementation plan for the provider. Do not reopen the completed reading-foundation plan.
2. Revalidate this decision against the actual format and product requirements before changing shared models.
3. Keep each imported publication as its own shelf item unless a separate decision introduces cross-provider edition linking.
4. Identify the provider capabilities that genuinely differ from BookSource before extracting interfaces. Prefer small capability-specific interfaces over a provider registry or one comprehensive provider interface.

### Provider and storage ownership

- Store provider-native identity and state in provider-owned storage. Do not add a growing set of nullable TXT-, EPUB-, archive-, or BookSource-specific columns to one shared record.
- Keep file paths, archive entry names, encoding details, parsing state, and provider-native identifiers behind the backend reading seam.
- Validate imported files at the import boundary and preserve enough provider-owned identity to reopen sections and resources deterministically.
- Application-level reading orchestration should select the provider, authorize the shelf item, open its section, normalize the result, issue resource references, and return stable reading errors. The HTTP layer and frontend renderer should not branch on local paths, archive formats, or BookSource rules.

### Sections and documents

- Map the provider's ordered divisions to Reading Sections. A BookSource chapter, EPUB spine item, and generated TXT section are provider-specific forms of the same cross-provider concept.
- Normalize plain TXT and reflowable EPUB sections into the existing Prose Document semantics whenever their reading behavior is flowing prose.
- Extend prose block kinds only for demonstrated semantic behavior needed by a real provider. Do not expose raw HTML or create format-named block kinds such as `epub-paragraph`.
- Do not infer the renderer from file extension in the frontend. The backend/provider returns an explicit document kind, and the Reading Session selects the renderer from that discriminator.
- Treat fixed-layout EPUB as a separate design question. Use an image-sequence or separately justified fixed-layout document only if its observed layout, navigation, and location behavior cannot be represented honestly as prose.

### Resources

- Convert EPUB images, fonts, stylesheets, and other permitted archive assets into NovelReader-controlled opaque Content Resource references. Never expose archive paths or local filesystem paths to the frontend.
- Resolve each reference only within its authorized shelf item and section/provider state. Resource delivery must not become a general local-file, archive-entry, or remote-URL proxy.
- Reuse common authorization, bounded reads, media-type validation, caching policy, and response safety where their invariants are truly shared; keep BookSource headers, cookies, sessions, request options, and decoding inside the BookSource adapter.
- Prefer streaming for large untransformed resources, but retain bounded whole-body handling where parsing or transformation requires it.

### Frontend and reading state

- Reuse the Prose Renderer for TXT and reflowable EPUB documents. Provider-specific import or parsing behavior must not enter that renderer.
- Keep the Reading Session responsible for common loading, section navigation, chrome, failures, and persistence coordination. Provider-only controls such as BookSource recovery should be capability-gated rather than made mandatory for every provider.
- Continue using the current prose location/progress behavior while the new provider still produces prose. A second provider alone does not justify modality-specific progress migration.
- Introduce structured, versioned Reading Locations only when a second reading modality supplies a real incompatible location shape, then migrate existing prose progress explicitly.

### Completion checks

Before considering the first provider integration complete, verify that:

- the provider can enumerate/open Reading Sections without the frontend knowing its native format;
- its flowing content reaches the existing Prose Renderer as a versioned Prose Document;
- any binary assets resolve through authorized opaque references;
- BookSource-specific behavior has not leaked into the provider-neutral reading interface;
- provider-specific identity and storage remain cohesive and independently maintainable;
- existing BookSource reading and progress continue to work;
- current architecture documentation reflects the concrete interfaces and storage chosen during implementation.

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
