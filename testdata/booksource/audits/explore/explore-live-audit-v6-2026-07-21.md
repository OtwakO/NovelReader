# Explore live audit v6 — 2026-07-21

## Scope

- 50 enabled Explore identities from `test_booksource4.json`
- Unrestricted deterministic ranking with seed `NovelReader-explore-random-v6-2026-07-21`
- Disjoint from all 225 stable identities in batches 1–5
- Fresh production database, first selectable category, page 1, 90-second timeout
- Four-client initial pass; all 28 initial non-passes rerun sequentially
- No source, parser, transport, or bridge behavior changed during the audit

## Results

| Classification | Sources |
|---|---:|
| Credible non-empty | 23 |
| Shared engine gap | 6 |
| Stale source contract | 7 |
| Upstream HTTP/DNS | 6 |
| Blocked or authentication required | 3 |
| Incomplete or invalid source | 3 |
| Site drift | 2 |

The 23 credible sources returned **488 books with 488 distinct book URLs**. Raw 346 failed only during the concurrent pass and returned 20 books sequentially.

## Shared engine gaps

1. **Redirected source headers are lost or rejected — raw 112 and 311**
   - Production returns `http_status` for both first pages.
   - Direct requests using each source's exact imported headers follow the current redirects and return HTTP 200 with 40 matching rows.
   - The two independent sources expose the same reusable transport/header/redirect seam.

2. **Missing `source.setVariable` — raw 933**
   - Qidian catalog JavaScript fails before producing categories because the source bridge exposes `putVariable` but not the used `setVariable` alias.
   - The current Qidian rank endpoint returns populated records.

3. **Incomplete Jsoup selection compatibility — raw 749 and 301**
   - Raw 749 calls Java-compatible `selection.size()` but the bridge exposes numeric `size`, causing `TypeError` despite seven live matching links.
   - Raw 301 calls `selection.eq(index)`, which is absent; both source pages are live and contain the expected row groups.

4. **Nullable optional JSON field aborts a valid page — raw 757**
   - The live API returns eight books.
   - Fallback extraction applies the source's search rules, but a missing/null `lastChapterInfo` value aborts the entire page instead of yielding an empty optional field.

## Non-engine outcomes

- **Stale contracts:** raw 22, 222, 406, 678, 750, 908, 247. Live redirects, URL patterns, selectors, or endpoints no longer agree with the imported source.
- **Upstream failures:** raw 700 DNS; raw 138, 411, 508, 817, 912 HTTP/TLS/timeout failures.
- **Blocked/auth:** raw 857 returns nginx 444, raw 223 returns 403, raw 169 redirects to login.
- **Incomplete/invalid:** raw 37 needs missing app/device bootstrap state, raw 71 has malformed JavaScript, raw 164 has no `exploreUrl` despite `enabledExplore`.
- **Site drift:** raw 369 redirects to generic iQiyi content; raw 625's retired Duokan rank contract no longer returns populated books.

## Recommendation

Fix the four shared seams test-first, verify all six identities and adjacent corpus users, then run one final **25-source** disjoint sample biased toward redirected custom headers, source-variable methods, Jsoup collection methods, and nullable JSON fields. Stop broad random sampling if that targeted batch finds no new shared family.
