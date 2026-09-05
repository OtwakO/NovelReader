# Reader Navigation Performance

**Status:** Completed
**Branch:** `feat/reader-navigation-performance`

## Goal
Make repeat and adjacent chapter navigation responsive without sacrificing progress ordering,
source isolation, or explicit content refresh.

## Scope and Accepted Approach
- Remove outgoing progress-save acknowledgement from chapter display's critical path; retain
  ordered writes and explicit barriers for bookmarks and source switching.
- A reader-owned, bounded chapter loader serves navigation and one-chapter lookahead. It keeps
  recent documents and deduplicates requests; no durable-cache policy change or new service.
- Prefetch the next readable chapter after display, enabled by default with a setting to disable.
  One speculative request at a time; no recursive downloads, image preloads, or progress writes.
- Serialize chapter fetches within a reading session because source scripts can mutate session
  state. Source switching drains outstanding chapter work before changing the binding.
- Reset the loader on book/source/catalog replacement and unmount; obsolete completions must not
  replace current content or navigate the route. Browser cancellation reaches the backend workflow.
- Reuse catalog and chapter conversion by original object identity and conversion mode, without
  mutating canonical documents or making a global cross-reader cache.
- Replace the top-right bookmarks button with a three-dot actions dropdown, matching the existing
  reader UI. Actions: existing Bookmarks panel and Refetch current chapter. Keep Back on the left.
- Refetch bypasses session reuse, keeps scroll position, and refreshes the current display. Existing
  backend outage fallback remains explicit (offline-copy status), not misrepresented as fresh data.

## Decisions
- Session-local cache first: avoids redesigning the persistent outage cache and its freshness keys.
  No cross-tab/account persistence. Bounded recent entries, not a whole-book cache.
- Changing source (even returning to a previous source) clears old cached documents and converted
  displays. Explicit refresh clears speculative work/results before fetching again.
- Source fetches remain ordinary authenticated chapter requests; no provider-specific behavior.
- No subagents, real-source fixtures, or private diagnostic output in commits.

## Evidence
- Backend synthetic A/B/A probe: all three visits fetch upstream, even when A is cached.
- Browser controlled 100 ms progress + 200 ms content delays produced 319–328 ms display latency;
  immediate responses produced 25–31 ms. Actual conversion of 2,041 synthetic strings added 21–24 ms.
- Live sampling could not reach readable content; no successful live latency claim.

## Current State
Implemented and verified: bounded five-document session loader with in-flight deduplication,
serialized prefetch/foreground execution, source-switch draining, lifecycle disposal; navigation no
longer awaits progress acknowledgements; local displayed progress now advances on successful visits
so returning to the opening chapter persists correctly. Conversion reuse and the three-dot menu,
manual refetch, default-on prefetch preference, and translations are wired. Backend cancellation
now reaches content workflow execution while retaining the configured timeout. Documents and
chapter identity commit atomically after conversion, so progress cannot point at undisplayed text.
The menu preserves Back and the existing Bookmarks panel; refetch has a visible pending status.
Disabling prefetch stops future speculation; already-started work may finish.

## Next Action
No implementation work remains. Live-source timing remains unverified; source compatibility
failures from diagnosis are outside this workstream. Push and deployment require separate approval.

## Verification
- Reader/API/i18n slice: 17 test files, 46 tests passed; frontend typecheck and production build passed.
- Pre-merge verification: complete `internal/api` and `internal/book` package tests passed,
  including the in-flight request-cancellation regression (not a full backend suite).
- Both deployment/local Compose configurations validate. Containers were not started against reader data.
- Browser: production frontend with synthetic 100 ms progress and 200 ms content responses,
  2,000 catalog entries and 40 paragraphs. After one-chapter prefetch warmed, A/B navigation displayed
  in 13–28 ms (prior controlled baseline 319–328 ms). No foreground content request on cache hits;
  manual refetch issued one new current-chapter request and restarted bounded next-chapter lookahead.
- Desktop and 390 px mobile actions menu visually checked; keyboard dismissal and disabled actions
  covered by component tests. Mechanical UI detector found no issues in the changed controls.
- Source switching with an in-flight prefetch rejects the old result; progress ordering, cached
  revisits, manual-refetch recovery/position, and atomic display identity have deterministic coverage.
- AFT diagnostics were incomplete/unavailable; compiler/build and targeted tests were the gates.
- No live BookSource contents or credentials added; private fixtures were not used for implementation.

## Compatibility and Rollback
No storage migration or public document-shape change. First loads still incur upstream latency;
source switching/manual refetch can wait for an already-running fetch to drain, intentionally avoiding
late script/session writes across a binding change. The new preference defaults on for existing
readers. Removing this branch restores previous behavior; persistent chapter fallback data remains
unchanged. Unfinished work must stay handoff-ready here.
