# Explore live audit v10 — 2026-07-23

## Scope

- **Sample:** 50 unrestricted enabled Explore identities
- **Seed:** `NovelReader-explore-random-v10-2026-07-23`
- **Corpus:** `test_booksource4.json`, SHA-256 `23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c`
- **Excluded:** all 400 stable identities from audits 1–9
- **Eligible before selection:** 321
- **Ranking:** `SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)`
- **Execution:** fresh isolated SQLite database, unmodified corpus, production Explore API, four-client initial pass, 90-second timeout, sequential rerun of every non-pass

No identity was substituted and no source/parser behavior changed during sampling.

## Results

| Classification | Count |
|---|---:|
| `credible_nonempty` | 37 |
| `engine_gap` | 1 |
| `stale_source_contract` | 6 |
| `upstream_http` | 4 |
| `blocked_or_auth` | 1 |
| `source_incomplete_or_invalid` | 1 |

The 37 credible sources returned **1,172 books with distinct book URLs**. All 13 initial non-passes reproduced sequentially.

## Shared compatibility gap

### Partial `ruleExplore` object fallback — raw 927

The imported source has a complete `ruleSearch`, but its `ruleExplore` object contains only `coverUrl`. Legado's Explore contract uses the complete search-rule object whenever the Explore `bookList` is blank. The current live page returns HTTP 200 and contains 28 valid `ul.mh-list.col7 > li` cards with book links, but NovelReader treats the partial Explore object as complete and returns zero books.

This is a shared source-selection seam rather than a site-specific selector issue: any source with a partial `ruleExplore` object and blank `bookList` can lose the required search rules.

## Other confirmed outcomes

- **Stale contracts (6):** removed iQiyi manga section; two changed hosts/routes; two stale `bookUrlPattern` constraints; one stale HTTPS API scheme.
- **Upstream HTTP (4):** Cloudflare 521, Cloudflare 522 on the production request path, a redirect loop, and a live DedeCMS database error page.
- **Blocked/auth (1):** signed API returns HTTP 401 Unauthorized.
- **Invalid source (1):** catalog JavaScript contains malformed destructured arrow parameters rejected by both goja and Node.

## Recommendation

Fix only the shared partial-`ruleExplore` fallback seam through the existing Explore rule-selection boundary, with a regression proving that blank Explore `bookList` selects the whole `ruleSearch` object rather than merging fields. Do not weaken URL-pattern enforcement or patch stale source definitions as engine behavior.
