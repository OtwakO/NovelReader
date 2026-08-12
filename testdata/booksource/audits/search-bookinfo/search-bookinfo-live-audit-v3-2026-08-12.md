# Search → Book Info live audit v3 — 2026-08-12

## Scope

- Corpus: `test_booksource3.json`, SHA-256 `9928f8b217126804ea5ef9524d919f05cfafddfeff51ef9691e0edc308e13cdb`, 458 entries.
- Deterministic unrestricted sample: 50, disjoint from all 50 v1/v2 identities; 279 eligible identities remained before selection.
- Seed: `NovelReader-search-bookinfo-random-v3-2026-08-12`; identity: `(rawIndex, bookSourceUrl)`.
- Query: `凡人修仙传`; page 1 only.
- Workflow: production Search, then Book Info for the first credible result; stop before TOC/content.
- Exactly the 50 frozen raw definitions were executed, preventing duplicate runtime keys from replacing sampled contracts.
- Initial concurrency: four; all 39 non-passes replayed sequentially.
- No source or parser behavior changed during the audit.

## Result

- 11/50 completed Search and Book Info with usable data.
- 2/50 are primarily affected by two confirmed recoverable shared compatibility gaps.
- 4/50 explicitly require deferred WebView transport; one additional source is primarily a deferred Rhino/JVM dependency.
- Remaining outcomes are blocked/authentication, legitimate empty pages, upstream transport/DNS/HTTP failures, source/site drift, or an invalid source contract.

| Raw index | Source | Classification | Evidence-backed rationale |
|---:|---|---|---|
| 361 | 起点中文（优+） | `credible_search_and_detail` | Production Search returned 20 books and Book Info enriched the selected result. |
| 371 | 书旗小说 | `upstream_timeout` | The exact authored request timed out in sequential production replay and direct curl before any HTTP response. |
| 37 | 七猫小说（优++） | `credible_search_and_detail` | Production Search returned 10 books and Book Info enriched the selected result. |
| 418 | 若初文学 | `upstream_timeout` | The exact authored request timed out in sequential production replay and direct curl before any HTTP response. |
| 14 | 猫眼看书（优++） | `credible_search_and_detail` | Production Search returned 15 books and Book Info enriched the selected result. |
| 130 | 过期杂志（优+） | `deferred_webview` | The source explicitly requests WebView behavior, outside the current regular/JavaScript transport target. |
| 375 | 七点小说 | `shared_engine_gap` | Standalone OnlyOne regex replacement preserved surrounding outer HTML, corrupting bookUrl and coverUrl. Changing only the two URL-field expressions to equivalent first-match replacement semantics recovered 20 Search results and successful Book Info. |
| 123 | 秋风书屋（优+） | `blocked_or_auth` | The exact route returns Cloudflare HTTP 403; Chromium remains on the security-verification page. |
| 57 | 阅友小说（优+） | `upstream_timeout` | The exact authored request timed out in sequential production replay and direct curl before any HTTP response. |
| 414 | 起点中文 | `deferred_webview` | The source explicitly requests WebView behavior, outside the current regular/JavaScript transport target. |
| 158 | 五五读书（优） | `upstream_timeout` | The exact authored request repeatedly timed out after redirect before a usable response. |
| 349 | 百度图片（优） | `credible_search_and_detail` | Production Search returned one result and Book Info completed successfully. |
| 180 | 大美书网（优） | `legitimate_empty` | The exact GBK POST returns HTTP 200 but no source-authored result-list structure for the query. |
| 118 | 爱久久网（优+） | `legitimate_empty` | The live HTTP 200 response explicitly says no related content was found. |
| 102 | 八三中文（优+） | `upstream_transport` | The exact authored HTTPS endpoint fails TLS negotiation in production and direct curl. |
| 340 | 青花鱼评（优） | `upstream_http` | The exact authored search route repeatedly returns HTTP 404. |
| 135 | 多多书院（优） | `upstream_http` | The exact authored search route repeatedly returns HTTP 400 with an empty body. |
| 355 | 百度贴吧（优） | `blocked_or_auth` | The exact Tieba route returns HTTP 403 and Chromium remains on 百度安全验证. |
| 160 | 爱下电子（优） | `credible_search_and_detail` | Production Search returned 20 books and Book Info enriched the selected result. |
| 51 | 西瓜免费（优+） | `deferred_rhino_jvm` | The source requires Rhino/JVM APIs including JavaImporter, Packages.javax.crypto, and Java byte arrays; regular goja execution cannot provide that JVM surface. |
| 27 | 立方体儿（优++） | `credible_search_and_detail` | Production Search returned 20 books and Book Info completed successfully. |
| 182 | 霹雳书坊（优） | `blocked_or_auth` | The exact route returns Cloudflare HTTP 403; Chromium remains on the security-verification page. |
| 419 | 梧桐中文 | `upstream_timeout` | The exact authored request timed out in sequential production replay and direct curl before any HTTP response. |
| 129 | 过期杂志（优+） | `credible_search_and_detail` | Production Search returned 18 books and Book Info enriched the selected result. |
| 193 | 电线看书（优） | `credible_search_and_detail` | Production Search returned 11 books and Book Info enriched the selected result. |
| 364 | 飞卢小说（优） | `shared_engine_gap` | The declared GB2312 charset was applied to response decoding but not GET query encoding. Changing only the query bytes to declared GB2312 recovered 20 Search results and successful Book Info. |
| 192 | 棉花小说（优） | `upstream_http` | The exact current route returns HTTP 403/redirect failure. Its inline <js> syntax is unsupported by NovelReader, but current content does not satisfy the full frozen workflow, so it is recorded separately rather than promoted as a recoverable gap. |
| 352 | 百度知道（优） | `site_or_source_drift` | The source script assumes final baseUrl contains word=<value>&, but the current final URL places word last; upstream Legado also exposes the final response URL. |
| 427 | 网易云说 | `upstream_timeout` | The exact authored request timed out in sequential production replay and direct curl before any HTTP response. |
| 6 | 篱笆文学（优+++） | `deferred_webview` | The source explicitly requests WebView behavior, outside the current regular/JavaScript transport target. |
| 201 | 火球书库（优） | `site_or_source_drift` | The multiline template JavaScript executes, but the current homepage no longer contains the expected action attribute, so the source script throws on match()[1]. |
| 46 | 番茄小说（优+） | `site_or_source_drift` | The current bootstrap response no longer defines variables required by the frozen source program. NovelReader also lacks whole-<js> URL execution, recorded separately as unsupported sampled syntax rather than a recoverable primary gap. |
| 66 | 古龙武侠（优+） | `upstream_timeout` | The exact authored request timed out in sequential production replay and direct curl before any HTTP response. |
| 78 | 中华诗词（优+） | `legitimate_empty` | The live HTTP 200 response is an information page stating that no search result was found. |
| 372 | 猫九小说 | `upstream_timeout` | The exact authored request timed out in sequential production replay and direct curl before any HTTP response. |
| 13 | 天天小说（优++） | `credible_search_and_detail` | Production Search returned 15 books and Book Info enriched the selected result. |
| 86 | 熊猫文学（优+） | `upstream_transport` | The exact authored route repeats HTTP 301 redirects until the redirect limit is reached in production and direct curl. |
| 199 | 万通蜡笔（优） | `upstream_http` | The exact authored Next.js data route repeatedly returns HTTP 400. |
| 354 | 百度贴吧（优） | `blocked_or_auth` | The exact Tieba route returns HTTP 403 and Chromium remains on 百度安全验证. |
| 108 | 米读小说（优+） | `upstream_timeout` | The exact authored API request timed out in sequential production replay and direct curl. |
| 431 | 安轻小说 | `upstream_timeout` | The exact authored API request timed out in sequential production replay and direct curl. |
| 395 | 红薯网站 | `credible_search_and_detail` | Production Search returned 10 books and Book Info enriched the selected result. |
| 387 | 纵横中文 | `deferred_webview` | The source explicitly requests WebView behavior, outside the current regular/JavaScript transport target. |
| 384 | 豆腐阅读 | `upstream_dns` | The authored API hostname does not resolve in production or direct curl. |
| 373 | 猫九小说 | `upstream_timeout` | The exact authored request timed out in sequential production replay and direct curl before any HTTP response. |
| 187 | 独步小说（优） | `invalid_source_contract` | The trailing URL-option text is malformed (`{"body": "id"="search-form"}`) and cannot represent a valid Legado request option object. |
| 405 | 铁血读书 | `upstream_timeout` | The exact authored request timed out in sequential production replay and direct curl before any HTTP response. |
| 134 | 夜天连看（优） | `credible_search_and_detail` | Production Search returned 20 books and Book Info enriched the selected result. |
| 47 | 番茄小说（优+） | `deferred_rhino_jvm` | The source requires Rhino/JVM JavaImporter after its jsLib is supplied. Search also omits source jsLib from URL evaluation, recorded separately as a compatibility observation without claiming workflow recovery. |
| 347 | 海词词典（优） | `upstream_timeout` | The exact authored request timed out in sequential production replay and direct curl before any HTTP response. |

