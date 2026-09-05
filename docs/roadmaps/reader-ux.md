# Reader UX Roadmap

**Status:** Proposed future direction, not committed work

## Purpose

Track user-facing reading improvements that remain relevant after the current Vue Reader, shelf, TOC, source-recovery, keyboard, and wake-lock work. Promote an item into `PLAN.md` and a focused implementation plan only after the user problem and scope are accepted.

The archived 2026-08 assessment is preserved at [`docs/archive/research/future-reader-ux-opportunities-2026-08.md`](../archive/research/future-reader-ux-opportunities-2026-08.md). That snapshot also lists gaps now implemented and is not current truth.

## Small opportunities

### Reader presets

Offer a small fixed set of device-local presets—such as Day, Night, Parchment, E-ink, and Large text—that apply existing Reader preference fields. Do not introduce user-created profile CRUD, backend persistence, or cross-device synchronization in the first version.

### Reader top/bottom and return actions

The existing overflow menu already provides Bookmarks and Refresh. Consider extending it with
chapter top, chapter bottom, and return to the last persisted position after temporary navigation.
Keyboard top/bottom navigation already exists; the remaining opportunity is explicit pointer/touch
actions. Do not expand the primary mobile control bar without evidence of need.

### Consistent source update-time presentation

`updateTime` already travels through Search, Explore, Book Info, candidate, and shelf data. If prioritized, render it consistently beside latest-chapter metadata without triggering hidden source requests or treating it as catalog freshness authority.

### Automatically reveal current rows

The TOC already centers/focuses the current chapter on opening, order changes, and clearing search,
and provides an explicit current-chapter action. Remaining opportunities are equivalent known-source
list behavior and relative-position preservation where useful. Keep any extension frontend-local.

## Moderate product decisions

### Browser speech synthesis

A useful implementation requires paragraph chunking, voice selection, pause/resume, active-paragraph tracking, lifecycle behavior, and chapter transitions. Begin with browser `speechSynthesis`; do not add proprietary/cloud voice infrastructure by default.

### Shelf groups

Requires a canonical group model, storage, CRUD, assignment, deletion semantics, and integration with shelf filtering/navigation. This is not a presentation-only change.

### Manual book addition by source and URL

Requires a source selector and a backend interface that validates a source-specific book URL into canonical Book Info before admission.

### BookSource export

A dedicated user-facing export flow remains an opportunity; lossless definition serialization and
single-definition retrieval already exist. Export must distinguish imported definition content
(which may itself contain sensitive values) from reader-owned credentials and interaction state,
and define explicit handling of secrets rather than claiming every lossless export is credential-free.

### Cache/offline management

Recent session reuse, one-chapter prefetch, explicit chapter Refresh, and server outage fallback are
already implemented. Broader offline management still needs a product model distinguishing these
from durable browser/device downloads, including ownership, counts, bulk refresh, and deletion semantics.

### Full-book text search

Requires a defined corpus (cached/downloaded versus network), progress/cancellation, storage limits, result location, and source-switch behavior.

### Source dependency visibility

Showing which shelf books rely on a BookSource requires a backend association query and an explicit deletion policy: warn, block, or allow while preserving stored binding snapshots.

## High-complexity areas

Do not treat these as quick Reader enhancements:

- user-defined Reader profiles or keymap CRUD;
- user-defined text replacement/filter rules;
- chapter editing and persistence;
- a separate e-ink frontend;
- EPUB, CBZ/comic, PDF, audio, or video reading models;
- server-side or source-defined HTTP TTS;
- cross-device progress synchronization.

## Planning rules

Before promoting an item:

1. Confirm an observed user problem.
2. Prefer frontend-local work when current backend interfaces already provide the data.
3. Keep tab-local interaction state, device-local preferences, and server-owned Reader Data distinct.
4. Define mobile, desktop, keyboard, localization, and failure-state behavior.
5. Add only the focused tests needed for the practical risk.
