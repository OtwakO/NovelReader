# Explore Live Compatibility Audit — 2026-07-18

## Scope

- Corpus: `test_booksource4.json`, SHA-256 `23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c` (939 raw entries; 722 independently Explore-eligible).
- Sample: 50 distinct source URLs selected with seed `NovelReader-explore-audit-v1`, stratified across strict/lenient/legacy/JavaScript catalogs, GET/page-selector/POST/charset/WebView requests, and Default/CSS/JSONPath/XPath/Regex/JavaScript result rules.
- Mandatory reports: raw indices 9 and 752 (`夜伴书屋` identities), 89 (`中文看书（优）`), and 15 (`笔趣阁 · 常` in the UI), plus two other stable-identity `笔趣阁` samples.
- Probe: first selectable category, page 1, through the production fingerprint transport in an isolated store. Apparent failures were compared with captured upstream responses and direct Playwright navigation. The imported UI was also exercised at 390×844.
- Boundary: diagnosis only. No production behavior or source rule was changed.
- Reproducibility manifest: [`explore-live-audit-2026-07-18.json`](explore-live-audit-2026-07-18.json) records every stable source URL, raw list/name/URL rule, catalog/result mode, selected category, expanded request, final response URL/status/transport/body SHA-256, engine diagnostic, count, duration, and classification. POST bodies are included only when non-sensitive; captured response bodies remain local and are represented by hashes.

This is a point-in-time compatibility sample, not a claim that a live source will remain available. One page per source limits upstream load and means later categories may have different outcomes.

## Result

| Classification | Sources | Share |
|---|---:|---:|
| Confirmed non-empty, distinct results | 20 | 40% |
| Shared result/analyzer engine gaps | 16 | 32% |
| Missing advanced JavaScript bridge | 2 | 4% |
| Invalid raw source scripts | 3 | 6% |
| Upstream/WAF/stale source rules | 8 | 16% |
| Legitimate empty upstream response | 1 | 2% |

The UI is not the cause. For `中文看书（优）`, the API returned one concatenated result and the UI accurately displayed `1 book`; Playwright reported no console warnings/errors. Direct Playwright inspection of the same upstream page found 23 book nodes.

## Sample evidence

`Direct` is the directly observed upstream item count when it was needed to distinguish parser behavior. `—` means the engine already returned a credible non-empty set or the failure was self-identifying JSON/script evidence.

| Raw | Source | Engine | Direct | Classification |
|---:|---|---:|---:|---|
| 1 | 八叉书库 | 20 | — | Pass |
| 9 | 夜伴书屋 | 0 | 0 | Upstream redirects to `/lander`; not an extraction gap |
| 15 | 笔趣阁 (`m.22biqu.com`) | 0 | 50 | Engine applies absolute `bookUrlPattern` before resolving relative book URLs |
| 23 | 猫眼看书（优++） | error | valid JSON | Engine lacks JSON array-property projection used by `$.categoryNames.className` |
| 53 | 中华诗词（优+） | 0 | 188 broad candidates | Engine does not implement the connector branch slice `a[8:]` correctly |
| 72 | 米读小说（优+） | catalog error | — | Raw script has invalid unparenthesized destructuring arrow parameters |
| 89 | 中文看书（优） | 1 concatenated | 23 | Engine misroutes `.lis@li` as one collection instead of Default traversal |
| 99 | 速读谷吧（优） | 1 fallback | 10 | Engine misclassifies standalone Default selector `class.item` as CSS |
| 105 | 白浅小说（优） | 0 | 30 | Site moved from `178yhr.com` to `178xs.cc`; source URL pattern is stale |
| 140 | 如漫画网（优+） | JSON error | WAF HTML | Upstream returned Parklogic/anti-bot HTML instead of the expected JSON |
| 150 | 网络漫画（优） | error | valid JSON | Engine lacks Legado single-brace value interpolation inside a JSON-derived URL |
| 176 | 优品文档（导+） | 20 | — | Pass |
| 182 | 爱发电网（优） | 1 | 1 | Pass; upstream payload contained one album |
| 184 | 乐乎文章（优） | 12 | — | Pass |
| 193 | 盒子游戏（优） | error | valid JSON | Engine routes mixed Default/JSON connector expression as one JSONPath |
| 212 | 纵横中文 | catalog error | — | Raw script has invalid unparenthesized destructuring arrow parameters |
| 216 | 晋江文学 | catalog error | — | Missing advanced Legado bridge (`getLoginHeader`; source is also configuration-dependent) |
| 242 | 刺猬猫网 | 10 | — | Pass |
| 257 | 爱漫客栈 | 0 | 30 | Source pattern expects `m.mkzhan.com`; live catalog resolves on `www.mkzhan.com` |
| 285 | 菜小说网 | 30 | — | Pass |
| 286 | 👔 天式从横 | 0 | 20 | Engine misclassifies standalone Default selector `class.common-bookele` |
| 291 | PO18文学 | 30 | — | Pass |
| 323 | 🔰 BL小说 | 20 | — | Pass |
| 370 | 🎃腐小说🎃 | 0 | 20 | Engine tests relative URLs against an absolute `bookUrlPattern` before resolution |
| 418 | 笔尚小说 | 1 concatenated | 50 captured | Engine misroutes `#j@li`, returning one collection instead of 50 list elements |
| 433 | 🎨 漫客栈🅰 | error | valid JSON | Engine lacks JSON-derived interpolation in a literal URL expression |
| 434 | 繁星四月🎃 | 20 | — | Pass |
| 441 | 繁星四月 | 20 | — | Pass |
| 476 | 晋江🎃 | 238 | — | Pass |
| 506 | 我爱读者 | 30 | — | Pass |
| 522 | 火星小说🎃 | catalog error | — | Raw Explore script is syntactically malformed |
| 529 | 中文书城🎃 | error | valid JSON | Engine lacks JSON-derived interpolation in a formatted field value |
| 533 | ㊣ 豆瓣阅读 #一程 | error | valid JSON | Engine JSONPath implementation lacks filter expressions (`[?()]`) |
| 552 | 腐小说🎃 | 20 | — | Pass |
| 561 | 八零小说 | 0 | 20 | Site moved from `80zw.la` to `80ge.info`; source URL pattern is stale |
| 562 | 笔尖中文 | catalog error | — | Missing advanced bridge (`java.toast`, then Jsoup-style APIs) |
| 576 | 得间·女生 | 0 | 23 | Source pattern requires a trailing slash that the live absolute book URLs do not contain |
| 587 | 红薯阅读 | 30 | — | Pass |
| 596 | 笔趣阁🎃#45 | 30 | — | Pass |
| 599 | 多看阅读🎃#2 | 0 | 0 | Legitimate upstream JSON: `items: []`, `more: false` |
| 616 | 蜘蛛小说网 | 10 | — | Pass |
| 704 | 笔趣阁小说网 | 20 | — | Pass |
| 722 | 趣书网小说 | 100 | — | Pass |
| 752 | 🎒 夜伴书屋 | 0 | 0 | Live redirect ends at HTTP 403; source host/pattern is stale |
| 823 | 🎃第一版主🎃#10 | 0 | 20 | Engine misclassifies standalone Default selector `class.line` |
| 869 | 男友书屋 | timeout | HTTP 503 | Upstream availability failure |
| 897 | 纪念读书 | 0 | 21 | Engine misclassifies standalone Default selector `id.articlelist` |
| 902 | 笔库小说 | 20 | — | Pass |
| 909 | 😍轻写真 | 1 | 10 | Engine misroutes `#post_list_box@li` as one collection |
| 928 | 西瓜看书 | 25 | — | Pass |

