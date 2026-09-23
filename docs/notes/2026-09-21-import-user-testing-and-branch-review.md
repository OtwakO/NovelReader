# Import user-testing issues and branch review

## Status and scope

Follow-up is now authorized in small, focused increments. The user accepted one device-local EPUB image-optimization preference shared by browser and server-folder intake, preserving queued choices. EPUB preview work is diagnosis-first: understand the root cause before selecting a remedy or expanding the preview contract. Mixed-format history/filter contracts remain unsettled. No schema change or expanded preview contract has been approved. No subagents were used.

Branch reviewed: `feat/multi-provider-library`, HEAD `e68fbb0`; comparison with `main` merge-base `070246995b92a9dc597e6a56383a8f30479aebc1`. The quick review sampled shared library/reading, TXT/EPUB intake, lifecycle coordination, persistence, backup/restore and frontend state across a 496-file branch diff. It was not an exhaustive audit. The completed [EPUB milestone](../plans/2026-09-17-epub-support.md) remains historical verification, not proof that subsequent real-user cases work.

## User-reported issues and requested improvements

### U1 — EPUB image optimization preference is not persisted

- Report: **最佳化 EPUB 圖片** resets across page navigation or refresh, unlike **加入書架前先確認**.
- Code evidence: `frontend/src/features/imports/ImportQueuePanel.vue` owns local `optimizeImages: false`; `ImportInboxPanel.vue` separately owns local `optimize: false`. These controls are not using the persisted import-preference owner.
- Desired outcome: retain the user's image-mode preference across navigation and refresh. Browser/inbox consistency should be considered together. Already queued imports must keep their captured image mode rather than change retroactively.
- Accepted direction: one device-local preference shared by browser and server-folder controls, using the existing import-preference owner and retaining original images as the default. Queued imports keep their captured mode. Implementation and verification remain pending.

### U2 — EPUB preview shows no titles, chapter contents or images

- Report: EPUB review preview shows no ToC titles, chapter contents, or images.
- Relevant path: `frontend/src/features/imports/EPUBReviewView.vue`, `frontend/src/api/epub-imports.ts`, `backend/internal/epubstore/review.go`, and the EPUB preview HTTP handler.
- Important distinction: the current review template is designed to render a paged **section-heading inventory** and a bounded **plain-text sample**, not a full hierarchical publication ToC, selectable chapter reader, or image preview. It has no image-rendering path. Chapter-click preview was previously deferred (see the completed import-history handoff linked from `PLAN.md`).
- The installed private EPUB reproduces missing titles and the initial empty sample through fresh acquisition → preparation → `epubstore.Review`, using a temporary reader home. This is backend-boundary evidence, not a reproduction against the user's existing receipt or a new browser check.
- **Title cause:** `epub/normalize.go` derives section titles solely from XHTML `head/title`. The inspected source sections have empty title elements; sections 1 and 25 have body headings. All 1,737 saved section titles are empty. Authored NCX navigation labels and targets are separately retained, but `epubstore.Review` returns the section-title inventory, not that navigation. `EPUBReviewView.vue` displays those empty strings without a fallback. There is no evidence of titles being lost in storage or frontend transport.
- **Sample cause:** `Review` samples only the section at the page's `start` index. Section 0 contains one image and no text; its plain-text projection is therefore empty. Explicit review starting at section 1 returns 301 sample bytes, and section 25 returns 4,094 bytes with truncation. Text preparation is working in those sections; the initial selection does not provide a useful prose sample for this book.
- **Image absence:** the prepared first section retains its image binding. The review DTO/template supports only a text sample, not image resources; image absence is not evidence of failed image preparation.
- These findings separate title presentation, sample selection and richer preview scope. A remedy is not yet selected; do not silently reinterpret authored navigation as one title per section, change persisted titles/revisions, or expand preview resource access.
- No real EPUB needs to be committed for investigation. Keep any supplied private fixture local/ignored.

### U3 — Received-files format selector needs All

- Request: **已接收檔案 → 格式** should offer **All / 全部**, in addition to TXT and EPUB.
- Code evidence: `frontend/src/features/imports/ImportReceiptsPanel.vue` has separate TXT/EPUB choices and calls their separate paginated receipt-list endpoints.
- Assessment: a reasonable mixed-import UX improvement, not evidence that storage formats should merge. Future implementation must define coherent ordering, pagination and filtering across formats; merely concatenating two independently paginated pages would not establish those semantics.
- Status: requested improvement recorded; no approach selected or implemented.

### U4 — EPUB received-files view lacks a status filter

- Request/question: EPUB should have a **狀態** filter like TXT unless its absence is intentionally necessary.
- Code evidence: `ImportReceiptsPanel.vue` displays the status selector only for TXT (`v-if="format === 'txt'"`); EPUB listing currently receives a cursor without a status filter. `import-format.ts` already projects EPUB acquisition/preparation/publication into shared UI states.
- Assessment: EPUB has meaningful pending, ready, failed, published and removing conditions. There is no demonstrated domain reason it cannot benefit from a status filter. Current absence is an implementation asymmetry, not a verified intentional requirement. Do not assume all TXT labels or the TXT `needs_review` state map directly to stored EPUB states.
- Status: desired consistency recorded; state choices, All-format behavior and filtering/pagination contract remain to be settled before implementation.

## Review findings

### R1 — Inbox refresh notification can be dropped while busy (medium)

`ImportInboxPanel.vue` watches `queue.revision` and requests a scan; `import-task.ts` refuses a task while busy. There is no retained pending refresh. Completion during a scan/review can leave the inbox stale until manual refresh. `ImportReceiptsPanel.vue` already handles revision changes during its load loop.

Evidence level: code-traced race, not a new executable reproduction. Small proposed direction, not accepted implementation: coalesce a refresh after the active operation without interrupting explicit review/resolution.

### R2 — Shared reader orchestration is unnecessarily compressed (medium)

`frontend/src/features/reader/ReaderView.vue`, especially mounting/loading, conversion and content-commit methods, packs asynchronous operations and state changes into dense lines. The ordering of navigation, displayed content, scroll and progress is hard to review safely.

Assessment: readability/maintainability concern, not a confirmed behavior defect. Prefer normal statement/block formatting first; extract only genuinely cohesive operations if needed. No new reader framework or generic state machine recommended.

### R3 — Passing frontend tests emit avoidable warnings (low)

The reviewed run emitted missing i18n keys, unresolved `RouterLink`, and missing test-router routes. Examples include `ReaderSettingsSheet.test.ts`, `ReaderBookmarksSheet.test.ts`, `TocNavigationList.test.ts`, `TocChapterList.test.ts`, `ReaderView.test.ts`, and `ImportWorkspace.test.ts`.

Assessment: incomplete fixture setup obscures useful warnings; these warnings do not prove production routes/translations are broken. Supply needed test stubs/routes/translations rather than globally suppress warnings.

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

Present the reproduced preview causes and settle the smallest appropriate preview behavior before implementing it. The shared device-local image preference is already accepted and can be implemented independently. Resolve mixed-history/filter contracts before U3/U4 implementation; keep reader readability cleanup separate unless a functional correction requires it. Update this note as fixes land; create a dedicated plan only if the accepted work warrants one.
