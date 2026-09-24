---
status: completed
updated: 2026-09-16
---
# Import history and review preference

## Goal and scope
Show persisted TXT imports reliably and restore user control over automatic shelf admission. No data reset, schema changes, automatic deletion, or chapter-preview expansion.

## Accepted approach
- Default review-before-adding on. Browser-local preference, like existing reader appearance settings; shared between Settings and Local Import.
- Snapshot the preference for each newly queued file. Changing it never approves existing queued work. Warnings always require review.
- Keep authoritative receipt history visible even when this tab retains a transfer entry. Preserve explicit version checks and uncertain-operation recovery.

## Findings
Receipt history is a database projection, not duplicate book storage. Published records/originals/indexes remain needed while books exist. Discard/removal deletes records after byte cleanup; unfinished/failed imports persist until explicitly discarded. List pagination is by ID, not chronological. Preview responses are capped at 4096 bytes without reparse, but the current implementation decodes the selected section before truncating it. Chapter clicks would add requests and section decoding, not zero-overhead navigation; deferred as optional.

## Current State
Both history defects reproduced: filtering out this tab's receipts and dropping a completion refresh during an in-flight list request. Fixed with authoritative display and revision-aware re-reading. Shared device-local review preference implemented, default on and snapshotted for new entries; Settings and Local Import use one control/store. Existing settings remain browser-local; no reset needed.

## Next Action
No implementation pending. Optional follow-up: separately scope chronological pagination or per-chapter sample navigation if requested; neither is included here.

## Verification
Both new history regressions failed before fixes. Affected suite passes: 8 files / 45 tests; production build/typecheck passes. Scoped ESLint and whitespace checks pass. AFT inspection timed out; compiler/tests are the successful gates.
Synthetic production Chromium workflow passed default review, explicit addition, automatic opt-in, completed-history visibility, shared Settings control and persistence through reload. Desktop 1280px and mobile 390px had no horizontal overflow or page errors. No backend changes/tests or live-server audit; retention and preview findings are source inspection, not a storage benchmark.

Additional final boundary verification: reader-state reset, restore boundary and locale parity passed (3 files / 8 tests). No existing reader home was modified.
