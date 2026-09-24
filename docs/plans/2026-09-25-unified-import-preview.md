---
status: active
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

Design selected; production implementation has not started. Existing owners inspected:

- `frontend/src/features/imports/TXTPreview.vue` currently displays paged headings and a truncated text sample. Both `ImportReviewView.vue` and `TXTReparseView.vue` use it; re-analysis supplies a separate heading-action slot.
- `EPUBSectionPreview.vue` owns on-demand navigation/section loading and anchor scrolling. Preserve those behaviors while replacing its selector with B's visible contents.
- `ImportReviewView.vue` and `EPUBReviewView.vue` currently hide editable title/author fields in disclosures below preview. Move their persistent presentation above preview without changing acceptance semantics.
- `backend/internal/txtstore/review.go` already enforces interpretation role/generation before and after reading saved section bytes, then truncates samples to 4096 bytes. Reuse this ownership logic for whole-section reads rather than introducing a second analysis path.
- `ProseRenderer.vue` centers figure-contained images but ordinary structured images lack equivalent centering. The production browser baseline still needs confirmation; centering all images is an explicit user preference regardless.

## Next Action

1. Inspect existing TXT API handlers and focused tests; add a narrow full-section read for import and re-analysis using the current saved-generation authorization rules.
2. Implement the shared B presentation and connect format-owned loaders. Keep ordinary navigation independent of mutation/resume controls.
3. Make metadata persistent and center images through the existing prose renderer; update relevant translations.
4. Run focused backend/frontend checks and an isolated desktop/mobile browser journey. Update usage/architecture only for implemented behavior, then commit the cohesive feature.

## Verification

Prototype-only checks already passed: JavaScript syntax, desktop/mobile navigation, disabled Previous at section zero, persistent B author, no stage horizontal overflow and separate mock resume selection. These are not production verification.

Production checks still required:

- Synthetic TXT chapter longer than 4 KiB is returned in full; stale/discarded or applied candidate cannot be previewed under its old ownership, and reader isolation remains enforced.
- TXT/EPUB section navigation, stale-request cancellation and re-analysis resume separation.
- Frontend typecheck, affected tests and production build; existing published-reader/EPUB checks where touched.
- Browser check of desktop/mobile layout and centered ordinary/inline images in preview and reader, using isolated data. No live reader-home mutation.

## Compatibility and rollback

Keep existing summary endpoints and acceptance/re-analysis contracts compatible. No stored data change is expected. A frontend layout/renderer change and additive TXT section-read routes can be reverted without migrating reader homes. If implementation requires a durable-data change, stop and explain before expanding scope.
