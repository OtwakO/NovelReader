# Clean-data-root account-shell verification — 2026-08-13

## Result

The required Playwright ownership workflow passed against a disposable empty data root. This verifies the replacement per-reader path end to end; it does not complete the separate Phase 2 legacy-removal audit. A machine-readable observation record is stored beside this report in [`ACCOUNT_SHELL_CLEAN_ROOT_2026-08-13.json`](ACCOUNT_SHELL_CLEAN_ROOT_2026-08-13.json).

## Environment

- Branch: `main` (`git status --short --branch` reported `main...origin/main` before the run).
- Server: `http://127.0.0.1:8894`
- Data root: `backend/.e2e/clean-account-workflow/data`, deleted and recreated before startup.
- Public origin pinned to `http://127.0.0.1:8894`.
- Source corpus: repository `test_booksource4.json`.
- Browser driver: `playwright-cli`, named session `account-e2e`.
- Query: `凡人修仙传`.
- Selected real result: `凡人修仙传仙界篇`, author `忘语`, source `365小说网`.

## Browser evidence

1. A fresh server opened the first-Administrator setup screen. Setup created `E2EAdmin` and navigated to the empty shelf.
2. The Administrator signed out, reached `#/login`, signed in with the new credential, and returned to `#/shelf`.
3. Sources initially showed “No book sources installed.” The browser file chooser imported `test_booksource4.json`; the UI alert reported `Imported 939 sources`.
4. The first 50-source live search batch checked 50 of 800 eligible sources and returned 102 books. Thirteen sources succeeded and 37 produced explicit upstream/source diagnostics.
5. Adding `凡人修仙传仙界篇` navigated to its stored detail page. The page showed source `365小说网`, full description, and `Chapters (1412)`.
6. Opening `第一章 狐女` navigated to chapter index 2 and rendered genuine Chinese chapter paragraphs beginning `辽阔荒地，渺无人烟`.
7. Returning to the shelf showed the persisted book and reading state `Ch.3 / 1412`.
8. Before logout, browser fetches returned HTTP 200 for `/api/books`, `/api/sources`, and `/api/fonts`.
9. After logout, the same browser context received HTTP 401 with `{"error":"authentication required"}` from `/api/books`, `/api/sources`, and `/api/fonts`.
10. Direct navigation to `#/shelf` after logout rendered the “Welcome back” login UI and no private shelf content.

## Storage evidence

The clean root contained only system ownership data at its root (`system.db`, `storage.json`, lock files) and one immutable-ID user directory under `users/`. Reader Data was stored in that user's `reader.db` and `files/` subtree. No root-level `novelreader.db` was created.

## Diagnostics and limitations

- Live search was network-dependent: 37 of the first 50 sources reported explicit DNS, timeout, HTTP, WebView, stale-rule, or unsupported-helper failures. This did not affect the selected 365小说网 path, which completed detail → TOC → real content.
- The server logged the existing `InsecureSkipVerify` transport warning. It did not affect ownership verification.
- Browser console errors for the deliberate post-logout API probes were the expected 401 responses.
- An exploratory request to nonexistent `/api/settings` returned 404; it was not part of the required gate. `/api/auth/me` was also guessed incorrectly during a post-logout probe and returned 404. Neither response exposed Reader Data.
- This is recorded Playwright CLI evidence, not a new committed automated E2E test.

## Independent review

A focused reviewer independently assessed the artifacts. It agreed that the Playwright ownership-workflow sub-gate passed and that no observed console or server issue invalidated it. The reviewer correctly distinguished this result from the still-pending Phase 2 legacy-removal checks.
