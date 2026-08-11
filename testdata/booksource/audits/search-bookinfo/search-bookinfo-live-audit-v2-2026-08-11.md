# Search → Book Info live audit v2 — 2026-08-11

## Scope

- Corpus: `test_booksource3.json`, SHA-256 `9928f8b217126804ea5ef9524d919f05cfafddfeff51ef9691e0edc308e13cdb`, 458 entries.
- Deterministic unrestricted sample: 25, disjoint from the 25 v1 identities; 304 eligible identities remained before selection.
- Seed: `NovelReader-search-bookinfo-random-v2-2026-08-11`; identity: `(rawIndex, bookSourceUrl)`.
- Query: `凡人修仙传`; page 1 only.
- Workflow: production Search, then Book Info for the first credible result; stop before TOC/content.
- Exactly the 25 frozen raw definitions were imported, preventing duplicate runtime keys from replacing sampled contracts.
- Initial concurrency: four; every non-pass replayed sequentially.
- No source or parser behavior changed during the audit.

## Result

- 8/25 completed Search and Book Info with usable data.
- Two distinct shared compatibility gaps were confirmed against currently working live responses.
- One source explicitly depends on deferred WebView behavior.
- All other outcomes were upstream HTTP/timeouts, blocking, source/site drift, or legitimate empty results.

| Raw index | Source | Classification | Evidence-backed rationale |
|---:|---|---|---|
| 96 | 大唐小说（优+） | `credible_search_and_detail` | Production Search and Book Info both returned usable data. |
| 402 | 九怀小说 | `upstream_http` | Sequential replay and direct HTTP both return HTTP 403 Forbidden from the authored search endpoint. |
| 233 | 爱看漫画（优+） | `credible_search_and_detail` | Production Search and Book Info both returned usable data. |
| 179 | 爱尚小说（优） | `search_engine_gap` | The authored POST redirects to a live detail page. Its Search rules produce a named item but no `bookUrl`; NovelReader discards it instead of using the final response URL as Legado does. |
| 49 | 有度轻说（优+） | `detail_engine_gap` | Search extracts two matching hrefs into one newline-concatenated bookUrl, so Book Info requests a malformed aggregate and fails. The first extracted href is live and enriches successfully. |
| 174 | 南极小说（优） | `credible_search_and_detail` | Production Search and Book Info both returned usable data. |
| 148 | 无线电子（优） | `upstream_http` | The live endpoint repeatedly redirects to the identical URL until the client reaches its redirect limit; direct HTTP reproduces the loop. |
| 413 | 轻菠萝包 | `credible_search_and_detail` | Production Search and Book Info both returned usable data. |
| 16 | 猫眼看书（优++） | `credible_search_and_detail` | Production Search and Book Info both returned usable data. |
| 378 | 企鹅阅读 | `credible_search_and_detail` | Production Search and Book Info both returned usable data. |
| 155 | 壹二小说（优） | `credible_search_and_detail` | Production Search and Book Info both returned usable data. |
| 396 | 中小说网 | `deferred_webview` | The source receives a JavaScript cookie challenge and explicitly calls java.webView before retrying its JSON API. WebView execution is outside the current regular/JS source target and is not promoted to a non-WebView engine gap. |
| 165 | 铅笔小说（优） | `upstream_http` | The live authored search endpoint returns HTTP 400 on replay; direct HTTP also fails. |
| 35 | 有度中文（优++） | `blocked_or_auth` | Direct HTTP returns Cloudflare 403 and Chromium renders an explicit “Sorry, you have been blocked” page. |
| 126 | 恩施轻语（优+） | `legitimate_empty` | The live HTTP 200 page contains the query text but no source-authored .col-lg-3 or .card-title result entries. |
| 152 | 就爱文学（优） | `upstream_timeout` | The authored POST repeatedly times out before any HTTP response; DNS still resolves. |
| 99 | 狸猫故事（优+） | `blocked_or_auth` | The live endpoint returns HTTP 503 with an explicit access-limit page. |
| 350 | 百度知道（优） | `stale_source_contract` | The source script assumes baseUrl contains word=<value>&. Baidu reorders the final URL so word is last; upstream Legado also exposes the final response URL as baseUrl, so the same authored regex fails there. |
| 111 | 神凑轻说（优+） | `blocked_or_auth` | The live endpoint returns a Cloudflare HTTP 403 challenge page. |
| 224 | 双语小说（英） | `site_drift` | The current HTTP 200 page retains an old .mcon container but no longer contains the source-authored result field structure needed to form books. |
| 19 | 神凑轻说（优++） | `upstream_http` | The live authored search endpoint consistently returns HTTP 400. |
| 55 | 阅友小说（优+） | `credible_search_and_detail` | Production Search and Book Info both returned usable data. |
| 306 | 面包聆听（优） | `legitimate_empty` | The live JSON response explicitly reports 没有相关作品 with results: []. |
| 198 | 歌书网吧（优） | `upstream_timeout` | The authored endpoint repeatedly times out before any HTTP response; DNS still resolves. |
| 69 | 金庸小说（优+） | `upstream_http` | Sequential replay and direct HTTP both return HTTP 404 for the authored search route. |

## Confirmed shared gaps

### Default/JSoup Search `bookUrl` must keep the first extracted value

Raw 49's broad Default/JSoup `a@href` rule matches both the detail link and a chapter link. NovelReader joins them with a newline and passes the malformed aggregate to Book Info. Upstream Legado calls `AnalyzeRule.getString(..., isUrl = true)` for Search `bookUrl`; its Default/JSoup branch uses `AnalyzeByJSoup.getString0()` and keeps the first value.

Both individual hrefs are live HTTP 200 pages. Replaying Book Info with the first extracted href succeeds and enriches `凡人修仙之仙界篇`. This is a genuine mode-specific compatibility gap even though the source could use a narrower selector. XPath and JSONPath retain their upstream ordinary string behavior, and intentionally plural URL fields continue using list semantics.

### Empty Search `bookUrl` must default to the final response URL

Raw 179's authored POST redirects to `https://www.23hh.la/book/3/3713/`. Its Search `bookList` and `name` rules produce one valid item named `凡人修仙传`, but its `bookUrl` rule extracts no value on this detail-shaped response. NovelReader discards the item because it requires both name and URL.

Upstream Legado's `BookList.getSearchItem` sets an empty Search `bookUrl` to `baseUrl`. Applying that shared behavior retains the item with the actual final response URL, after which the same source's Book Info rules enrich successfully.

## Deferred rather than promoted

Raw 396 receives a JavaScript cookie challenge and explicitly calls `java.webView` before retrying its JSON API. This is genuine WebView-dependent behavior, but WebView execution remains a deferred extension target. Do not patch it as a regular JavaScript bridge method without the WebView architecture.

## Browser evidence

Playwright was used only for raw 35, where direct HTTP left a browser-bypass ambiguity. Chromium rendered Cloudflare's explicit blocked page, so the result remains `blocked_or_auth`.

## Recommendation

Approved and resolved in a separate fix phase: reproduce Legado's mode-specific `isUrl` behavior for Default/JSoup Search `bookUrl`, and default an empty Search `bookUrl` to the final response URL. Each fix has a reduced public-boundary regression plus frozen live-source post-fix evidence. The implementation does not generalize first-value semantics to XPath/JSONPath or intentionally plural URL fields, implement WebView support, or patch sampled sources.

Post-fix evidence: `search-bookinfo-live-v2-fixes-rerun-2026-08-12.json`.
