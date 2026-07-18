# Explore live compatibility audit — batch 2

Date: 2026-07-19

Corpus: `test_booksource4.json` (SHA-256 `23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c`, 939 sources)

Seed: `NovelReader-explore-audit-v2`

## Method

This audit used a new deterministic 50-source sample disjoint from the first audit. It excluded the first 50 raw indices, malformed top-level catalogs, and duplicate source URLs, then stratified the remaining candidates across literal/legacy/JavaScript catalogs, GET/POST/page selector/charset/WebView request shapes, and Default/CSS/JSONPath/XPath/Regex/JavaScript result rules. Every identity is recorded by raw index and source URL because display names are not unique.

Each source ran against a fresh isolated database using the engine containing only the previously approved shared fixes. The audit selected the first usable category and requested page 1. Every empty/error result and each suspicious single result was rerun through a recording production transport. Playwright independently requested recorded page URLs and targeted dynamic-category endpoints, including selector-specific counts for raw 209, 342, 516, and 614. The mobile Explore UI was also checked at 390×844 for representative engine failures. No production parser behavior or raw BookSource rule changed during this audit.

The sample is deliberately stratified toward uncommon rule shapes, so its percentages are compatibility signals, not a prevalence estimate for all 939 sources.

## Summary

| Classification | Sources |
|---|---:|
| Credible non-empty pass | 18 |
| Shared analyzer/result-engine gap | 10 |
| Java/JavaScript bridge gap | 4 |
| Stale, unavailable, blocked, or authentication-dependent upstream | 17 |
| Legitimate empty upstream response | 1 |
| **Total** | **50** |

The first approved URL-resolution and Default-detection fixes held: none of this batch regressed those corrected shapes. The second batch instead exposed four recurring compatibility families: mixed-mode fallback handling, incomplete Default traversal/exclusion syntax, Legado interpolation semantics, and Java bridge coverage.

## Credible passes

Raw indices **10, 94, 181, 201, 231, 232, 282, 299, 358, 391, 403, 413, 451, 470, 658, 661, 780, and 807** returned credible results with distinct book URLs. Counts ranged from one legitimate upstream album (raw 181) to 153 distinct books (raw 282).

## Confirmed shared engine gaps

| Raw | Source | Engine | Direct evidence | Gap |
|---:|---|---:|---|---|
| 76 | 轻之文库（优+） | Error | Valid JSON has 10 books | Invalid HTML branch `a.0@text` aborts before JSON fallback `name` |
| 82 | 中文万维（优+） | Error | Valid JSON has 20 books | Single-brace `{$.grade}` interpolation is treated as JSONPath |
| 204 | 企鹅阅读 | Error | Valid JSON has 20 books | Missing Legado `java.toNumChapter` helper |
| 209 | 哎爱巴士 | Catalog error | Category page is HTTP 200 | Missing `java.toast` stops dynamic Jsoup category generation |
| 342 | 人文书库 | 0 | Live `#articlelist` has 29 list items | Raw `li!0` exclusion suffix is unsupported |
| 410 | 中文万维🎃 | Error | Valid JSON has 20 books | Same interpolation gap as raw 82 |
| 429 | 疯读小说 | Error | Valid JSON has 10 books | `{$.crazy_rating}` interpolation is treated as JSONPath |
| 471 | 疯读小说🎃 | Error | Valid JSON has 10 books | Same interpolation gap as raw 429 |
| 516 | 💰小说阅读网 | 1 concatenated | Live page has 20 books | `.right-book-list@tag.li` is misrouted instead of Default traversal |
| 614 | 云轩阁小说网 | 1 | Live page has 20 cards | `.box@.col-12` chained class traversal is misrouted |
| 742 | 去读书🎃#2 | Error | Live page has 15 `c_row` nodes | Trailing empty `&&` branches abort field extraction |
| 816 | 龙腾小说城 | Error | Live page has 20 `.articlegeneral` nodes | Optional `$.thumbnail` plus JS fallback fatally JSON-decodes HTML |
| 920 | 📚豆瓣阅读 | Error | Initial JSON is valid with 3 featured IDs | Raw rule requests GraphQL POST through a Legado URL-options suffix, but engine `java.ajax` only performs GET and does not parse those options |
| 134 | 蓝批漫画（优+++） | Catalog error | Script requires Pixiv state/login | Advanced cache/helper and authenticated browser bridge APIs are absent |

The UI faithfully reflected the API: raw 516 displayed one book, and raw 82 displayed `Could not parse Explore results`. The only browser-console error was the expected failed `/api/explore/page` HTTP 502; there was no separate frontend parsing failure.

## Stale, blocked, or authentication-dependent sources

| Raw | Source | Evidence |
|---:|---|---|
| 3 | 起点读书X-QD | Dynamic category host `static.yesui.me` no longer resolves |
| 40 | 轻说机翻（优+） | First selectable private-history endpoint returns HTTP 401 |
| 46 | 中华典藏（优+） | Redirects to `diancang.xyz` with 19 books, but raw `bookUrlPattern` still requires the old domain |
| 61 | 三七小说（优+） | Category domain is parked |
| 97 | 无线电子（优） | Upstream returns a self-referential HTTP 301 loop |
| 123 | 爱去小说（导） | Redirected site changed layout/domain while the raw pattern still requires `279txt.com` |
| 165 | 海洋听书（优） | Category returns a JavaScript redirect/lander |
| 173 | 坚果云盘（优++） | Private endpoint returns HTTP 401 without a valid cookie |
| 175 | 微博评论（优+） | API redirects to Weibo's visitor/login system |
| 183 | 青花鱼评（优） | Hard-coded IP/path returns HTTP 404 |
| 414, 426, 439 | Three 晋江 identities | JSON contains 6 books, but the raw pattern expects digits immediately after `bookX`; live URLs are `book2/<id>` |
| 437, 537 | Two `fxsw.top` identities | Host no longer resolves |
| 629 | ️夜寒 | Dynamic catalog's `/rank` and `/full-1/` endpoints both return HTTP 404 |
| 787 | 🔞18文学 | TLS handshake fails in both engine and Playwright |

Raw **146** (`曼哈漫画（优+）`) is the one legitimate empty result: its raw POST request succeeds with HTTP 200 and an empty response body.

## Ranked next fix candidates

1. **Complete Default traversal and exclusion syntax** — `@tag.*`, `@.class`, and `!index`; directly affects raw 342, 516, and 614 and recurs from the first audit.
2. **Implement Legado interpolation and tolerant connector branches** — single-brace JSON interpolation plus empty/incompatible `||` and `&&` branches; directly affects raw 76, 82, 410, 429, 471, and 742.
3. **Make optional mixed-mode fields non-fatal** — raw 816 should let an empty JSONPath feed its JS default rather than failing the whole page.
4. **Add small, verified Java helpers separately from advanced bridge work** — `toNumChapter` is a focused helper; raw 134, 209, and 920 require broader stateful/network/browser semantics and should not be approximated with source-specific no-ops.

Full machine-readable evidence, including source identities, engine diagnostics, expanded requests, response metadata, and Playwright observations, is in [`explore-live-audit-v2-2026-07-19.json`](explore-live-audit-v2-2026-07-19.json).