## Ranked shared gaps

1. **Resolve book URLs before `bookUrlPattern` filtering.** Directly explains raw 15 (`笔趣阁 · 常`) and 370. This is a small shared ordering defect, not a source-specific exception.
2. **Correct Default-mode detection.** Standalone `class.*`/`id.*` and CSS-shorthand traversal such as `#j@li` currently route incorrectly. This explains raw 89, 99, 286, 418, 823, 897, and 909; raw 897 also has a separate exclusion-suffix gap.
3. **Evaluate connector branches independently and support selector ranges.** Mixed Default/JSON rules and `a[8:]` must retain per-branch mode and slicing semantics (raw 53 and 193).
4. **Close targeted JSON compatibility gaps.** Add array-property projection, JSONPath filters, and Legado value interpolation only from captured fixtures (raw 23, 150, 433, 529, 533).
5. **Keep advanced bridge work separate.** `晋江文学` and `笔尖中文` require multiple stateful/UI/Jsoup-style Java APIs; implementing one no-op method would not make either source work.

## Priority-fix rerun — 2026-07-19

The user approved gaps 1 and 2 only. Fixture-driven changes repaired eight sampled sources without source-specific exceptions:

| Raw | Before | After |
|---:|---:|---:|
| 15 笔趣阁 · 常 | 0 | 50 |
| 89 中文看书（优） | 1 concatenated | 20 distinct |
| 99 速读谷吧（优） | 1 fallback | 10 distinct |
| 286 👔 天式从横 | 0 | 20 |
| 370 🎃腐小说🎃 | 0 | 20 |
| 418 笔尚小说 | 1 concatenated | 50 distinct |
| 823 🎃第一版主🎃#10 | 0 | 20 |
| 909 😍轻写真 | 1 | 10 distinct |

The stable non-empty baseline improved from **20/50 to 28/50**. The live rerun observed 29 non-empty sources because raw 869 recovered from its earlier upstream HTTP 503 and returned 30 books; it remains classified as an upstream availability result, not an engine repair. Raw 897 still returns zero because its `!0` exclusion suffix is a separate unapproved rule gap. Raw 576 also remains zero and was corrected to stale-source classification: its absolute pattern requires a trailing slash absent from current live book URLs.

Fresh 390×844 Playwright verified `笔趣阁 · 常` at 50 books and `中文看书（优）` at 20, with no console warnings/errors. Full rerun output is recorded in [`explore-live-audit-priority-fix-rerun-2026-07-19.json`](explore-live-audit-priority-fix-rerun-2026-07-19.json).

## Recommended fix boundary

Start with gaps 1 and 2 using the captured HTML fixtures because they are small, shared, and explain the user-reported `笔趣阁` failure plus six other sampled failures. Rerun the fixed 50-source manifest after each patch. Then implement gaps 3–4 in separate fixture-driven commits. Do not patch WAF responses, dead domains, invalid scripts, or individual source selectors in the engine.
