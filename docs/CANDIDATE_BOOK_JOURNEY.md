# Candidate Book Journey

## Purpose

This document is the current product, backend, and frontend contract for moving a Search or Explore result into a readable Book Detail and, optionally, onto the shelf. It replaces earlier synchronous preview/shelve concepts and is the standard used to audit the active implementation.

NovelReader must let a reader inspect a discovered book without persisting it, add it directly when desired, understand source validation progress, recover from failure, and never wait indefinitely without a truthful state.

## Product model

A discovered logical book has two user-visible forms:

1. **Candidate Book Detail** — transient and non-destructive. It shows verified metadata, introduction, contents, selected source, and fallback information. It has no reading progress, bookmark, source-recovery, or removal controls.
2. **Stored Book Detail** — durable and ID-based. It owns reading progress, continuing, source recovery, source switching, and removal.

The active transient route is `/books/candidate`, implemented by `CandidateBookDetailView`. User-facing copy says Book Detail, checking sources, or adding to shelf—not “preview”, “preview token”, or other implementation language.

Opening Candidate Book Detail never shelves the book. Adding from a Search/Explore card or Candidate Book Detail always uses the same backend candidate-operation contract.

## Entry points

### Search and Explore card

The card has two distinct actions:

- **Book metadata target** — opens Candidate Book Detail so the reader can inspect the book before adding it.
- **Add to shelf** — starts or reconnects to a candidate operation with automatic commit.

The metadata target must be labelled for assistive technology as “View details for {book}”. A directional affordance may accompany it, but no duplicate Details button may target the same route.

### Candidate Book Detail

Candidate Book Detail restores an existing operation for the same tab-local candidate when available. Otherwise it starts one operation without automatic commit. It first presents source-check progress, then replaces that state with the full candidate detail when verification succeeds.

The verified page offers explicit **Add to shelf** and **Back** actions. Add commits the verified server-held result without recrawling and replaces the route with Stored Book Detail.

## Backend operation contract

The authenticated operation API is:

- `POST /api/candidate-resolutions`
- `GET /api/candidate-resolutions/{id}`
- `GET /api/candidate-resolutions/{id}/events`
- `DELETE /api/candidate-resolutions/{id}`
- `POST /api/candidate-resolutions/{id}/shelve`

An operation owns:

- account ownership;
- the deduplicated primary-first candidate binding queue;
- one dedicated reader-runtime lease;
- per-source attempts and stage progress;
- the first verified readable result and TOC;
- bounded snapshot/event history;
- cancellation, expiry, cleanup, and idempotent commit.

No synchronous candidate-ingestion endpoint or separate preview-token store may coexist with this path.

## Scheduling and validation

- Queue every deduplicated discovered binding in stable primary-first order.
- Start up to five validations immediately.
- While no winner exists, refill every freed slot immediately from the queue.
- Validate Book Info, non-empty TOC with a readable chapter URL, and credible chapter content in sequence.
- Stop selecting candidates after the first fully verified readable source.
- Mark untouched queued attempts `skipped` after a winner appears.
- Cancel and drain losing active attempts, recording them as `skipped` rather than unavailable, before using or releasing their reader runtime.
- Preserve all discovered bindings for stored alternate-source persistence, including skipped bindings.

Attempt states are `queued`, `running`, `failed`, `verified`, and `skipped`. Stages are `book_info`, `toc`, and `content`.

## Operation states and transitions

### `running`

No verified winner has completed drain yet, or a winner exists while losing attempts are still unwinding.

- Before a winner: show true active, completed, queued, and failed state.
- After a winner: show the verified source separately, active losing attempts as finishing, and untouched sources as skipped—not waiting.

### `verified`

A readable server-held result is available. Losing attempts may still be draining privately, but visible commit progress is not blocked on that cleanup.

The snapshot explicitly distinguishes commit intent:

- `automaticCommit: false` means the verified result awaits an explicit Add action. Search/Explore cards restoring this operation show Add and reuse it instead of displaying an indefinite finishing state or starting another crawl.
- `automaticCommit: true` means direct Add already requested persistence. `verified` is an intermediate finishing snapshot, so clients keep following events until `committed` or `failed`.
- `commitPending: true` means validation succeeded but persistence failed. Retry commits the retained verified result without starting another validation operation.

The operation retains its reader-runtime lease until commit, cancellation, expiry, eviction, or shutdown.

### `committed`

Shelf persistence completed. The response includes the stored book. Commit is idempotent and must not recrawl the source.

Direct card Add automatically reaches this state. Candidate Book Detail reaches it only after explicit Add.

### `exhausted`

All usable bindings failed or the bounded operation lifetime expired without a winner. Show a clear retry action and concise source failure summary.

### `cancelled`

Explicit user cancellation is acknowledged immediately in the snapshot. Active work drains privately; the runtime releases only after drain completes. Collapse, route navigation, or SSE disconnect never cancels the operation.

### `failed`

Infrastructure or persistence failed outside normal source exhaustion. A verified automatic-commit operation must never remain visually stuck in “finishing”; commit failure becomes `failed` with a retryable, user-facing explanation. The verified result may remain commit-capable when retry is safe.

## Time and liveness guarantees

