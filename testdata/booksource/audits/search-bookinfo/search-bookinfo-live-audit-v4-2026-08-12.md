# Search → Book Info live audit v4 — 2026-08-12

## Scope

- Corpus: `test_booksource3.json`, SHA-256 `9928f8b217126804ea5ef9524d919f05cfafddfeff51ef9691e0edc308e13cdb`, 458 entries.
- Deterministic unrestricted sample: 50, disjoint from all 100 v1–v3 identities; 229 eligible identities remained before selection.
- Seed: `NovelReader-search-bookinfo-random-v4-2026-08-12`; identity: `(rawIndex, bookSourceUrl)`.
- Query: `凡人修仙传`; page 1 only.
- Workflow: production Search, then Book Info for the first credible result; stop before TOC/content.
- Exactly the 50 frozen raw definitions were executed, preventing duplicate runtime keys from replacing sampled contracts.
- Initial concurrency: four; all 41 non-passes replayed sequentially.
- No source or parser behavior changed during the audit.

## Result

- 9/50 completed Search and Book Info with usable data.
- 0/50 is an audit-proven recoverable shared-engine gap.
- 20/50 persistently timed out; 4/50 returned persistent HTTP failures.
- 3/50 explicitly require WebView and 1/50 requires Rhino/JVM interoperability.
- Remaining outcomes are legitimate empties, blocked/authentication boundaries, site/source drift, invalid source contracts, DNS, or transport failures.

| Raw index | Source | Classification | Evidence-backed rationale |
|---:|---|---|---|
| 221 | 英文小说（英） | `legitimate_empty` | The exact request reached a valid query response but produced no source-authored results for this query. |
| 138 | 阅友小说（优） | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 166 | 速读谷子（优） | `credible_search_and_detail` | Production Search returned usable results and Book Info enriched the selected result. |
| 388 | 纵横中文 | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 420 | 梧桐中文 | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 362 | 起点男频（优） | `deferred_webview` | The exact source explicitly requires WebView/browser behavior outside the regular HTTP/JavaScript transport path. |
| 409 | 酷我小说 | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 428 | 安轻小说 | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 79 | 古诗文网（优+） | `site_or_source_drift` | The current live page or response schema no longer satisfies assumptions encoded by the frozen source rules. |
| 324 | 吾爱破解（优+） | `upstream_dns` | The exact authored hostname does not resolve. |
| 425 | 企鹅浏览 | `credible_search_and_detail` | Production Search returned usable results and Book Info enriched the selected result. |
| 1 | 番茄小说（优+++） | `deferred_rhino_jvm` | The source library requires Rhino/JVM facilities such as JavaImporter before its URL function can be defined. |
| 351 | 百度知道（优） | `site_or_source_drift` | The current live page or response schema no longer satisfies assumptions encoded by the frozen source rules. |
| 181 | 蚂蚁阅读（优） | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 103 | 阅读库子（优+） | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 74 | 盐选文库（优+） | `upstream_http` | The exact authored route persistently returned an upstream HTTP failure in production and direct replay. |
| 327 | 微博评论（优+） | `invalid_source_contract` | The frozen source script or URL construction is internally invalid under standard JavaScript/URL semantics. |
| 113 | 轻之文库（优+） | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 88 | 全本小说（优+） | `upstream_transport` | The exact authored endpoint fails at the transport/protocol boundary. |
| 146 | 四三看书（优） | `blocked_or_auth` | The exact request reached a security or login boundary rather than a usable search result page. |
| 374 | 百度小说 | `credible_search_and_detail` | Production Search returned usable results and Book Info enriched the selected result. |
| 127 | 七百小说（优+） | `deferred_webview` | The exact source explicitly requires WebView/browser behavior outside the regular HTTP/JavaScript transport path. |
| 424 | 松鹤阅读 | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 407 | 九阅小说 | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 38 | 七猫小说（优++） | `credible_search_and_detail` | Production Search returned usable results and Book Info enriched the selected result. |
| 433 | 刺猬猫吧 | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 132 | 追光阅读（优） | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 154 | 蚂蚁文学（优） | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 345 | 海词木稽（优） | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 394 | 追书出版 | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 186 | 全本小说（优） | `credible_search_and_detail` | Production Search returned usable results and Book Info enriched the selected result. |
| 157 | 爱爱中文（优） | `upstream_http` | The exact authored route persistently returned an upstream HTTP failure in production and direct replay. |
| 18 | 鲸云轻说（优++） | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 178 | 全本同人（优） | `legitimate_empty` | The exact request reached a valid query response but produced no source-authored results for this query. |
| 195 | 顶点小说（优） | `site_or_source_drift` | The current live page or response schema no longer satisfies assumptions encoded by the frozen source rules. |
| 329 | 书单推荐（优+） | `invalid_source_contract` | The frozen source script or URL construction is internally invalid under standard JavaScript/URL semantics. |
| 125 | 铅笔轻说（优+） | `upstream_http` | The exact authored route persistently returned an upstream HTTP failure in production and direct replay. |
| 163 | 零零小说（优） | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 140 | 文桑小说（优） | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 168 | 无忧书城（优） | `credible_search_and_detail` | Production Search returned usable results and Book Info enriched the selected result. |
| 217 | 爱下电子（繁） | `credible_search_and_detail` | Production Search returned usable results and Book Info enriched the selected result. |
| 106 | 快眼看书（优+） | `credible_search_and_detail` | Production Search returned usable results and Book Info enriched the selected result. |
| 435 | 经致文学 | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 120 | 有度中文（优+） | `credible_search_and_detail` | Production Search returned usable results and Book Info enriched the selected result. |
| 403 | 九怀文学 | `upstream_timeout` | The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check. |
| 76 | 我是盐神（优+） | `upstream_http` | The exact authored route persistently returned an upstream HTTP failure in production and direct replay. |
| 177 | 爱久小说（优） | `deferred_webview` | The exact source explicitly requires WebView/browser behavior outside the regular HTTP/JavaScript transport path. |
| 62 | 国学经典（优+） | `blocked_or_auth` | The exact request reached a security or login boundary rather than a usable search result page. |
| 360 | 起点中文（优+） | `blocked_or_auth` | The exact request reached a security or login boundary rather than a usable search result page. |
| 72 | 民间故事（优+） | `legitimate_empty` | The production bridge ignored the frozen gb2312 argument and sent UTF-8; changing only that bridge result fixed the query text, but the live page still explicitly returned zero records. |

