# Future Reader UX Opportunities

## Purpose

This note records user-facing behaviors from Web Legado and original Android Legado that NovelReader does not yet provide, but that could improve everyday use without requiring broad crawler, storage, or security architecture changes.

It is a future-reference shortlist, not an active implementation plan. Items should be discussed and approved before entering `PLAN.md` as committed work.

## Current baseline

NovelReader already implements many of the reference products' highest-value behaviors:

- Streaming, cancellable, batched multi-source Search.
- Strict single-BookSource Explore with source-native catalogs and controls.
- Aggregated title/author Search results retaining source alternatives.
- Canonical shelf ownership of progress, source identity, bookmarks, and cache state.
- Durable alternate-source discovery and explicit source switching.
- Failure-triggered source recovery in Book Detail and Reader.
- Chapter and normalized in-chapter progress restoration.
- Bookmark creation, opening, deletion, and source-switch migration/orphan handling.
- Locally persisted typography, width, colors, and custom fonts.
- Text and server-indexed chapter-image rendering.
- Responsive mobile and desktop reading/management surfaces.

The opportunities below should extend these existing workflows rather than reproduce Web Legado's dense root interface or Android-only behavior.

## Recommended low-complexity sequence

### 1. Reader TOC search, ordering, and current-chapter positioning

**Current gap**

The Reader TOC renders the complete chapter list but has no filter, ordering control, or direct current-position action. This is increasingly inconvenient for books with hundreds or thousands of chapters.

**Suggested first version**

- Filter chapters by title.
- Toggle ascending and descending order.
- Provide **Jump to current chapter**.
- Automatically bring the current chapter into view when opening the TOC.
- Distinguish readable-chapter count from volume-heading rows.
- Preserve the existing chapter index when the displayed list is filtered or reversed.

**Why it is useful**

Web Legado and original Legado treat the catalog as an active navigation tool rather than a passive list. NovelReader already holds the complete chapter array in the Reader, so this can remain frontend-local.

**Estimated complexity:** Low.

**Likely ownership:** `frontend/src/features/reader/ReaderTocSheet.vue` plus focused component/state tests and locale keys.

---

### 2. Bookshelf search, sorting, and tab-local restoration

**Current gap**

The shelf renders every stored book without local filtering or sorting. Returning from Book Detail or Reader does not intentionally restore shelf query, sort selection, or previous scroll position.

**Suggested first version**

- Search by title or author.
- Sort by recently read, title, author, or reading progress.
- Show a clear no-matches state distinct from an empty shelf.
- Restore the active query, sort order, and shelf scroll position after Shelf → Detail/Reader → Back.
- Keep this state tab-local rather than synchronizing it across devices.

**Why it is useful**

Shelf search and location restoration become valuable as soon as a personal library grows beyond a few dozen books. They improve the core reading journey without exposing source mechanics.

**Estimated complexity:** Very low to low.

**Likely ownership:** `frontend/src/features/shelf/`, using route-local state or versioned `sessionStorage`.

---

### 3. Local filtering in Source Recovery

**Current gap**

Source Recovery can accumulate many persisted and newly discovered alternatives, but the user cannot filter the visible list by source name.

**Suggested first version**

- Text filter by source name.
- Quick views for **All**, **Stored**, **Newly discovered**, and **Current** when those distinctions remain useful and clear.
- Filtering must not affect scan progress, cursor state, persistence, or source selection.
- Empty-filter feedback must not be presented as an empty scan result.

**Why it is useful**

Web Legado's source popup supports searching known candidates. This becomes useful after several scan batches and is entirely independent of backend source execution.

**Estimated complexity:** Very low.

**Likely ownership:** `frontend/src/features/source-recovery/SourceRecoveryPanel.vue` and its component tests.

---

### 4. Conservative Reader keyboard controls

**Current gap**

The Vue Reader does not currently install desktop/e-ink keyboard navigation.

**Suggested fixed defaults**

| Key | Initial behavior |
|---|---|
| `ArrowLeft` | Previous chapter |
| `ArrowRight` | Next chapter |
| `PageUp` | Scroll one viewport upward |
| `PageDown` | Scroll one viewport downward |
| `Space` | Scroll one viewport downward |
| `Home` | Top of current chapter |
| `End` | Bottom of current chapter |
| `Escape` | Close the active Reader overlay; otherwise return to Book Detail |

**Required guards**

- Ignore navigation shortcuts while an input, select, textarea, or editable element owns focus.
- Except for Escape, ignore shortcuts while a Reader overlay is open.
- Do not interfere with browser text selection.
- Avoid chapter navigation for an arrow key when an applicable horizontal scrolling surface owns the event.
- Keep the first version fixed rather than introducing configurable keymaps.

**Why it is useful**

Original Legado and Web Legado support keyboard and page-key workflows because long-form reading occurs on desktops, tablets with keyboards, and e-ink devices as well as phones.

**Estimated complexity:** Low to moderate because interaction guards need focused regression coverage.

**Likely ownership:** A small Reader-focused keyboard controller/composable or ordinary TypeScript module consumed by `ReaderView.vue`.

---

### 5. Optional screen wake lock while reading

**Current gap**

Mobile and tablet screens may sleep during long reading sessions.

**Suggested first version**

- Device-local Reader preference: **Keep screen awake while reading**.
- Request `navigator.wakeLock.request('screen')` only while Reader is active and the preference is enabled.
- Reacquire after document visibility returns.
- Release when leaving Reader or disabling the preference.
- Treat unsupported browsers and request rejection as non-fatal progressive-enhancement outcomes.

