# Codebase Cleanup Audit

## Purpose

This document records confirmed and suspected dead, superseded, or remnant code that should be reviewed in a future maintainability cleanup.

It is an audit and cleanup backlog, not approval to delete everything listed here. Each removal must still prove that the symbol, endpoint, compatibility contract, test, or file is no longer required by the current product.

NovelReader is a pre-public application with a Vue-only production frontend. Historical migration scaffolding has no continuing value merely because it helped replace the archived Svelte frontend. The inert archive is sufficient migration evidence; active production modules should contain only current behavior and intentional compatibility contracts.

## Cleanup principles

1. **Current behavior is the contract.** Preserve implemented user journeys, current backend/domain contracts, authentication boundaries, Reader Data ownership, and deterministic compatibility tests.
2. **Migration history does not justify production code.** Remove adapters, barrels, wrappers, flags, tests, and dependencies that exist only to support superseded frontend or disposable-schema migrations.
3. **An unused frontend wrapper and an unused backend endpoint are different decisions.** A wrapper can be dead even when the endpoint remains an intentional HTTP contract.
4. **Do not trust unused-symbol tools without verification.** Vue templates, aliased imports, Pinia actions, type-only imports, dynamic routes, reflection, and test-only entry points can produce false positives.
5. **Preserved data is not implemented behavior.** A BookSource field or low-level runtime primitive does not constitute a complete product capability unless its execution, persistence, API, UI, security, and failure contracts exist end to end.
6. **Delete vertically.** When removing a superseded path, remove its implementation, exclusive DTOs/types, tests, translations, configuration, documentation, and dependencies together.
7. **Keep cleanup behavior-neutral.** Product or API changes discovered during cleanup require a separate decision rather than being folded silently into deletion work.
8. **Use focused proof.** For each removal, identify callers and contracts, add or retain the smallest relevant regression coverage, then run affected frontend and backend tests.

## Confirmed active-frontend cleanup candidates

The following exported wrappers have no callers in the active `frontend/src` production tree. References in `.migration/` are historical archive evidence and do not make a symbol active.

### Superseded book-entry wrappers

File: `frontend/src/api/books.ts`

- `addBook()`
- `addReadableBook()`
- `enrichBook()`

The production Search and Explore journeys use the canonical two-stage workflow:

1. `previewBook()` through `POST /api/books/preview`;
2. `shelveBook()` through `POST /api/books/shelve`.

This workflow preserves canonical logical-book identity and discovered alternate sources. The three older wrappers should be removed unless a current caller or independently supported client contract is identified during cleanup.

Do **not** classify these adjacent functions as dead:

- `mergeBookSources()` is used by source discovery/recovery persistence.
- `clearBookSources()` is used by explicit recovery reset.
- `deleteBook()` supports shelf removal.

### Superseded search wrappers

File: `frontend/src/api/search.ts`

- `searchBooks()`
- `searchBooksStream()`

The production Search and Source Recovery workflows use `searchBooksBatchStream()`, which owns cursor progression, continuation, retries, cancellation, stale-session handling, eligible-source progress, and partial failure reporting.

The older unbatched wrappers should be removed unless a current caller is found. Their exclusive callbacks, types, or tests should be removed in the same change.

## Backend endpoints requiring a compatibility decision

The backend currently exposes alternative or older workflow endpoints corresponding to some dead frontend wrappers:

- `POST /api/books`
- `POST /api/books/enrich`
- `POST /api/books/readable`
- `GET /api/search`
- legacy/unbatched use of `GET /api/search/stream`

These are not missing frontend features. The Vue application deliberately uses the canonical preview/shelve and batched-search paths instead.

Before deleting a backend endpoint, determine and document whether NovelReader's HTTP API is:

1. private to the bundled frontend;
2. an internal but stable automation boundary; or
3. a supported external-client contract.

If the API is private to the bundled frontend and no backend workflow or focused test requires the old handler, remove the endpoint, handler, exclusive request/response types, and legacy-only tests. If external compatibility matters, retain or deprecate the endpoint explicitly rather than leaving its status accidental.

## Known false-positive categories

A structural unused-export report previously marked several active functions as unused. Direct inspection showed real consumers:

| Capability | Active consumer |
|---|---|
| Initial Administrator setup | `frontend/src/features/account/SetupView.vue` |
| Password-reset completion | `frontend/src/features/account/PasswordResetView.vue` |
| Administrator recovery status and submission | `frontend/src/features/account/RecoveryView.vue` |
| Login and registration | `frontend/src/stores/session.ts` |
| Account password change | `frontend/src/features/account/AccountView.vue` |
| Reader-account administration | `frontend/src/features/account/ReaderAdministrationView.vue` |
| Font list/upload/delete/file URL | `frontend/src/features/settings/SettingsView.vue` |
| Source list/import/update/delete | `frontend/src/features/sources/SourceManagementView.vue` |
| Source switching and bookmarks | Stored Book Detail and Reader feature modules |

A future cleanup must verify candidates using multiple forms of evidence:

- AST caller/import search scoped to `frontend/src` or the relevant active tree;
- raw import/literal search where dynamic behavior is possible;
- router and Vue-template inspection;
- tests that import the symbol directly;
- backend route registration and handler tests;
- production build and browser verification after removal.

