# Import user-testing issues and branch review

## Status and scope

Follow-up is authorized in small, focused increments. After root-cause diagnosis and code-based approach analysis, the user accepted a selected-section EPUB preview and one shared device-local image-optimization preference. U1/U2 are implemented and verified within the limits recorded in the [completed EPUB import preview plan](../plans/2026-09-24-epub-import-preview.md). U3/U4 are implemented and verified in the [completed mixed-format history/filter plan](../plans/2026-09-24-import-history-filters.md). R1–R3 are also resolved, with focused verification recorded below. No schema change is approved. No subagents were used.

Branch reviewed: `feat/multi-provider-library`, HEAD `e68fbb0`; comparison with `main` merge-base `070246995b92a9dc597e6a56383a8f30479aebc1`. The quick review sampled shared library/reading, TXT/EPUB intake, lifecycle coordination, persistence, backup/restore and frontend state across a 496-file branch diff. It was not an exhaustive audit. The completed [EPUB milestone](../plans/2026-09-17-epub-support.md) remains historical verification, not proof that subsequent real-user cases work.

## User-reported issues and requested improvements

### U1 — EPUB image optimization preference is not persisted

- Report: **最佳化 EPUB 圖片** resets across page navigation or refresh, unlike **加入書架前先確認**.
- Original code evidence: `frontend/src/features/imports/ImportQueuePanel.vue` owned local `optimizeImages: false`; `ImportInboxPanel.vue` separately owned local `optimize: false`. The controls did not use the persisted import-preference owner.
- Desired outcome: retain the user's image-mode preference across navigation and refresh. Browser/inbox consistency should be considered together. Already queued imports must keep their captured image mode rather than change retroactively.
- Accepted direction and implementation status: [EPUB import preview plan](../plans/2026-09-24-epub-import-preview.md#image-preference).

### U2 — EPUB preview shows no titles, chapter contents or images

- Report: EPUB review preview shows no ToC titles, chapter contents, or images.
- Relevant path: `frontend/src/features/imports/EPUBReviewView.vue`, `frontend/src/api/epub-imports.ts`, `backend/internal/epubstore/review.go`, and the EPUB preview HTTP handler.
- Original limitation: the review template was designed to render a paged **section-heading inventory** and a bounded **plain-text sample**, not a full hierarchical publication ToC, selectable chapter reader, or image preview. It had no image-rendering path. Chapter-click preview was previously deferred (see the completed import-history handoff linked from `PLAN.md`).
- The installed private EPUB reproduces missing titles and the initial empty sample through fresh acquisition → preparation → `epubstore.Review`, using a temporary reader home. This is backend-boundary evidence, not a reproduction against the user's existing receipt or a new browser check.
- **Title cause:** `epub/normalize.go` derives section titles solely from XHTML `head/title`. The inspected source sections have empty title elements; sections 1 and 25 have body headings. All 1,737 saved section titles are empty. Authored NCX navigation labels and targets are separately retained, but `epubstore.Review` returns the section-title inventory, not that navigation. At diagnosis, `EPUBReviewView.vue` displayed those empty strings without a fallback. There is no evidence of titles being lost in storage or frontend transport.
- **Sample cause:** `Review` samples only the section at the page's `start` index. Section 0 contains one image and no text; its plain-text projection is therefore empty. Explicit review starting at section 1 returns 301 sample bytes, and section 25 returns 4,094 bytes with truncation. Text preparation is working in those sections; the initial selection does not provide a useful prose sample for this book.
- **Image absence:** the prepared first section retains its image binding. At diagnosis, the review DTO/template supported only a text sample, not image resources; image absence is not evidence of failed image preparation.
- These findings separate title presentation, sample selection and richer preview scope. The accepted remedy is the [selected-section preview](../plans/2026-09-24-epub-import-preview.md), using saved authored navigation and structured content with explicit import-authorized resource access. Persisted titles/revisions remain unchanged.
- No real EPUB needs to be committed for investigation. Keep any supplied private fixture local/ignored.

### U3 — Received-files format selector needs All

- Request: **已接收檔案 → 格式** should offer **All / 全部**, in addition to TXT and EPUB.
- Original code evidence: `frontend/src/features/imports/ImportReceiptsPanel.vue` had separate TXT/EPUB choices and called their separate paginated receipt-list endpoints.
- Assessment: a reasonable mixed-import UX improvement, not evidence that storage formats should merge. Future implementation must define coherent ordering, pagination and filtering across formats; merely concatenating two independently paginated pages would not establish those semantics.
- Implemented: All formats by default, newest imports first with coherent pagination; verification in the [history/filter plan](../plans/2026-09-24-import-history-filters.md).

### U4 — EPUB received-files view lacks a status filter

- Request/question: EPUB should have a **狀態** filter like TXT unless its absence is intentionally necessary.
- Original code evidence: `ImportReceiptsPanel.vue` displayed the status selector only for TXT (`v-if="format === 'txt'"`); EPUB listing received a cursor without a status filter. `import-format.ts` already projects EPUB acquisition/preparation/publication into shared UI states.
- Assessment: EPUB has meaningful pending, ready, failed, published and removing conditions. There is no demonstrated domain reason it cannot benefit from a status filter. The original absence was an implementation asymmetry, not a verified intentional requirement. Do not assume all TXT labels or the TXT `needs_review` state map directly to stored EPUB states.
- Implemented: shared lifecycle filters, including plain “Needs review” for both formats under existing content-review rules; verification in the completed history/filter plan above.

## Subsequent manual feedback — preview consistency and image alignment

The user reports that the implemented EPUB preview works in rudimentary manual testing, but images appear uncentered in preview/reader and preview boundaries/separators need improvement. This is follow-up feedback, not a reversal of U1–U4 completion.

Accepted intent: center **all** displayed images for manual evaluation; add whole-selected-chapter TXT previews for import and re-analysis where the existing shared preview owner makes reuse practical. Keep preview selection separate from re-analysis resume selection/apply, and preserve format-owned loading and authorization. The old TXT summary returned a 4 KiB sample; whole-section preview now uses a separate saved-generation read, leaving the summary compatible. No new preparation pipeline or generic reader framework is requested.

After reviewing the disposable import-preview prototypes (preserved in Git history at `9136395`), the user selected refined B. The prototypes were removed after implementation. The completed [unified import-preview plan](../plans/2026-09-25-unified-import-preview.md) records implementation and scoped verification. TXT import/re-analysis share B's presentation with EPUB, metadata stays visible, and ordinary/inline images are centered in preview and reader. Focused tests and isolated synthetic desktop/mobile checks passed; broader manual real-book feedback remains useful.

## Review findings

### R1 — Inbox refresh notification can be dropped while busy (resolved)

Original finding: `ImportInboxPanel.vue` watched `queue.revision` and requested a scan; `import-task.ts` refused a task while busy, with no retained pending refresh. Completion during a scan/review can leave the inbox stale until manual refresh. `ImportReceiptsPanel.vue` already handles revision changes during its load loop.

Reproduced with deferred scan and review responses, then fixed with one panel-owned pending refresh. A subsequent load consumes the pending notification; notifications during that load request another pass. Explicit review/resolution is never interrupted, proof remains intact, and failed actions retain their error until an explicit retry rather than being hidden by automatic refresh. The shared task guard is unchanged.

Verification: 23 tests in `import-views.test.ts` and `import-image-preference.test.ts`, plus frontend typecheck, passed. Regression coverage includes coalescing during a scan, refresh after review, and preserving resolution errors.

### R2 — Shared reader orchestration is unnecessarily compressed (resolved)

Original finding: `frontend/src/features/reader/ReaderView.vue`, especially mounting/loading, conversion and content-commit methods, packed asynchronous operations and state changes into dense lines. The ordering of navigation, displayed content, scroll and progress is hard to review safely.

Resolved by expanding the script's compressed statements, state fields and blocks using normal formatting. No extraction, ownership change or new abstraction was needed. The TypeScript AST was compared before/after (ignoring redundant parentheses); template and styles are unchanged. Frontend typecheck and 76 tests across reader features plus `ImportWorkspace.test.ts` passed. The existing fixture warnings remain for R3; no new reader behavior is claimed.

### R3 — Passing frontend tests emit avoidable warnings (resolved)

The reviewed run emitted missing i18n keys, unresolved `RouterLink`, and missing test-router routes. Examples include `ReaderSettingsSheet.test.ts`, `ReaderBookmarksSheet.test.ts`, `TocNavigationList.test.ts`, `TocChapterList.test.ts`, `ReaderView.test.ts`, and `ImportWorkspace.test.ts`.

Reproduced in the scoped run and corrected at the fixture boundary: reader tests now load the existing English message modules into test-local i18n instances, button-only ToC tests register `RouterLinkStub`, and the import-workspace router includes `/explore`. No global warning suppression or production translation/router changes.

Verification: 126 tests across the reader and import directories (29 files) passed without warnings, followed by frontend typecheck and production build. Runs use `NODE_OPTIONS=--no-experimental-webstorage` for the local Node 25 test environment. No full repository suite, new browser journey, backend run or deployment is claimed for R1–R3.

## Overall assessment and optimization limits

The sampled architecture is practical: a small shared reading interface with real provider variants; one bounded TXT/EPUB scheduler and admission budget; format-owned transactions, generation checks, publication and removal; shared filesystem mechanics without a generic receipt state machine; library-owned progress/bookmarks; and authenticated revision-qualified resource delivery. No confirmed data-loss/security blocker or broad architectural rewrite was identified in this limited review. This is not a guarantee of defect-free behavior.

No measured performance defect justified caching, more workers, or speculative abstractions. `backend/internal/epubstore/catalog.go` performs one resource query per section image: a possible image-heavy optimization target, not a proven bottleneck. No evidence supported bulk test deletion as over-testing; persistence/cancellation/stale-generation coverage addresses real risks. Fixture/output quality is the concrete testing improvement.

## Verification recorded from the review

- Passed from `backend/`: `GOPATH=/tmp/novelreader-webp-go go test ./internal/library ./internal/reading ./internal/fileimport ./internal/txtstore ./internal/epubstore ./internal/inboxfiles ./internal/readerstore ./internal/backup ./internal/api ./cmd/server -count=1` (11 packages).
- Passed from `frontend/`: `npm run typecheck` and `npm test -- src/features/imports src/features/reader src/features/backups src/api src/features/books src/features/shelf --reporter=dot` (194 tests, 49 files; warnings above).
- Passed: `git diff --check`.
- AFT diagnostics failed at the transport layer; not a clean diagnostics result.
- No new full-repository suite, race run, or browser reproduction was performed during that review. None of these passing checks reproduces U2 or verifies the requested U1/U3/U4 changes.

## Preview diagnosis verification

The ignored local harness is `reference/epub-preview-diagnosis/review_test.go`, injected with a Go overlay so no diagnostic test or private fixture is added to the default suite. From `backend/`, set `NOVELREADER_EPUB_FIXTURE` to the absolute path of the installed private EPUB and run:

```sh
go test -overlay ../reference/epub-preview-diagnosis/overlay.json ./internal/epubstore -run '^TestLocalPreviewDiagnosis$' -count=1 -v
```

With that environment variable set, the diagnostic intentionally fails assertions for nonempty titles at starts 0, 1 and 25, and for a nonempty sample at start 0. It logs counts/booleans only, not book text, image bytes or archive member paths. Read-only ZIP/XML inspection independently confirmed empty source title elements, body headings in sections 1/25 and the image-only initial section. An initial harness invocation used an invalid receipt ID and failed before preparation; the corrected harness uses the existing generated-ID convention. No production code, original EPUB or existing reader data was changed. No frontend/browser verification was run during diagnosis.

## Next action

U1–U4 are complete within the verification limits of the linked preview and history/filter plans. Preview-consistency follow-up is implemented within the completed B plan's verification limits above. R1–R3 are resolved. Gather further manual usability feedback; scope any new findings separately. No implementation remains pending from this note. This note retains original reports and diagnosis evidence; the original preview change did not include U3/U4 or R1–R3.
