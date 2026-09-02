# Candidate Book Journey

**Status:** Current product and interface reference

## Purpose

Define how a Search or Explore result becomes a transient Candidate Book Detail and, optionally, a stored shelf book. This reference supersedes the historical readable-admission contract archived at [`docs/archive/designs/candidate-book-journey-readable-admission.md`](../archive/designs/candidate-book-journey-readable-admission.md).

## User-visible forms

1. **Candidate Book Detail** — transient and non-destructive. It displays selected Book Info metadata and known-source progress. It has no reading progress, bookmarks, source recovery, or removal controls.
2. **Stored Book Detail** — durable and ID-based. It owns catalog state, reading progress, bookmarks, source recovery/switching, and removal.

Opening Candidate Book Detail never shelves the book. Search/Explore card Add and Candidate Book Detail use the same backend operation; card Add requests automatic commit, while Candidate Book Detail requires explicit confirmation.

## Operation interface

Authenticated routes:

- `POST /api/candidate-resolutions`
- `GET /api/candidate-resolutions/{id}`
- `GET /api/candidate-resolutions/{id}/events`
- `DELETE /api/candidate-resolutions/{id}`
- `POST /api/candidate-resolutions/{id}/shelve`

An operation owns reader identity, the deduplicated source-binding queue, one reader-runtime lease, per-source Book Info attempts, bounded snapshots/events, selected metadata, cancellation/expiry, and idempotent commit.

## Scheduling and selection

- Queue all known bindings in stable primary-first order.
- Start up to five Book Info attempts immediately.
- Refill freed capacity while no binding is selected.
- A successful lower-priority attempt waits as `ready` until every preferred attempt completes or fails.
- Select the first successful binding by stable priority, then mark it `verified`.
- Untouched and winner-cancelled losing attempts become `skipped`, not `failed`.
- Preserve every discovered binding for shelf source recovery.

The only validation stage is `book_info`. TOC and chapter content are not shelf-admission gates; they belong to the separately observable catalog/Reader workflows.

## Operation states

- `running` — Book Info attempts remain unresolved or losing work is draining.
- `verified` — selected metadata is available; explicit Add may commit it. Automatic commit continues following updates.
- `committed` — shelf persistence completed and includes the stored book.
- `exhausted` — all usable bindings failed or operation lifetime ended without metadata.
- `cancelled` — user cancellation acknowledged; active work may drain privately.
- `failed` — infrastructure or persistence failed outside normal source exhaustion; safe commit retry may retain verified metadata.

Commit is idempotent and does not recrawl. A stored logical-book match merges bindings while preserving the existing book ID and reading state.

## Frontend ownership

`frontend/src/features/candidates/` owns candidate transport, tab-local operation identity, SSE restoration, and shared progress presentation.

- Search/Explore cards own direct automatic-commit continuity.
- Candidate Book Detail owns transient metadata presentation, explicit commit, and route replacement.
- Backend-annotated `shelfBookId` is authoritative for existing shelf membership; tab-local markers only smooth same-tab recency.
- Remount fetches the authoritative operation snapshot before reconnecting.
- Route changes, disclosure collapse, and SSE disconnect do not cancel operations.

## Liveness and cleanup

- Every Book Info attempt and the whole operation are bounded.
- Automatic commit reaches `committed` or a visible `failed` state; it never remains indefinitely in a finishing state.
- Cancellation is acknowledged immediately, while runtime release waits for active work to drain.
- Process restart invalidates transient operation IDs; a missing operation is forgotten and may be restarted.
- Expiry, eviction, cancellation, commit, and shutdown release runtime ownership without leaks.

## Presentation and recovery

- Show selected source metadata and concise per-source progress without raw parser/source payloads.
- Distinguish source exhaustion from storage/commit failure.
- Keep retry, cancellation, disconnect, and committed states visible.
- Candidate Book Detail shares Book Detail visual primitives but does not show stored-book controls.
- English, Simplified Chinese, and Traditional Chinese catalog keys remain symmetric.

## Verification focus

- stable-priority Book Info selection with five work-conserving attempts;
- direct automatic commit and explicit detail-page commit;
- no TOC/content execution during admission;
- idempotent no-recrawl persistence;
- binding preservation and existing logical-book merge;
- cancellation/drain/expiry/runtime release;
- authoritative snapshot restoration and terminal failure visibility;
- responsive, keyboard-accessible frontend states.