## Confirmed recoverable shared gaps

### Correct OnlyOne regex replacement

Raw 375's standalone `##pattern##replacement###` rule should return only the first matched-and-replaced value. NovelReader preserves surrounding outer HTML and corrupts URL fields. A captured counterfactual changed only the two URL-field expressions to equivalent first-match capture/replacement semantics; Search returned 20 valid results and Book Info completed successfully.

### Honor URL-option charset for GET query encoding

Raw 364 declares GB2312. NovelReader decodes the response using that charset but sends the query in UTF-8. Sending the same query in GB2312 recovers 20 matching books. The fix belongs before RequestSpec construction in the shared URL builder.

## Deferred dependencies

Four sampled sources explicitly require WebView. Raws 47 and 51 require Rhino/JVM interoperability beyond regular goja support. Whole/inline `<js>` URL syntax is unsupported for raws 46/51/192, and Search omits `jsLib` for raw 47, but their current workflows also have independent site/HTTP/JVM blockers. These observations are preserved without claiming source recovery or counting them as primary shared-gap outcomes.

## Browser evidence

Playwright was used only for raws 123, 182, 354, and 355, where direct HTTP left browser-bypass ambiguity. Chromium remained on Cloudflare or Baidu security-verification pages for all four.

## Recommendation

Do not change production behavior inside this audit. Present the two confirmed recoverable shared gaps for approval, then implement them as focused shared-seam slices with deterministic public regressions and exact frozen-source post-fix reruns. Keep unsupported `<js>`, `jsLib`, WebView, and Rhino/JVM observations separate until a current satisfiable workflow proves recovery.
