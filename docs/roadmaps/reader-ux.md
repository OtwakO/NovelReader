# Reader UX Roadmap

**Status:** Proposed future direction, not committed work

## Purpose

Track user-facing reading improvements that remain relevant after the current Vue Reader, shelf, TOC, source-recovery, keyboard, and wake-lock work. Promote an item into `PLAN.md` and a focused implementation plan only after the user problem and scope are accepted.

The archived 2026-08 assessment is preserved at [`docs/archive/research/future-reader-ux-opportunities-2026-08.md`](../archive/research/future-reader-ux-opportunities-2026-08.md). That snapshot also lists gaps now implemented and is not current truth.

## Small opportunities

### Reader presets

Offer a small fixed set of device-local presets—such as Day, Night, Parchment, E-ink, and Large text—that apply existing Reader preference fields. Do not introduce user-created profile CRUD, backend persistence, or cross-device synchronization in the first version.

### Reader top/bottom and return actions

Consider compact overflow actions for chapter top, chapter bottom, and return to the last persisted position after temporary navigation. Do not expand the primary mobile control bar without evidence that these actions need permanent prominence.

### Consistent source update-time presentation

`updateTime` already travels through Search, Explore, Book Info, candidate, and shelf data. If prioritized, render it consistently beside latest-chapter metadata without triggering hidden source requests or treating it as catalog freshness authority.

### Automatically reveal current rows

When opening the TOC or known-source list, bring the current row into view where practical and preserve its relative position across local filtering/ordering. This should remain frontend-local.

## Moderate product decisions

### Browser speech synthesis

A useful implementation requires paragraph chunking, voice selection, pause/resume, active-paragraph tracking, lifecycle behavior, and chapter transitions. Begin with browser `speechSynthesis`; do not add proprietary/cloud voice infrastructure by default.

### Shelf groups

Requires a canonical group model, storage, CRUD, assignment, deletion semantics, and integration with shelf filtering/navigation. This is not a presentation-only change.

### Manual book addition by source and URL

Requires a source selector and a backend interface that validates a source-specific book URL into canonical Book Info before admission.

### BookSource export

Must preserve unknown imported fields and clearly exclude or separately handle source credentials, cookies, tokens, and reader-owned interaction state.

### Cache/offline management

Needs a product model distinguishing remote-only content, server-cached chapters, and browser/device availability, including ownership, counts, refresh, and deletion semantics.

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
