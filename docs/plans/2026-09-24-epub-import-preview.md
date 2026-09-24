---
status: completed
updated: 2026-09-24
---

# EPUB import preview and image preference

## Goal

Before shelf admission, let a reader inspect an EPUB's saved contents navigation and a selected section with text and illustrations. Reuse prepared content and the existing prose renderer without creating a second reading session. Remember one device-local image-optimization choice across browser and server-folder intake.

Done means existing ready imports work without reimport, the image-only opening section renders rather than appearing empty, authored contents labels remain meaningful even when XHTML titles are empty, and preview never writes reading progress or publishes a book.

## Scope

- U2: authored contents (or an explicitly labelled section-list fallback), selected-section structured preview and authenticated prepared-image access.
- U1: shared browser-local image preference through the existing import-preference owner; original images remain the default and queued selections retain their captured mode.
- Preserve metadata editing, diagnostics, encoder notices, retry/discard and explicit/automatic admission behavior.
- Exclude U3/U4 mixed-history filters, R1 inbox refresh, unrelated R2/R3 cleanup, reader cache/prefetch work, and TXT preview expansion. Their status remains in the [issue note](../notes/2026-09-21-import-user-testing-and-branch-review.md).
- No progress, bookmarks, persistent preview state/cache, speculative prefetch, nested note-return stack, full ReaderView embedding or publisher HTML/CSS rendering. Internal prose links remain visibly inactive initially; contents selection supports section/anchor navigation.

## Accepted Approach

### Content and ownership

- Keep `epubstore` responsible for ready-generation metadata, indexed section reads, resource bindings and bounded original/derivative file reads. Do not reparse archives for preview or expose native paths.
- Reuse saved authored navigation, preserving its hierarchy and section/anchor targets. Navigation labels are not one title per section; do not copy them into persisted section titles. For unavailable authored navigation, retain the section-list fallback and ensure unnamed entries have a readable section-number label.
- Keep the initial section, including a cover, as legitimate content. Contents selection loads one section; do not scan the publication to manufacture a nonempty text sample. Provide a way to move through sections even when authored contents omits front matter.
- Reuse `ProseRenderer.vue` and the existing safe prose projection/parser. Separate reusable presentation from the published-reading envelope only where this second real caller requires it. Preparation generation must not masquerade as library `contentRevision`.
- Preview selection, loading/error state and scoped anchor positioning belong to the import review. Reuse existing task cancellation and discard superseded results; do not import the reader's progress/session machinery.

### API and authorization

- Retain the existing review-summary contract: `import-format.ts` uses it for metadata, diagnostics and automatic/bulk admission. Rich content must be requested separately and only while previewing, not during receipt polling or automatic addition.
- Preview content/resources are qualified by reader ownership, receipt and current ready preparation generation. API handlers retain the authenticated reader-home lease. Reuse current scoped resource-URL and no-store conventions rather than inventing another token system.
- Keep published reading and import preview as explicit authorization entry points. Share bounded resource-reading mechanics beneath them; never add a bypass flag to the published-book endpoint.
- Enforce current ready identity around file reads, following existing store conventions. Wrong reader, superseded generation, discard/removal and missing resources must not expose another import's bytes. Acceptance of the same immutable ready generation need not invalidate review evidence.
- No new scheduler, provider registry, generic identity framework or global preview store. Use named functions and cohesive components; extract only code actually shared by reading and preview.

### Image preference

Use the existing browser-local import-preference owner for one saved choice shared by both controls. Snapshot it at selection, just as the queue already captures image mode. Changing the preference must not alter queued or acquired imports. Do not move this device preference into server-owned Reader Data or portable backups.

## Decisions and Tradeoffs

The accepted selected-section approach addresses contents, text and illustrations together. A label/plain-text-sample correction would be smaller, but would retain the image-only-cover mismatch and cannot preview illustrations. Mounting the full ReaderView would reuse more orchestration while introducing publication/progress dependencies that imports do not need.

The real cost is a narrow unpublished content/resource access boundary and separating presentation reuse from published identity. This is structural/auth-sensitive work at those boundaries, not justification for a repo-wide refactor or exhaustive testing.

Existing ready generations already contain navigation, semantic sections and image bindings. No persisted-title rewrite, preparation rerun, schema epoch change, migration or data reset is planned. If implementation reveals such a requirement, stop and explain it before changing durable data.

## Implementation Steps

1. **Prepared preview boundary:** define the small import-qualified content/navigation contract; reuse storage reads and prose projection, add authorized resource delivery, and protect existing publication access and admission-summary behavior.
2. **Review UI:** connect saved contents and selected-section loading to the shared renderer, including anchors, ordinary section selection, initial cover and local loading/failure feedback. Keep internal prose links inactive and preview free of reading writes.
3. **Shared image preference:** wire both controls to the existing saved preference owner without changing captured queue options. This is independently deliverable.
4. **Verify and hand off:** run focused boundary/component checks and one browser journey; update current architecture/usage documentation only after behavior lands.

Each increment should be a complete, working commit. Adjust ordering to actual dependencies; do not split layers into artificial half-working milestones. Update Current State, Next Action and Verification at meaningful stopping points.

## Current State