**Why it is useful**

Web Legado attempts to hold a wake lock during reading. The browser API provides a narrow equivalent without backend involvement.

**Estimated complexity:** Low.

**Likely ownership:** Reader lifecycle helper plus the existing Reader preference model and Settings/Reader controls.

---

### 6. Built-in Reader presets

**Current gap**

NovelReader already stores detailed Reader preferences, but users must adjust individual controls to move between common environments.

**Suggested first version**

Provide a small set of built-in presets that apply existing preference fields:

- Day.
- Night.
- Parchment.
- E-ink.
- Large text.

The first version should not introduce arbitrary user-created profile CRUD, cross-device synchronization, or backend persistence.

**Why it is useful**

It exposes the value of the existing preference model with much less complexity than Web Legado's full named-configuration system.

**Estimated complexity:** Low for fixed presets; moderate if later expanded to user-created profiles.

**Likely ownership:** `frontend/src/features/reader/reader-preferences.ts`, Reader settings, application Settings, translations, and unit tests.

## Other small convenience candidates

### Reader top and bottom actions

Possible actions:

- Scroll to chapter top.
- Scroll to chapter bottom.
- Return to the last persisted position after temporary navigation.

These should live in an overflow/action area rather than expanding the primary mobile control bar.

**Estimated complexity:** Very low.

### Automatically reveal the active source and current chapter

When opening source recovery or the TOC, bring the current row into view and preserve that relative location after local sorting/filtering where practical.

**Estimated complexity:** Very low.

### Latest-chapter context on alternate-source rows

Web Legado displays a candidate source's latest chapter beside its name. NovelReader should add this only if the backend can provide the value without extra hidden health checks or source requests. It must not introduce continuous background source monitoring.

**Estimated complexity:** Low only if metadata is already available; otherwise discuss the backend cost first.

## Valuable but moderate-complexity ideas

These are worth future product discussion but should not be treated as quick frontend additions.

### Browser speech synthesis

A useful implementation needs paragraph chunking, voice selection, pause/resume, active-paragraph tracking, browser lifecycle handling, and possibly chapter transitions. It should begin with browser `speechSynthesis`, not bundled proprietary/cloud voice behavior.

### Shelf groups

Requires a canonical group model, storage, CRUD endpoints, assignment behavior, deletion semantics, navigation/filtering, and migration of UI state.

### Manual book addition by source and URL

Requires a source selector and a backend contract that validates and resolves an arbitrary source-specific book URL into canonical preview information.

### BookSource export

Must preserve unknown source fields while explicitly excluding or safely handling login credentials, tokens, cookies, and per-reader source state.

### Cache/offline management

A clear product surface must distinguish remote-only content, server-cached chapters, and browser/device availability, including counts, freshness, refresh, and deletion ownership.

### Full-book text search

Requires a defined search corpus—downloaded/cache-only versus network fetch—plus progress, cancellation, result location, storage limits, and source-change semantics.

### Basic source dependency visibility

Web Legado shows which shelf books use a BookSource before deletion. This would be useful but requires a backend association query and a decision about whether deletion is blocked, warned, or allowed while stored books retain source snapshots.

## High-complexity or intentionally deferred areas

Do not treat these as minor features:

- BookSource-defined interactive `loginUi`, `loginUrl`, and `loginCheckJs` execution.
- Captcha or WAF interaction and browser-login session persistence.
- Automated source-health monitoring or broad source validation.
- User-defined text replacement/filter rules.
- Chapter editing and persistence semantics.
- User-defined Reader profile and keymap CRUD.
- Remote BookSource subscriptions.
- Backup, restore, and progress synchronization.
- EPUB, CBZ/comic, PDF, audio, and video workflows.
- A separate simple/e-ink frontend.
- Server-side or user-defined HTTP TTS.
- Advanced Android/JVM-only Legado APIs.

These involve durable domain state, security boundaries, untrusted source execution, new backend APIs, or separate media models.

## Reference behaviors intentionally not copied directly

Future work should preserve the useful behavior without reproducing these Web Legado weaknesses:

- Do not place every power-user tool in the root reading interface.
- Do not use punctuation-only expert syntax as the only way to scope Search.
- Do not expose generic backend exception strings as normal UX.
- Do not start continuous source-health monitoring; source failures should surface from user-triggered operations.
- Do not silently clear useful discovery, cache, or progress state.
- Do not use GET requests for mutations.
- Do not load format-specific media engines for ordinary text reading.
- Do not execute unrestricted source JavaScript with broad filesystem or process access.

## Planning guidance

Before promoting an item into active work:

1. Confirm it still solves an observed user problem.
2. Prefer a frontend-local implementation when existing backend contracts already provide the necessary data.
3. Keep state ownership explicit: tab-local interaction state, device-local preferences, and server-owned Reader Data should not be conflated.
4. Complete English, Simplified Chinese, and Traditional Chinese translations with the feature.
5. Add focused unit/component coverage and responsive browser verification.
6. Update `PLAN.md` only after the item is approved as committed scope.

## Source material

This shortlist was derived from:

- `reference/web-legado/docs/frontend-ux.md`
- `reference/web-legado/docs/behavior-assessment.md`
- `reference/web-legado/docs/live-user-flow-observations.md`
- Original Legado source behavior under `reference/legado/`
- NovelReader's current Vue frontend and backend contracts
- `PRODUCT.md`, especially progressive disclosure, reading-first navigation, and equal mobile/desktop support
