# Explore live compatibility audit — batch 3

Date: 2026-07-19

Corpus: `test_booksource4.json` (SHA-256 `23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c`, 939 sources)

Seed: `NovelReader-explore-audit-v3-2026-07-19`

## Method

This audit used a deterministic random 50-source sample of unique Explore-capable source URLs, excluding all 100 raw indices from the first two audits. Candidates were ranked by SHA-256 of the seed, raw index, and source URL; the first 50 were selected. Each source ran against a fresh isolated database with the current engine, using its first selectable URL category and page 1.

Every empty/error result and the suspicious single result were independently checked with Playwright direct requests and targeted live DOM selector counts. Raw 674’s initial transport timeout was rerun through the production transport and returned 12 distinct books. No raw BookSource or production parser behavior changed during the audit.

Live-site results are time-sensitive. This random sample is a compatibility signal for the remaining eligible corpus, not a deterministic regression test.

## Summary

| Classification | Sources |
|---|---:|
| Credible non-empty pass | 25 |
| Shared engine compatibility gap | 6 |
| Stale, unavailable, blocked, or authentication-dependent upstream | 14 |
| Invalid or incomplete raw Explore contract | 4 |
| Legitimate empty upstream response | 1 |
| **Total** | **50** |

The engine returned credible distinct books for **25/50 (50%)**. Six failures were attributable to shared engine behavior rather than source-specific rules. Fourteen were external drift or access failures, four raw sources lacked a valid executable Explore contract, and one API response was genuinely empty.

## Credible passes

Raw indices **144, 147, 266, 399, 401, 417, 447, 475, 482, 607, 639, 674, 735, 743, 762, 768, 772, 812, 827, 834, 847, 865, 915, 916, and 921** returned credible non-empty results. Counts ranged from 5 to 100 distinct books, except raw 674’s 12-book production rerun.

## Confirmed shared engine gaps

| Raw | Source | Live evidence | Gap |
|---:|---|---|---|
| 726 | 鲤鱼乡 | 2,432 matching `.m-list-top ul li` rows | The 2,000 retained-result safety ceiling rejects the whole page instead of returning a bounded result |
| 788 | 玩文学网 | 20 current data rows | `{{$.coverUrl}}` against HTML aborts with a JSON decode error instead of allowing the literal fallback |
| 462 | 绾书文学🎃 | Valid JSON response | Go regexp rejects Java’s `\h` horizontal-whitespace escape in `lastChapter` |
| 305 | 🔰 太子爷 | 16 current list rows | Go regexp rejects Java-compatible escaped Chinese title punctuation in `name` |
| 215 | 晋江文学 | 100 current data rows | Missing Legado `java.getElement` helper stops field extraction |
| 737 | 乡土小说🎃 | 10 distinct table rows | `tbody@tag.tr` returns only the first row when traversal crosses multiple parent elements |

These are generic compatibility families: bounded large-result handling, tolerant mixed-data templates, Java-regex parity, one Java bridge helper, and multi-parent Default traversal. None requires a source-specific patch.

## Invalid or incomplete raw contracts

- **Raw 365 — 闲看🎃:** category JavaScript contains invalid unparenthesized destructuring arrow parameters; its API also returns `params exception`.
- **Raw 81 — 笔下文学（优+）:** `ruleExplore` is empty.
- **Raw 55 — 古诗文网（优+）:** `ruleExplore` is empty despite a live page with 15 content cards.
- **Raw 534 — 黑岩小说:** all 100 extracted URLs duplicate two absolute prefixes (for example, `https://www.heiyan.com/book/https://www.heiyan.com/chapter/161190/`), so distinctness does not make them usable books.

## External drift, access, and authentication failures

- **DNS/unavailable/status:** raw 199 no longer resolves; raw 652 and 851 return Cloudflare 521; raw 511 returns 502; raw 841 returns 404; raw 730 repeatedly times out.
- **WAF/authentication:** raw 174 returns Baidu `errno -6`; raw 761 returns a Cloudflare 403 challenge; raw 185 is blocked for the engine transport while a browser request succeeds, but the source does not request WebView.
- **Stale selectors or URL patterns:** raw 630 lost the image `title` attribute; raw 602’s URLs no longer contain `book_`; raw 789’s `td.0` name selector is empty; raw 116 redirects to a layout matching none of its list selectors; raw 760 redirects to `qudushu.la` while its pattern still requires `qudushu.com`.

Raw **532** (`多看阅读🎃`) is the legitimate empty response: HTTP 200 JSON explicitly reports `total: 0` and `items: []`.

## Recommended next shared fixes

1. Correct Default traversal across multiple parents; raw 737 is a focused regression case.
2. Add Java-regex compatibility for accepted identity escapes and `\h`; this repairs raw 305 and 462 through one shared boundary.
3. Make mixed HTML/JSON template fallback non-fatal; raw 788 exercises the same compatibility family seen in earlier audits.
4. Decide an explicit large Explore-page policy for raw 726: return the first safe bounded set rather than fail the entire page.
5. Implement the focused `java.getElement` bridge helper used by raw 215.

Machine-readable evidence, including all identities, raw rules, engine outcomes, direct responses, and DOM counts, is in [`explore-live-audit-v3-2026-07-19.json`](explore-live-audit-v3-2026-07-19.json).
