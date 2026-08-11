# Search → Book Info live audit v1 — 2026-08-11

## Scope

- Corpus: `test_booksource3.json`, SHA-256 `9928f8b217126804ea5ef9524d919f05cfafddfeff51ef9691e0edc308e13cdb`, 458 entries.
- Eligible identities: 329; deterministic unrestricted sample: 25.
- Seed: `SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)` with `NovelReader-search-bookinfo-random-v1-2026-08-11`.
- Query: `凡人修仙传`; page 1 only.
- Workflow: production search, then Book Info only for the first result with non-blank name and book URL.
- Initial concurrency: four; all 15 non-passes replayed sequentially.
- Fresh disposable data root; all 458 unmodified corpus entries imported.
- No source/parser behavior changed during sampling.

## Result

- 10/25 completed both Search and Book Info with usable data.
- 1 confirmed shared engine gap affected a currently working source at Book Info.
- 14 outcomes were upstream, blocked/authenticated, stale/drifted contracts, or a legitimate empty result.

| Raw index | Source | Classification | Evidence-backed rationale |
|---:|---|---|---|
| 4 | 天天看书（优+++） | `site_drift` | HTTP 200 search page no longer contains the source-authored .novel_cell contract; direct body inspection finds current chapter links but zero .novel_cell nodes. |
| 40 | 爱淘小说（优++） | `credible_search_and_detail` | Production search and Book Info both returned usable data. |
| 101 | 风云小说（优+） | `stale_source_contract` | The imported source itself says search is invalid; its generated API request now receives HTTP 403. |
| 406 | 铁血读书 | `stale_source_contract` | The authored SearchResults.aspx endpoint redirects to HTTPS and returns the site’s HTTP 404 resource-not-found page. |
| 133 | 松鹤庭沐（优） | `credible_search_and_detail` | Production search and Book Info both returned usable data. |
| 80 | 古诗词网（优+） | `stale_source_contract` | The imported source explicitly marks search invalid; the current redirected search page has none of its authored result selectors. |
| 408 | 轻次元姬 | `credible_search_and_detail` | Production search and Book Info both returned usable data. |
| 173 | 天堂深圳（优） | `credible_search_and_detail` | Production search and Book Info both returned usable data. |
| 53 | 天天书吧（优+） | `upstream_http` | The authored POST reaches the live endpoint, which consistently returns HTTP 500 with an empty body. |
| 190 | 笔趣小说（优） | `site_drift` | The request reaches a redirected replacement domain, whose HTTP 200 response asks for a search term instead of returning the authored .lis dl result shape. |
| 267 | 星际漫画（优） | `detail_engine_gap` | Search returns live results. The source-authored bookUrl suffix ,{Cookie:"xmanhua_lang=2"} is valid under Legado lenient URL-option parsing, but NovelReader percent-encodes it into the path and gets 404; fetching the URL portion with that cookie returns HTTP 200 detail HTML. |
| 22 | 轻说百科（优++） | `credible_search_and_detail` | Production search and Book Info both returned usable data. |
| 392 | 晋江评论 | `legitimate_empty` | The live API returns HTTP 200 with code 1055 and “没有更多小说了” for the frozen query; $.items is absent because the source reports no results. |
| 323 | 百度贴吧（优+） | `blocked_or_auth` | The live search endpoint consistently returns HTTP 403 with a Baidu security-verification page. |
| 97 | 狸猫故事（优+） | `blocked_or_auth` | The live endpoint consistently returns HTTP 503 with an explicit access-limit page. |
| 170 | 殓师灵异（优） | `credible_search_and_detail` | Production search and Book Info both returned usable data. |
| 184 | 红牛小说（优） | `upstream_dns` | The source hostname consistently fails DNS resolution in the audit environment. |
| 56 | 阅友小说（优+） | `credible_search_and_detail` | Production search and Book Info both returned usable data. |
| 145 | 车群小说（优） | `credible_search_and_detail` | Production search and Book Info both returned usable data. |
| 415 | 苏轻小说 | `credible_search_and_detail` | Production search and Book Info both returned usable data. |
| 151 | 灵感小说（优） | `upstream_http` | The source uses a valid lenient URL-option POST contract that NovelReader does not parse, but a direct correctly formed POST to the live endpoint returns HTTP 500; this sample cannot establish a working-source failure. |
| 92 | 旭日小说（优+） | `stale_source_contract` | The source domain now returns an HTTP 404 domain-parking page. |
| 137 | 中文看书（优） | `credible_search_and_detail` | Production search and Book Info both returned usable data. |
| 50 | 轻说机翻（优+） | `blocked_or_auth` | The source declares a login UI and its computed live API returns HTTP 401 “游客没有权限执行此操作”. NovelReader also does not execute whole-URL <js> rules, but anonymous live behavior cannot establish a working-source failure. |
| 164 | 八一中文（优） | `upstream_http` | The authored POST consistently times out before any HTTP response. |

## Confirmed shared gap

### Lenient URL-option object keys

Legado’s documented URL contract permits a trailing request option object, and upstream `AnalyzeUrl` parses strict JSON first then falls back to lenient Gson. Raw 267’s live search returns valid detail links ending:

`,{Cookie:"xmanhua_lang=2"}`

NovelReader leaves that object in the path and percent-encodes it, producing a detail 404. A direct request to the URL portion with the declared cookie returns HTTP 200 and current detail HTML. This is a reusable URL-parser seam, not an xmanhua-specific patch opportunity.

Raw 151 uses the same unsupported lenient form for a POST search URL, but the correctly formed live POST currently returns HTTP 500. It supports the shared syntax diagnosis but is not counted as proof of a working-source failure.

## Observed but not promoted to an engine gap

Raw 50 uses a whole-URL `<js>…</js>` search rule. Upstream `AnalyzeUrl` executes it before URL parsing; NovelReader currently percent-encodes it. The source declares login requirements and its computed anonymous API returns HTTP 401, so this batch cannot establish that the syntax gap broke a currently working source. Preserve the observation for a future valid live fixture; do not fix from this source alone.

## Recommendation

Approve one focused TDD fix for lenient trailing URL-option object keys at the shared URL builder, using reduced fixtures plus raw 267 as targeted live verification. Do not bundle whole-URL `<js>` support until a valid working source or deterministic upstream-contract priority justifies it. Do not patch any sampled source contract.
