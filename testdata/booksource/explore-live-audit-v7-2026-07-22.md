# Explore live audit v7 — 2026-07-22

## Scope

- 25 targeted identities from `test_booksource4.json`, disjoint from all 275 identities in batches 1–6
- Seed: `NovelReader-explore-targeted-v7-2026-07-22`
- Strata: 6 redirected/custom-header, 6 Jsoup collection, 3 remaining source-variable, 2 other catalog-JavaScript, 4 nullable JSON, and 4 controls
- Only three unsampled source-variable identities remained, so all three were selected and two planned slots moved to catalog JavaScript
- Fresh production database, first selectable category/page 1, four-client initial pass, 90-second timeout
- All 13 initial non-passes rerun sequentially
- No source, parser, bridge, or transport behavior changed during the audit

## Results

| Classification | Sources |
|---|---:|
| Credible non-empty | 12 |
| Shared engine gap | 4 |
| Blocked or authentication required | 3 |
| Upstream HTTP/DNS | 3 |
| Site drift | 1 |
| Legitimate empty | 1 |
| Incomplete or invalid source | 1 |

The 12 credible sources returned **274 books with 274 distinct book URLs**.

## Shared engine gaps

1. **Full source metadata missing from rule JavaScript — raw 177**
   - The live Yuque page contains populated encoded `appData` matching the imported rule.
   - The rule evaluates `source.bookSourceComment`; NovelReader's `source` binding exposes variable/helper methods but not source metadata.

2. **Executable source headers are not evaluated — raw 5**
   - The source header is `@js:` returning JSON and depends on `source.bookSourceUrl`.
   - Header parsing accepts strict or quoted literal maps only, so this valid executable contract is dropped. Evaluated headers can return populated endpoint JSON.

3. **JSON object wildcard/filter list selection — raw 503**
   - The live endpoint returns populated object entries containing `novelName`/`novelId`.
   - A reduced production Analyzer probe confirms `@Json:$.*[?(@.novelName)]` returns no elements while `$.*` returns the objects.

4. **Jsoup `ownText()` method parity — raw 288**
   - Both live category pages return HTTP 200 with 100 tag and 16 category links.
   - Valid catalog JavaScript calls `a.ownText().trim()`, but the bridge exposes `ownText` as a string property rather than a method.

## Other outcomes

- **Blocked/auth:** raw 194 requires a Qidian CSRF cookie; raw 666 and 913 return Cloudflare 403 challenges.
- **Upstream:** raw 337 times out, raw 41 fails DNS, and raw 407 returns HTTP 503.
- **Site drift:** raw 300's imported ranking page no longer contains the declared result rows.
- **Legitimate empty:** raw 605's first rank returns valid empty JSON; another category returns a valid book.
- **Invalid source:** raw 523 contains malformed destructuring arrow-function syntax.

## Recommendation

Fix the four shared seams test-first at their reusable boundaries, verify the four identities and every adjacent corpus consumer, then run one final targeted 25-source disjoint batch. If that batch finds no new shared family, stop broad Explore auditing.
