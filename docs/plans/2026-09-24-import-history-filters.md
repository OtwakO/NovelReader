---
status: completed
updated: 2026-09-24
---

# Mixed-format import history and status filters

## Goal

Resolve U3/U4 from the [import issue note](../notes/2026-09-21-import-user-testing-and-branch-review.md): one received-files list with All/TXT/EPUB, newest imports first, and useful status filtering for both formats.

## Scope

One cohesive listing/API/UI change. Preserve receipt acquisition, preparation, admission and removal ownership. No schema migration, live-data reset, deployment, inbox-refresh fix or reader cleanup.

## Accepted Approach

- Default to All formats. Order by original creation time descending, with deterministic format/ID tie-breaking; preparation updates must not move rows.
- Shared filters: All, Processing, Ready, Needs review, Failed, Added, Removing. Rows retain format-specific details.
- Needs review applies to both formats under existing content-review rules, not the device's review-before-adding preference. Ready excludes these cases. EPUB encoder performance notices alone do not require review.
- An additive authenticated listing endpoint uses bounded format-owned queries and one coherent cursor. Do not concatenate independently paginated client pages or load all receipts into the browser. Keep existing format-specific endpoints compatible.
- Derive EPUB review eligibility from saved preparation diagnostics, without opening sections or reparsing archives. Existing prepared receipts must work without reimport.

## Compatibility and rollback

No stored-data changes. Existing provider endpoints and admission checks remain available. The listing/UI change can be reverted without rewriting receipt data. Shared status is a listing projection, not a new persisted lifecycle or generic receipt framework.

## Current State

Completed: `importhistory.Query` owns the shared chronological cursor; each store's `history.go` owns status projection and bounded SQL. `/api/imports/receipts` merges at most two bounded result sets, retaining provider DTOs. The received-files panel uses the new endpoint, shared filters and explicit format labels. Existing provider HTTP endpoints remain unchanged. Affected backend packages, 50 import UI tests, typecheck and build pass. The final list-reset change also passed the 23 tests in the two affected view files. Current usage/architecture documentation is updated.

## Next Action

No remaining U3/U4 implementation. Inbox refresh (R1), reader readability (R2) and unrelated test-fixture warnings (R3) remain separate.

## Verification

Passed: `go test ./internal/txtstore ./internal/epubstore ./internal/api -count=1`; frontend direct-Node typecheck/build; `NODE_OPTIONS=--no-experimental-webstorage node node_modules/vitest/vitest.mjs run src/features/imports --reporter=dot` (50 tests). The new synthetic API test failed on the absent route first, then passed through actual acquisition/preparation, equal-time mixed pagination, review/ready/failed/added filters, stable order after publication and reader isolation. Frontend tests cover default All, filter/page resets and retained admission checks. Existing `/explore` test-router warnings remain unrelated.

Also passed: the new history API test under `-race`; an isolated Chromium journey through actual synthetic TXT/EPUB uploads, default All, mixed-format Needs review results and EPUB-only narrowing. Desktop (1280×900) and mobile (390×844) screenshots were inspected with no page overflow. Mechanical UI detection reported no findings. Browser artifacts remain under ignored `reference/import-history-check/`; no real book content was used. The disposable server initially expired during setup; it was restarted against the same temporary test root before the successful journey.

Limits: no deployment, schema change, existing-home mutation, full-repository suite, load benchmark or cross-browser campaign. The existing unrelated `/explore` fixture warnings were not suppressed or folded into this work.