Completed: authenticated navigation/section/resource preview routes, shared bounded storage reads and prose projection/parser, a review-owned section preview component, and the shared device-local image preference. No stored-data/schema change, reimport or published-reader authorization change. Current usage and architecture docs are updated; U3/U4 and R1–R3 remain separate. The root-cause reproduction and original review evidence remain canonical in [U2 and preview diagnosis](../notes/2026-09-21-import-user-testing-and-branch-review.md#u2--epub-preview-shows-no-titles-chapter-contents-or-images).

The implementation retains these ownership boundaries:
- `backend/internal/epubstore/{review,prepared_read,catalog,resource_read}.go`: ready-generation reads exist; published resource/section reads currently require a library binding.
- `backend/internal/reading/{epub_document,epub_catalog}.go`: safe projections exist but emit published revision-qualified envelopes/targets.
- `backend/internal/api/{epub_receipts,epub_preview_response,epub_resources}.go`: current summary and published resource boundaries.
- `frontend/src/api/{structured-prose,catalog-navigation,reading-target}.ts`: strict parsers currently qualify internal targets with content revision.
- `frontend/src/features/reader/{ProseRenderer.vue,TocNavigationList.vue}`: renderer has no progress owner; contents presentation has a button mode, but its target type still assumes reading identity.
- `frontend/src/features/imports/{EPUBReviewView.vue,import-format.ts}`: review owns preview state; automatic/bulk addition also consumes the summary endpoint.

`epubstore/preview.go` owns ready-generation access; `api/epub_review_content.go` owns HTTP DTOs/resources. `epub-review.ts` validates preview identity and uses the shared prose-document parser without a publication revision. `EPUBSectionPreview.vue` owns only transient preview state. The original summary endpoint and automatic-admission caller are unchanged.

## Next Action

No remaining implementation in this workstream. Any expanded internal-link navigation or receipt-filter work requires its own scoped decision. Preview uses a generation-qualified envelope with section/anchor navigation targets; prose documents contain no published targets. A native contents selector preserves hierarchy through indented entries, with Previous/Next for sections omitted from contents; no generic navigation component was introduced.

## Verification

Completed: root-cause diagnosis and direct source inspection; the new synthetic API regression failed on the absent route before implementation and now passes. `go test ./internal/epubstore ./internal/reading ./internal/api -count=1` and the same three packages with `-race` pass. Frontend typecheck, production Vite build, and 59 tests across imports, EPUB review wire parsing, structured prose and ProseRenderer pass. Local dependency launchers lack execute permissions, so tools were invoked through Node; tests additionally use `NODE_OPTIONS=--no-experimental-webstorage` because Node 25 native storage conflicts with jsdom. Existing ImportWorkspace missing `/explore` test-route warnings remain unrelated. An isolated Chromium journey with the ignored private EPUB verified browser upload/preparation, cover-image delivery, populated authored contents, selected prose, Previous/Next, ready-receipt reopening without reimport, and image-preference persistence across reload. Network checks found no preview admission/progress/bookmark writes and the library remained empty. Desktop (1280×900) and mobile (390×844) inspection confirmed the final layout without page overflow. Screenshots remain under ignored `reference/epub-preview-diagnosis/`; no private content is tracked. The diagnostic's expectation of nonempty legacy section titles/text is not the accepted new preview contract and must not drive production special cases.

Coverage and limits:
- `epub_review_content_test.go` covers a synthetic publication through real preparation/authenticated preview: empty XHTML titles, authored labels/anchor targets, an image-only cover, prose, safe internal-link handling and no private archive paths.
- Integration checks cover anonymous/wrong-reader requests, stale generation and access after discard. Existing published-reading and automatic-admission tests still pass.
- Frontend tests cover selection, fallback labels, literal prose rendering, late-response cancellation on receipt replacement, wire generation/target validation, shared preference controls, persistence and unchanged queued image modes. Existing automatic/bulk-admission coverage remains intact.
- Exact local frontend commands: `node node_modules/vue-tsc/bin/vue-tsc.js --noEmit`; `NODE_OPTIONS=--no-experimental-webstorage node node_modules/vitest/vitest.mjs run src/features/imports src/api/epub-review.test.ts src/api/structured-prose.test.ts src/features/reader/ProseRenderer.test.ts --reporter=dot`; `node node_modules/vite/bin/vite.js build --logLevel warn`. UI mechanical detection reported no findings.
- Keep real EPUBs, extracted content and raw diagnostic artifacts ignored. Default tests use synthetic fixtures and require no private books or live sites.

No deployment, full-repository suite, broad private-book compatibility campaign, stress testing or performance claim. Browser verification used original images; optimized-image reading retains shared existing storage coverage. Fresh temporary reader storage was used; existing reader data was untouched.

## Compatibility and Rollback

Preserve the current summary endpoint and published-reading semantics; new import preview operations must not relax them. Reuse existing prepared data without conversion, so rollback should require only reverting application changes, not restoring or downgrading reader homes. The device preference is additive browser-local state; verify existing saved preferences retain their meaning. No deployment, live-data reset or existing-home migration is authorized by this plan.

`README.md`, current import/reading architecture, U1/U2 and `PLAN.md` reflect completion. The earlier completed EPUB milestone plan remains frozen history.
