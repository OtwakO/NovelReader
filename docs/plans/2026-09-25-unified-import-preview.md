---
status: completed
updated: 2026-09-25
---

# Unified TXT and EPUB preview

## Goal

Implement the user-selected B layout for import previews: visible contents, consistent reading-area boundaries, persistent book identity and arrow navigation. TXT must show the whole selected chapter, including in re-analysis, without changing its approval workflow. Center every displayed prose image in preview and reader for the user's manual evaluation.

## Scope

- Shared preview presentation for TXT import, EPUB import and TXT re-analysis.
- Whole-section TXT preview from saved analysis, with authenticated, generation-checked reads.
- Title and author remain visible rather than hidden in a disclosure. Preserve metadata editing in import review.
- All displayed prose images centered, including images originally inline.
- Relevant translations, focused tests and current documentation.

No schema migration, reset, reimport, deployment, progress writes from preview, new preparation pipeline, generic reader framework, cache/prefetch work, or unrelated R1–R3 cleanup.

## Accepted Approach

The user selected [prototype B](../../frontend/prototypes/import-preview/README.md), refined through commits `e2ad256` and `9136395`:

- Desktop contents column: 250px, with a bounded scrollable list.
- Mobile: contents above the reading area, not a horizontally squeezed sidebar.
- Title and author always visible; Previous/Next use left/right icons with accessible labels and disabled boundary states.
- A shared presentation component owns layout and controls, not format-specific fetching, authorization, admission or re-analysis state.
- Keep the existing working EPUB navigation/section/resource loading and generation checks. Preserve authored hierarchy, unavailable entries, section stepping and anchor handling.
- TXT loads one complete selected saved section on demand. Keep heading pagination bounded; do not fetch the whole book or fabricate publication revisions.
- Preserve lightweight admission summaries. Add the smallest TXT section-read boundary needed rather than making every summary fetch return full chapters.
- Re-analysis preview selection is not resume selection. Its explicit resume/apply controls and concurrency checks remain separate.

The prototype is visual evidence, not production code to copy verbatim. A/C remain comparison artifacts; B is the selected direction.

## Current State

Implemented and verified within the scope below. Current usage and architecture documents are updated. Owners:

- `BookPreview.vue` owns B's presentation only. `TXTPreview.vue` loads a complete selected chapter and a bounded 25-heading page; both import and re-analysis use it. The explicit resume action is below the selected preview, not triggered by browsing.
- `EPUBSectionPreview.vue` retains on-demand navigation/section loading and anchor scrolling while delegating visible contents and reading-area layout to `BookPreview.vue`.
- Import title/author fields are visible above preview without disclosures. Re-analysis status now includes its saved author as an additive response field.
- `backend/internal/txtstore/review.go` shares saved role/generation checks between bounded summaries and full-section reads. `txt_review_section.go` exposes additive generation-qualified import/re-analysis section endpoints.
- `ProseRenderer.vue` centers every image using block display and automatic inline margins, confirmed for ordinary cover and inline images in an isolated browser.
- The contents list keeps the selected entry visible when navigation crosses a page or moves beyond the list viewport, without scrolling the surrounding page.

## Next Action

Gather manual feedback on the implemented B layout and all-image centering. No implementation remains pending in this scope. Keep unrelated R1–R3 cleanup separate.

## Verification

- `go test ./internal/txtstore ./internal/api -count=1` passed. The new API test verifies a full synthetic chapter longer than 4 KiB, unchanged bounded summary, authentication, cross-reader isolation and stale-generation rejection.
- `go test -race ./internal/api -run 'TestTXT(SelectedSectionPreview|ReparseHTTPReviewApplyAndDiscard|ReadingThroughCommonHTTPRoutes)$' -count=1` passed after final test changes, including applied/discarded candidate invalidation and published-reading compatibility.
- Direct-Node `vue-tsc --noEmit`, 56 focused Vitest tests across imports and `ProseRenderer.test.ts`, and Vite production build passed. Vitest used `NODE_OPTIONS=--no-experimental-webstorage` for the existing Node 25 localStorage conflict; existing `/explore` fixture-route warnings remain outside scope.
- Isolated Chromium with synthetic TXT/EPUB: whole TXT chapter, contents-page crossing, 250px desktop sidebar, visible editable metadata, EPUB authored navigation and cover/inline image centering, mobile bounds, and separate re-analysis resume choice passed. Published-reader inline image centering also passed. The final rebuilt UI kept the selected contents entry visible on mobile after crossing back to the preceding page.
- Selected desktop/mobile screenshots were visually inspected; ignored evidence is under `reference/unified-preview/`. Test books were added only to a fresh temporary reader home; no existing data, schema or deployment changed.

Limits: no broad real-book compatibility audit, full repository frontend suite, hosted CI or deployment verification. Prototype mock coverage is not counted as production proof.

## Compatibility and rollback

Keep existing summary endpoints and acceptance/re-analysis contracts compatible. No stored data change is expected. A frontend layout/renderer change and additive TXT section-read routes can be reverted without migrating reader homes. If implementation requires a durable-data change, stop and explain before expanding scope.
