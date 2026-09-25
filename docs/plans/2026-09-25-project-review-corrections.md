---
status: active
updated: 2026-09-25
---

# Verified project-review corrections

## Goal
Verify the review findings against production paths, record evidence before fixing, and correct confirmed causes with deterministic regressions. Work directly without subagents.

## Scope and accepted approach
Five reported defects plus verification of the reported maintenance concerns. No speculative redesign, dependency changes, broad cleanup, source-specific exceptions, or live/private BookSource fixtures. Preserve useful diagnostics without logging untrusted source content. Browser invalidation is a lifecycle/ownership change and needs race tests. Other fixes are localized or shared-boundary corrections.

## Verification ledger (baseline `183620a`)

| ID | Finding | Verification before implementation | Status |
|---|---|---|---|
| R1 | Credentials in search logs | Temporary Go overlay drives real `searchSources` against a synthetic failed HTTP request. `TestQuickReviewSearchLogDoesNotExposeURLSecrets` fails: the query token appears at INFO. Repeated on unchanged baseline. Repository regression additionally reproduced DEBUG leakage of URL, source name and search query. Search/fetcher logs now emit only owned source ID, fixed categories, counts and error types, not raw errors/URLs/source text. | Fixed; book/fetcher tests and focused race regression pass |
| R2 | Browser requests survive source invalidation | Overlay tests `TestQuickReviewPendingBrowserInvalidation` and `TestQuickReviewStartingBrowserInvalidation` both fail: a pending request starts after `CloseSource`, and a blocked launch publishes after invalidation. Repeated on unchanged baseline. Repository regressions also fail three consecutive race-enabled runs. `pending` has no source owner and starting work is not represented in owned state. | Fixed; sourceinteraction/API normal tests and three focused race runs pass |
| R3 | Silent HTTP body truncation | `TestQuickReviewRejectOversizeResponse` receives 10,485,760 of 10,485,761 bytes without error through real `fetcher.Client`. Repeated on unchanged baseline. Repository `TestTextResponseLimit` and `TestFingerprintRejectsOversizedTextWithoutFallback` also fail before fixes through both real transports; exact-limit ordinary responses pass. The fingerprint check includes a fallback to prevent bypassing the size boundary. | Fixed with shared complete-body admission; transport/consumer tests pass |
| R4 | Alternate binding loses `lastChapter` | Vite reproduction and repository parameterized regression both fail before the fix, covering promotion and non-promotion. Both branches now use one binding projection preserving `lastChapter` and existing context. | Fixed; search tests/typecheck/build pass |
| R5 | Unavailable storage breaks Search | Vite-loaded production Pinia store with storage throwing `SecurityError`: both `initialize()` and `search()` throw. Repository regression also fails before the fix. All Search persistence operations now remain optional at the storage boundary, with an existing warning and in-memory search unaffected; corrupt saved-state recovery is covered. | Fixed; search tests/typecheck/build pass |
| M1 | Legacy merger / misplaced tests | Go `TestMergeAndSortKeepsLastChapterOnAlternateBinding` does not cover the production frontend merger. The stronger assertion that all legacy code is unused still requires reliable caller verification; scanner absence alone is insufficient evidence for removal. | Coverage gap confirmed; dead-code claim unverified |
| M2 | Compressed ReaderView template/styles | Source inspection confirms compressed sections. This is a readability concern, not an independently demonstrated behavior bug; do not mix broad reformatting into functional fixes. | Confirmed maintenance concern; no functional fix required |
| M3 | Stale project branch description | `PLAN.md` says cache work is on its feature branch; local `main` is `183620a` and includes that work. | Confirmed; corrected in router |

The temporary overlay command was `cd backend && go test -overlay=/tmp/novelreader-review/overlay.json ./internal/sourceinteraction ./internal/fetcher ./internal/book -run TestQuickReview -count=1 -timeout 30s`. Its four failing assertions are the expected baseline reproductions, not verification of fixes. Temporary files are not durable tests; promote each reproduction into the appropriate repository test before its fix.

## Browser correction boundary
Bind each registered request to its emitting source. Install the session owner atomically when consuming that request, before any worker I/O; the same owned slot spans starting and active states. Source invalidation removes matching pending requests and detaches matching starts or active sessions. A detached launch cannot publish and must close any late worker result. Invalidation does not cancel the worker HTTP exchange: allowing its bounded response to return preserves the worker ID needed for cleanup, rather than creating an untracked worker context. Replacement uses that same ownership transition, not a second lock or delayed cleanup patch. Internal registration callers change together; no browser wire schema changes. Verify unrelated-source preservation and concurrent replacement in addition to the two baseline failures.

## Current State
Verification results recorded before implementation. R1–R5 are fixed and verified. HTTP text admission reads one extra byte and returns a typed size error instead of publishing partial data; size-policy failure cannot retry through the fingerprint fallback. Browser requests now carry their source owner, and the owned session slot covers launch as well as active use. Invalidation rejects late publication, cleans late workers and preserves unrelated requests. All source-action, continuation and Explore registration callers are updated. Existing full Go/frontend checks passed during review, but missed the confirmed defects. Worker verification was blocked by missing `patchright`; this is an environment limit, not an established worker bug.

## Next Action
Verify M1 using a compiler-backed removal probe before removing anything; then finish affected-area regression checks and documentation.

## Verification
Frontend: `npm test -- src/features/search` passes 17 tests in 4 files; direct `vue-tsc --noEmit` and Vite build pass. R4's two branch assertions and R5's unavailable-storage regression failed before their fixes. Use targeted normal tests first; race tests for browser lifecycle; frontend typecheck/build after frontend changes. Expand only for affected shared boundaries. `go test ./internal/fetcher ./internal/fingerprint ./internal/sourceexec ./internal/book -timeout 120s` passes after R3. For R1, `go test ./internal/book ./internal/fetcher -timeout 120s` and `go test -race ./internal/book -run TestSearchLogsExcludeSourcePayloads -timeout 30s` pass. For R2, `go test ./internal/sourceinteraction ./internal/api -timeout 120s` passes, and `go test -race ./internal/sourceinteraction ./internal/api -run 'Browser|SourceInteraction|ScheduledReplacement' -count=3 -timeout 90s` passes. Tests cover invalidation, source ownership, concurrent replacement, cleanup and the HTTP action/reset boundary. No live-source or real worker-browser checks claimed.

## Compatibility and rollback
No reader schema or persistent-data migration is intended. Reject oversized responses explicitly rather than accepting corrupt partial input; this deliberately changes oversized-response behavior. Browser ownership changes are internal and require updating all registration callers together. Roll back isolated commits if necessary; no data rollback is expected. Do not push or deploy without authorization.