- Every validation stage has a bounded deadline.
- The whole operation has a bounded lifetime.
- Automatic commit must complete or transition to a visible failed state; it may not remain `verified` indefinitely.
- Frontend SSE disconnect shows reconnection status but does not invent a terminal state.
- Remount fetches the authoritative snapshot before reconnecting.
- Process restart invalidates transient operation IDs; the frontend forgets a 404 operation and offers a fresh action without treating it as a source failure.

## Frontend state ownership

`features/candidates` owns candidate DTOs, operation transport, tab-local operation identity, event subscription, and reusable progress presentation.

- Search/Explore card action owns direct automatic-commit restoration and terminal card status.
- Candidate Book Detail owns transient detail restoration, explicit commit, and route replacement.
- Neither surface duplicates scheduler logic or interprets BookSource rules.
- Expanded/collapsed disclosure preference is component-local and non-persistent.
- Canonical shelf membership is backend-owned; tab-local completion markers are continuity hints, not authorization or durable truth.

`CandidateProgressDetails` renders the same attempt semantics on cards and Candidate Book Detail. Operation transport/lifecycle orchestration must not be duplicated across pages when the behavior is identical.

## Candidate Book Detail presentation

Candidate and Stored Book Detail share the same visual grammar:

- shared cover/identity hero primitives where behavior permits;
- `BookDetailSection` for Introduction and other bordered detail sections;
- a shared section-body wrapper for padding and readable prose measure;
- `BookDetailToc` for contents, configured non-interactive for candidates;
- shared spacing rhythm between hero, Introduction, Contents, and actions.

Candidate-specific differences are explicit:

- no progress, Continue Reading, remove, bookmark, or source-recovery controls;
- selected-source and fallback status are visible;
- Add to shelf and Back actions remain close to the verified identity and available after long content;
- missing introduction shows localized fallback copy inside the normal shared Introduction section;
- translation keys must render human copy—never literal keys such as `bookDetail.introduction`.

## Progress UI requirements

Progress presentation must show:

- concise summary control or page heading;
- verified winner separately from active checks;
- source display name with current user-facing stage;
- recent unavailable sources without raw URLs or parser internals;
- queued count only before a winner;
- skipped count after a winner;
- Cancel while cancellable;
- Retry after exhausted/failed;
- visible Added, Cancelled, or Failed terminal feedback.

Long source names and translations wrap or truncate without changing card columns or causing horizontal overflow. Dynamic updates use an appropriate live region without duplicating visible text for sighted users.

## Error and recovery behavior

- Source failures are expected attempt outcomes, not page-level crashes.
- WebView guidance appears only when relevant to a failure.
- Commit/storage errors are distinguished from source-validation exhaustion.
- Cancellation failure leaves the operation visible and retryable.
- SSE failure keeps the latest snapshot visible and reports reconnection.
- Explicit retry starts a fresh operation after clearing the old terminal operation identity.

## Cleanup rules

The following are remnants and must be removed when found in active production code:

- synchronous candidate preview/shelve/readable HTTP routes or frontend wrappers;
- separate preview-token persistence;
- fixed attempt-count truncation;
- staged three-worker hedge scheduling;
- duplicate Details controls;
- generic one-line progress copy that hides per-source state;
- independent card/page implementations of identical operation lifecycle logic;
- ad-hoc Candidate Book Detail section borders, padding, or TOC markup that bypass shared Book Detail components;
- obsolete translation keys and literal-key fallbacks;
- tests that pin superseded routes or state fields.

## Verification matrix

### Backend

- five initial active attempts;
- work-conserving refill starts the next queued binding immediately when one of five active attempts fails;
- first readable winner stops new launches;
- untouched bindings become skipped;
- losing attempts drain before runtime release;
- cancellation acknowledges immediately and releases after drain;
- explicit and automatic commit are idempotent and no-recrawl;
- automatic commit success reaches committed;
- automatic commit failure reaches visible failed state and supports safe retry;
- verified operation expires and releases resources;
- shutdown and per-reader eviction do not race or deadlock.

### Frontend component/integration

- card and Candidate Book Detail restore the authoritative snapshot, distinguish explicit from automatic commit intent, then reconnect only while updates are expected;
- Search/Explore restoration of a detail-owned verified operation exposes Add and commits it without recrawling;
- remembered non-committed operations are reused only when their complete ordered `(sourceUrl, bookUrl)` binding set still matches the current result; later Search batches invalidate stale finished work so newly discovered sources can participate;
- automatic verified operations continue through the committed or failed event instead of becoming stuck in finishing;
- collapse/navigation never cancel;
- winner/draining/skipped states render truthfully, including post-winner active cancellations as skipped rather than failed;
- stored-book deletion invalidates tab-local committed markers so Search/Explore return to Add;
- success, cancellation, exhaustion, disconnect, and commit failure remain visible;
- Candidate Book Detail uses shared section and TOC components;
- all English, Simplified Chinese, and Traditional Chinese keys have parity;
- long source names, introductions, and chapter titles remain responsive.

### Real browser

Use visible Search/Explore UI and real backend APIs through normal interaction—not direct API setup for the feature path—to verify:

- Search result → inline Add → committed card state;
- Search result → Candidate Book Detail → verified metadata/Introduction/TOC → Add → Stored Book Detail;
- cancellation acknowledgement;
- failed/exhausted retry;
- route away/back restoration;
- desktop and mobile layout, keyboard focus, no horizontal overflow, and clean console.
