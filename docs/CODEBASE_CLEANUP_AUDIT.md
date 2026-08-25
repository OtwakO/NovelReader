# Codebase Cleanup Audit

## Purpose

This document records the evidence and outcome of the cleanup completed alongside the asynchronous candidate-resolution architecture. NovelReader is pre-public, and its HTTP API is currently private to the bundled frontend and repository-owned tests. Superseded internal HTTP workflows therefore do not receive compatibility preservation unless explicitly designated as supported contracts.

## Cleanup principles

1. Current user journeys and domain behavior are the contract.
2. Migration history does not justify active production code.
3. Remove superseded workflows vertically: route, handler, DTO, client wrapper, test, translation, and documentation.
4. Treat unused-symbol reports as leads; verify Vue templates, dynamic routes, test fixtures, and protocol entry points directly.
5. Preserve domain methods when current Reader, source-switching, storage, or compatibility behavior still uses them independently of a deleted HTTP route.

## Completed cleanup

### Candidate and shelf ingestion

The asynchronous `/api/candidate-resolutions` operation is the sole Search/Explore candidate-validation and shelf-ingestion boundary. The following redundant routes and their exclusive handlers, DTOs, helpers, and tests were removed:

- `POST /api/books`
- `POST /api/books/enrich`
- `POST /api/books/preview`
- `POST /api/books/shelve`
- `POST /api/books/readable`

The removed paths duplicated subsets of Book Info, TOC, content validation, and persistence while bypassing operation progress, cancellation, bounded concurrency, runtime-lease ownership, or readability guarantees.

The frontend wrappers `addBook`, `previewBook`, `shelveBook`, `addReadableBook`, and `enrichBook` and their exclusive response types/payload mapper were removed. Candidate input and selection types now live in the candidate feature rather than the stored-books API module.

### Search transport

The active frontend uses only batched `GET /api/search/stream` with cursor, batch-size, concurrency, retry, stale-session, and partial-failure semantics. The following remnants were removed:

- frontend `searchBooks()`;
- frontend unbatched `searchBooksStream()`;
- backend `GET /api/search`;
- the legacy unbatched branch and merged-result event contract in `GET /api/search/stream`;
- the pass-through Search handler left after that branch was removed.

The raw-source API E2E now verifies the current path: source import → batched Search SSE → candidate resolution → explicit shelf commit → first/middle/last chapter reading.

### Tests and feature ownership

- Source-switch tests now create stored domain state directly instead of depending on a removed ingestion endpoint.
- Candidate-operation test fixtures are colocated with active operation tests.
- The candidate client test moved from `src/api/books.test.ts` to the feature-local candidate module.
- Candidate-only DTOs no longer live in `src/api/books.ts`.

## Intentionally retained adjacent code

The following are active and must not be classified as remnants:

- `mergeBookSources()` persists discovered alternate sources.
- `clearBookSources()` performs explicit source-recovery reset.
- `deleteBook()` removes stored shelf entries.
- `listBooks()` and `getBook()` serve stored-book views.
- `searchBooksBatchStream()` is the active Search and Source Recovery transport.
- `Searcher.GetBookInfoForBook`, `GetChapterListForBook`, and `GetChapterContentForBook` remain valid timeout-owning APIs for active stored-book/Reader workflows and tests; candidate operations use their caller-context variants.
- `book.PreviewBook` remains the narrow non-persistent candidate response shape used by asynchronous operations.

## Known false-positive categories

Structural unused-export reports may miss Vue component/template consumers or flag dynamic entry points. Verify candidates through imports, route registration, templates, tests, exact literals, typecheck, and production build before removal.

Examples of active capabilities previously reported incorrectly include account setup/recovery, font management, source management, source switching, bookmarks, and candidate-operation helpers imported by `.vue` files.

## Remaining cleanup boundaries

These were not part of the candidate-resolution cleanup and require their own evidence and decision:

- ownership cutover adapters and legacy storage paths tracked in `docs/USER_STORAGE_IMPLEMENTATION_TASKS.md`;
- archived framework dependencies or generated artifacts outside active production reach;
- obsolete scripts/configuration unrelated to candidate resolution;
- partial BookSource capabilities that are preserved as data but do not form complete products.

## Verification standard

The cleanup is considered complete for this scope when:

- no removed route or wrapper symbol remains in active source;
- current Search, candidate detail, direct shelf addition, source switching, and Reader contracts compile;
- candidate/API race tests pass;
- focused frontend operation/component/i18n tests pass;
- frontend typecheck, lint, and production build pass;
- `git diff --check` passes;
- project documentation describes only the asynchronous candidate-operation ingestion path.