An export appearing in a tool's `unused` or `entrypoints` report is a lead, not proof.

## Backend capabilities that are not missing UI features

Some backend behavior is intentionally consumed through an existing workflow and does not require a separate screen.

### Automatic chapter cache fallback

The backend stores bounded processed chapter copies and can return one after live retrieval fails. Reader chapter-content responses expose whether an offline copy was used.

NovelReader does not currently expose contracts to enumerate, prefetch, size, or selectively clear chapter cache entries. A cache-management screen would therefore be a new product/API feature, not a hidden existing capability needing only a button.

### Chapter-image serving

`GET /api/books/{id}/chapters/{idx}/images/{imageIdx}` supports image blocks inside normal Reader chapter content. It is Reader infrastructure, not an independent image-management feature.

### Stored cover serving

The stored-book cover endpoint supports Shelf and Book Detail cover rendering. It is part of those existing journeys rather than a separate feature surface.

## Preserved or partial capabilities that are not complete products

### BookSource login fields

The canonical source model preserves `loginUrl`, `loginUi`, and `loginCheckJs`, and parts of `SourceSession` can hold cookies and variables. NovelReader does **not** yet have a complete durable per-reader/per-source login capability spanning safe credential storage, source-defined UI execution, browser interactions, login checks, and all Search/Explore/Book/Reader workflows.

Therefore, source login must not be described as a completed backend capability lacking only frontend UI. It remains deferred architecture and product work.

The same rule applies to advanced Legado fields and JavaScript bridge primitives: successful import or partial low-level support does not establish end-to-end compatibility.

## Additional cleanup areas to audit

The following areas warrant later evidence-based inspection but are not yet confirmed deletion lists:

### Frontend

- Temporary compatibility barrels or re-exports left after the domain API split.
- Types exported only for superseded wrappers.
- Route, component, translation, or style remnants from replaced placeholder screens.
- Duplicate view-model helpers superseded by shared Book Detail/Reader modules.
- Tests that only pin removed migration behavior rather than current production contracts.
- Dependencies present only for the archived Svelte runtime or discarded prototype paths.

### Backend

- Alternative book-ingestion handlers after the HTTP compatibility decision.
- Unbatched search handlers and DTOs after the HTTP compatibility decision.
- Cutover-only ownership adapters, flags, paths, tables, and direct-global-store tests already tracked by the storage cleanup gate.
- Compatibility shims for disposable pre-public schemas.
- Dormant handlers or helpers whose only consumers are legacy-only tests.
- Duplicate domain logic bypassing the canonical `sourceexec`, `book`, `readerstore`, or authenticated runtime seams.

### Repository and tooling

- Production-unreachable archived framework dependencies.
- Generated or experimental files accidentally tracked outside their intended ignored locations.
- Obsolete scripts, environment variables, compose options, and documentation for removed startup paths.
- Stale comments and TODOs that describe completed or abandoned migration work.

## Proposed cleanup sequence

### Phase A — Inventory and contract classification

- Freeze the cleanup baseline at a commit.
- Generate frontend and backend symbol/route inventories.
- Classify each candidate as active, test-only, public contract, internal contract, archived, generated, or dead.
- Decide whether the backend HTTP API is private or externally supported before deleting handlers.
- Record false positives and dynamic entry points so later tools do not rediscover them.

### Phase B — Frontend remnants

- Remove confirmed unused Vue transport wrappers, exclusive types, and tests.
- Remove temporary API barrels once no active import depends on them.
- Remove obsolete framework dependencies and configuration after proving the production build is Vue-only.
- Run typecheck, lint, focused tests, the full frontend test command, and production build.

### Phase C — Backend workflow remnants

- Remove only endpoints whose compatibility status has been explicitly resolved.
- Remove handlers, DTOs, domain branches, and exclusive tests vertically.
- Verify focused API and domain packages before broader backend tests.
- Confirm current Vue workflows through browser checks where route behavior changed.

### Phase D — Repository and documentation cleanup

- Remove obsolete scripts/configuration and update README/PLAN documentation.
- Verify clean fresh setup using current documented commands.
- Run dead-code and duplicate-code inspection again and classify remaining findings.
- Record any intentionally retained compatibility surface and its owner.

## Completion criteria

The cleanup is complete when:

- every retained production module has a current caller, contract, or documented reason to exist;
- no Svelte runtime, mixed-framework build path, or migration-only frontend adapter remains active;
- no disposable-schema migration or global Reader Data fallback remains;
- duplicate/superseded API workflows are removed or explicitly retained as supported contracts;
- tests describe current behavior rather than historical migration paths;
- setup, typecheck, lint, targeted tests, appropriate broader tests, production build, and affected browser flows pass;
- `PLAN.md`, README, and operational configuration describe only current supported behavior.

## Current status

Documentation only. No cleanup deletion has been approved or performed as part of this audit.

Confirmed initial frontend candidates:

- `addBook()`
- `addReadableBook()`
- `enrichBook()`
- `searchBooks()`
- `searchBooksStream()`

Backend endpoint removal remains blocked on an explicit HTTP compatibility decision.
