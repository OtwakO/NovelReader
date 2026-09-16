# EPUB reader references: reflow, semantics and performance

Supports the [EPUB plan](../plans/2026-09-17-epub-support.md). Research only; no reader or parser implementation changes.

## Conclusion

Preserving original publisher layout is not required for NovelReader. Preserve meaningful structure and navigation, then let the existing reader own typography and responsive layout. Reflowable EPUB is already designed to wrap across viewport sizes; preserving publisher CSS is possible on mobile but adds styling conflicts and compatibility work. Fixed-layout EPUB instead preserves a page canvas and often needs fit/zoom/pan; it remains outside this milestone.

A good Readest-like reading experience does not require adopting its full renderer. Borrow the product qualities (quiet chrome, comfortable measure, accessible typography controls, contextual navigation); retain NovelReader's existing single reading/session owner. Readest/Foliate JS and Readium Go are different projects with different roles.

## Provenance and limits

- Local `reference/legado-E` at `8b87c5aba4df91c39a3a0939a68a1180b9f2ee1c`; inspected files have whitespace-only local differences under the agent's `--ignore-space-at-eol` comparison. Paths below refer to that local snapshot.
- `reference/web-legado` contains a packaged `reader.jar` and prior research notes, not a pinned upstream source checkout. Its observations below are explicitly secondary local evidence; no new binary decompilation/runtime verification was performed.
- [Readest](https://github.com/readest/readest/tree/95117b03fa1f327f09ae67aca2b5da6ba9df4ff1) at `95117b03fa1f327f09ae67aca2b5da6ba9df4ff1`: inspected source and its checked-in desktop footnote screenshot. No app execution, mobile screenshot verification or comparative benchmark was performed. Source comments' timing estimates are not our measurements.
- These are design references, not copied implementations. Readest identifies its application as AGPL-licensed; review applicable licenses before any source reuse. No reference source was copied into NovelReader.

## What the references actually do

### legado-E: app-owned prose with lazy archive access

`app/src/main/java/io/legado/app/model/localBook/EpubFile.kt`:

- `109–117`: opens the ZIP and calls `readEpubLazy`, rather than extracting the entire archive on this path.
- `getContent` / `getBody`, `163–274`: uses Jsoup, strips scripts/styles, optionally removes headings/ruby, resolves image references and calls `HtmlFormatter.formatKeepImg`.
- `277–299`: image resources are streamed separately; cover gets its own saved representation.
- `335–455`: TOC-first chapter discovery with spine fallback; records fragment start/end and cross-file continuation. `240–254` slices serialized HTML and reparses it.

`app/src/main/java/io/legado/app/utils/HtmlFormatter.kt:12–69` converts block boundaries to newlines and discards most tags except normalized images. This is materially more lossy than NovelReader's selected semantic profile.

`app/src/main/java/io/legado/app/help/book/BookHelp.kt:400–418` reads cached chapter text first and stores normalized EPUB content on a miss. `modules/book/src/main/java/me/ag2s/epublib/domain/LazyResource.java:66–120` streams uninitialized resources but can retain fully read bytes until close.

**Borrow:** separate metadata/navigation from lazy binary resources; avoid repeatedly normalizing a chapter. **Do not copy:** regex-based semantic flattening, serialized-fragment slicing, filename-specific cover logic or TOC-as-reading-order assumptions. A shared document can have several TOC anchors without becoming duplicated reading content.

### web-legado: separate original-content and parsed modes

`reference/web-legado/docs/live-user-flow-observations.md:388–410` records an EPUB iframe-versus-parsed setting, font/line-height/paragraph-spacing controls, adaptive/mobile mode and desktop width. This establishes a recorded UI distinction, not identical implementation to Android Legado or any speed advantage.

`reference/web-legado/docs/backend-architecture.md:287–304` reports import preview and archive extraction, and `24–27` reports `/epub/*` resource serving. These are prior binary research findings, not independently reverified source here.

**Borrow:** user-owned reading presentation. **Do not copy:** two rendering modes unless there is a real publisher-fidelity requirement. An extracted resource tree and separate iframe mode would add lifecycle/authorization work that the current milestone does not need.

### Readest: a richer reflow engine plus a restrained reading surface

Paths below are relative to `apps/readest-app/src/` at the pinned revision:

- [`app/reader/components/FoliateViewer.tsx:705–777`](https://github.com/readest/readest/blob/95117b03fa1f327f09ae67aca2b5da6ba9df4ff1/apps/readest-app/src/app/reader/components/FoliateViewer.tsx#L705): dynamically loads `foliate-js/view.js`, opens the publication, handles fixed-layout separately, applies user styles and uses inset-adjusted viewport dimensions. This is a browser publication-rendering stack, not a Go parser.
- [`utils/style.ts:382–454`](https://github.com/readest/readest/blob/95117b03fa1f327f09ae67aca2b5da6ba9df4ff1/apps/readest-app/src/utils/style.ts#L382): normalizes page spacing, handles image dimensions and scrollable table containers. The wider file also transforms publisher styles; NovelReader's normalization avoids inheriting that compatibility surface.
- [`app/reader/components/footerbar/FontLayoutPanel.tsx:54–89`](https://github.com/readest/readest/blob/95117b03fa1f327f09ae67aca2b5da6ba9df4ff1/apps/readest-app/src/app/reader/components/footerbar/FontLayoutPanel.tsx#L54): explicit font size, margins and line-height controls. `app/reader/utils/mobileLayout.ts` makes the mobile control layout a shared decision, including portrait tablets.
- [`libs/document.ts:313–401`](https://github.com/readest/readest/blob/95117b03fa1f327f09ae67aca2b5da6ba9df4ff1/apps/readest-app/src/libs/document.ts#L313): ZIP entry lookup, lazy text/blob access, metadata-text reuse and concurrent text-load deduplication. Settled promises are dropped deliberately to avoid retaining an inflated whole book. This is source evidence of a bounded-lifetime strategy, not a benchmark of total application memory.
- [`data/screenshots/footnote_popover.png`](https://github.com/readest/readest/blob/95117b03fa1f327f09ae67aca2b5da6ba9df4ff1/data/screenshots/footnote_popover.png): inspected desktop image shows unobtrusive chrome, separated TOC, clear active location and contextual note reading. Its two-column spread is not a mobile layout requirement or a proposal to change NovelReader's reading mode.

**Borrow:** reading-area priority, consistent responsive control placement, locally accessible notes and avoiding duplicate work. **Do not copy:** the full feature surface, iframe event machinery, publisher-style rewriting or every retained-cache policy. Prefer the current note-navigation/return implementation proposal initially; popovers can be a later focused UX improvement.

## Consequences for NovelReader

1. Normalize once during preparation, one section at a time; persist compact semantic documents and anchor mappings. Reject or warn according to an explicit support profile, not silently erase meaningful content.
2. Ordinary reads load the requested prepared section, not the entire book or a fresh XML/HTML parse. Images remain separate, lazy, size-bounded resources; hiding images must avoid their requests. Carry validated dimensions when available to reduce layout shift and inaccurate resume positioning.
3. Reuse `frontend/src/features/reader/chapter-loader.ts`: it already deduplicates pending requests, retains five recent chapters and allows one speculative load. Do not add an EPUB-only cache/prefetch manager. Count-bounded caching alone is insufficient for unusually large EPUB sections: checkpoint 1 must determine whether section limits suffice or a byte-weighted bound is warranted.
4. Render only active reading content, not a whole-book DOM. Do not add global pagination, eager whole-book image decoding, unlimited caches or arbitrary chapter splitting to claim performance. Large single-spine documents are a real input shape: include one in the proof, then choose an explicit size policy or bounded segmentation only if evidence requires it.
5. Keep browser presentation local: font/spacing changes should reflow existing semantic content without reimport or server normalization. Phone layout uses a single fluid column; desktop uses a comfortable maximum measure. Preserve the existing reading mode and controls rather than making an unrelated reader redesign part of EPUB support.
6. Measure import wall time/peak memory and prepared storage size on a normal synthetic book and a large-section/image-heavy case; measure cold/warm first-section load, payload size and image-induced layout movement. Record hardware/input sizes. Use one repeatable focused check, not a benchmark framework or unsupported claims that one reference is faster.
7. Evaluate Readium's parsing reuse independently of rendering fidelity. Dropping publisher CSS does not eliminate OPF/spine/nav/URL rules; neither does it justify dragging in rendering/services we do not use. Parser adoption requires both a small maintainable integration and acceptable measured resource costs.
