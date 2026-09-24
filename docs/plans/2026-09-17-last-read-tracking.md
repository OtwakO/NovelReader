---
status: completed
updated: 2026-09-17
---

# Independent last-read tracking

## Goal
Adding or updating a book must not count as reading or displace the last-read book.

## Scope
Shared library persistence and book projections, reader progress writes, shelf resume/recent ordering, schema compatibility and focused regression coverage. No relative-time display, reading-event history or migration framework.

## Accepted Approach
- Reader epoch 13 adds `library_items.last_read_at INTEGER NOT NULL DEFAULT 0 CHECK(last_read_at >= 0)`.
- Library owns one UTC Unix-millisecond timestamp per publication. JSON `lastReadAt` is reusable metadata for future UI/features; zero means never read, not an estimated time.
- A successful revision-guarded progress write records server time atomically with progress. Reader queues that existing write after displaying a chapter, including an unchanged first-chapter/zero position. Fetches, prefetch, previews, admission, metadata, bookmarks and interpretation changes do not independently mark reading.
- Continue Reading uses only positive last-read timestamps; recent shelf sorting uses last-read order, with unread items following in creation order. No guesses from chapter position, state version or generic `updatedAt`.
- User approved the existing no-migration development cutover. Leave all existing homes untouched; use fresh isolated test data. Epoch-12 homes/archives require the previous application. Rollback requires the matching previous application plus preserved compatible data, not schema-marker edits.

## Current State
Implemented the independent field, epoch-13 schema and shared/native book projections. Only accepted progress advances the clock. Shelf resume/recent ordering now uses actual reading; opening the initial unchanged location records activity. Backup/restore and reparse preserve the timestamp. Canonical architecture, compatibility README and reset runbook updated. No existing reader home was modified.

## Next Action
No implementation pending. Optional future relative-time UI can consume `lastReadAt`; no extra storage or event history is needed.

## Verification
- Regression run: `npm test -- src/features/shelf/ShelfView.test.ts` — fails with newly added book displayed in Continue Reading, as expected.
- Regression now passes, including an unread-only shelf and a genuinely read book still at chapter 1/position 0.
- `go test -race ./internal/library ./internal/book ./internal/api ./internal/reading ./internal/txtstore ./internal/txtimport ./internal/readerstore ./internal/backup ./internal/candidate -count=1` — all nine packages pass. Includes stale/invalid progress, non-reading mutation isolation, both provider HTTP projections, portable timestamp preservation through reparse, and rejection of mismatched schema epochs without mutation.
- Final deterministic historical-clock regression and composed server startup: `go test ./internal/library ./cmd/server -count=1` pass. Whitespace and dated-plan filename checks pass.
- Frontend reader/shelf/import-workspace suite: 21 files / 62 tests pass, including failed load and prefetch exclusion; production build/typecheck and scoped ESLint pass.
- AFT diagnostic inspection timed out; compiled/tested gates above are the evidence. No live-browser/server workflow, hosted CI, deployment, storage benchmark or data reset was performed.
