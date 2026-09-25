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
| R1 | Credentials in search logs | Temporary Go overlay drives real `searchSources` against a synthetic failed HTTP request. `TestQuickReviewSearchLogDoesNotExposeURLSecrets` fails: the query token appears at INFO. Repeated on unchanged baseline. | Confirmed; not fixed |
| R2 | Browser requests survive source invalidation | Overlay tests `TestQuickReviewPendingBrowserInvalidation` and `TestQuickReviewStartingBrowserInvalidation` both fail: a pending request starts after `CloseSource`, and a blocked launch publishes after invalidation. Repeated on unchanged baseline. `pending` has no source owner and starting work is not represented in owned state. | Confirmed; not fixed |
| R3 | Silent HTTP body truncation | `TestQuickReviewRejectOversizeResponse` receives 10,485,760 of 10,485,761 bytes without error through real `fetcher.Client`. Repeated on unchanged baseline. Fingerprint transport has the same unchecked `LimitReader` pattern; a direct regression there is still needed. | Confirmed fetcher; fingerprint regression pending |
| R4 | Alternate binding loses `lastChapter` | Vite-loaded production `mergeSearchResults` merges two synthetic source results; alternate `lastChapter` is `undefined` despite a supplied value. Both promotion branches construct bindings without that field. | Confirmed; not fixed |
| R5 | Unavailable storage breaks Search | Vite-loaded production Pinia store with storage throwing `SecurityError`: both `initialize()` and `search()` throw. Recovery/removal directly touches unavailable storage. | Confirmed; not fixed |
| M1 | Legacy merger / misplaced tests | Go `TestMergeAndSortKeepsLastChapterOnAlternateBinding` does not cover the production frontend merger. The stronger assertion that all legacy code is unused still requires reliable caller verification; scanner absence alone is insufficient evidence for removal. | Coverage gap confirmed; dead-code claim unverified |
| M2 | Compressed ReaderView template/styles | Source inspection confirms compressed sections. This is a readability concern, not an independently demonstrated behavior bug; do not mix broad reformatting into functional fixes. | Confirmed maintenance concern; no functional fix required |
| M3 | Stale project branch description | `PLAN.md` says cache work is on its feature branch; local `main` is `183620a` and includes that work. | Confirmed; corrected in router |

The temporary overlay command was `cd backend && go test -overlay=/tmp/novelreader-review/overlay.json ./internal/sourceinteraction ./internal/fetcher ./internal/book -run TestQuickReview -count=1 -timeout 30s`. Its four failing assertions are the expected baseline reproductions, not verification of fixes. Temporary files are not durable tests; promote each reproduction into the appropriate repository test before its fix.

## Current State
Verification results recorded before implementation. No production corrections yet. Existing full Go/frontend checks passed during review, but missed the confirmed defects. Worker verification was blocked by missing `patchright`; this is an environment limit, not an established worker bug.

## Next Action
Add repository regressions for R4/R5 and run them red, then fix their production paths. Address R1/R3 at logging/body-admission boundaries and R2 at browser request/session ownership. Record meaningful milestones here before committing. Verify M1 callers before removing anything.

## Verification
Fix verification: not started. Use targeted normal tests first; race tests for browser lifecycle; frontend typecheck/build after frontend changes. Expand only for affected shared boundaries. No live-source or worker-browser checks claimed.

## Compatibility and rollback
No reader schema or persistent-data migration is intended. Reject oversized responses explicitly rather than accepting corrupt partial input; this deliberately changes oversized-response behavior. Browser ownership changes are internal and require updating all registration callers together. Roll back isolated commits if necessary; no data rollback is expected. Do not push or deploy without authorization.