## Non-recovering compatibility observation

Raw 72 calls `java.encodeURI(key, 'gb2312')`. The rulebook and Legado implementation support the charset argument, while NovelReader currently ignores it and emits UTF-8. Replacing only that bridge result with the correct GB2312 bytes fixes the server-side mojibake, but the exact page still reports zero matching records. This proves a genuine bridge omission without proving recovery for the sampled query.

Recommendation at audit close: document and separately approve a small shared bridge fix adding the optional charset argument to the existing `java.encodeURI` binding. Do not count raw 72 as a recovered source and do not mix this with WebView or Rhino/JVM work.

## Post-audit resolution

The separately approved bridge fix is complete. `java.encodeURI(value)` retains UTF-8 behavior, while `java.encodeURI(value, charset)` now uses the existing shared charset encoder and reports unsupported charset names as JavaScript evaluation errors.

A fresh replay of the untouched frozen raw-72 definition confirms that production now requests GB2312 bytes (`%B7%B2%C8%CB%D0%DE%CF%C9%B4%AB`) and the server displays `凡人修仙传` correctly. The response still explicitly contains zero matching records, so the historical `legitimate_empty` classification and v4's zero-recoverable-gap result remain unchanged.

Post-fix evidence: `search-bookinfo-live-v4-fixes-rerun-2026-08-12.json`; verifier: `scripts/search-bookinfo-audit/v4/verify-fixes.mjs`.

## Browser evidence

Playwright was used only for four exact requests where direct HTTP left browser behavior ambiguous: raw 62 remained Cloudflare-blocked; raw 79 redirected to the app-download page; raw 146 remained login-gated; and raw 360 stayed at Qidian's security boundary. Raw 362 was not counted as a browser run: its frozen source contract explicitly invokes `java.webView`, and production reports the missing WebView bridge against the same Qidian probe response.

## Recommendation

The audit-stage recommendation was followed: no production behavior changed during sampling, and the charset-aware bridge omission was presented and approved separately. That bounded fix is now complete and verified without changing raw 72's legitimate-empty outcome. No further Search → Book Info fix is recommended from v4.
