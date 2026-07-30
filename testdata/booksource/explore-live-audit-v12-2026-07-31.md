# Explore live audit v12 — 2026-07-31

## Scope

- Unrestricted deterministic sample: **50** identities
- Seed: `NovelReader-explore-random-v12-2026-07-31`
- Excluded: all **500** stable `(rawIndex, bookSourceUrl)` identities from batches 1–11
- Eligible before selection: **221**
- Ranking: `SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)`
- Corpus: `test_booksource4.json`, SHA-256 `23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c`
- Execution: fresh isolated SQLite database, unmodified corpus, production Explore catalog/page APIs, four concurrent initial clients, 90-second audit limit, sequential confirmation of every non-pass or diagnostic
- Source/parser changes during audit: **none**

## Results

| Classification | Count |
|---|---:|
| `credible_nonempty` | 27 |
| `engine_gap` | 4 |
| `source_incomplete_or_invalid` | 5 |
| `site_drift` | 5 |
| `upstream_dns` | 3 |
| `upstream_http` | 2 |
| `stale_source_contract` | 2 |
| `blocked_or_auth` | 1 |
| `audit_infrastructure` | 1 |

The 27 credible sources returned **2,768 distinct book URLs**. Raw 734 is included: production retained the exact first 2,000 of 2,432 live rows and emitted the expected `result_truncated` diagnostic. All 24 initial non-passes or diagnostics were replayed sequentially; results were stable.

## Confirmed shared compatibility gaps

### 1. Default desktop User-Agent breaks mobile-only source routing — raw 251

The exact Tencent mobile rank URL is live. NovelReader's default desktop User-Agent redirects it to a desktop page with 30 rows but none of the imported mobile selectors. A mobile User-Agent reaches the intended page with seven `.comic-link` results, and changing only that request header in the production runtime returns seven books. This is a reusable transport/default-header mismatch for mobile source contracts.

### 2. Rule-driven AJAX cannot run after an outer non-2xx response — raw 197

The source intentionally uses a synthetic outer URL whose body is irrelevant; the list rule performs the real encrypted request through `java.ajax`. Production rejects the outer HTTP 400 before rule execution. The real Shubl endpoint is live and decodes to 15 books. Non-empty error bodies should remain available to source rules when the source contract deliberately drives its own request.

### 3. Missing crypto/login Java bridge APIs — raw 197

After substituting a successful outer trigger only for diagnosis, the exact current rule reaches missing reusable APIs: `java.createSymmetricCrypto`, `java.base64DecodeToByteArray`, and `source.getLoginHeaderMap`. These are distinct from the outer-status gate but affect the same identity.

### 4. Default class traversal does not recognize arbitrary attribute getters — raw 163

The live Missevan page contains 20 configured cards. `class.video-play-icon@title` is treated as another traversal because `title` is not in the recognized Default getter set, while equivalent `.video-play-icon@title` and `tag.a.0@title` rules return all 20 books. Arbitrary HTML attributes must remain valid getters after Default class traversal.

### 5. Default singleton bracket indexes are unsupported — raw 17

The live page contains ten configured cards. Legado's valid `a[1]@href` singleton-index syntax is not recognized because NovelReader currently detects only bracket ranges containing `:`. This is reusable across Default selectors.

### 6. Multi-match attribute extraction does not skip blank leading values — raw 17

The same source's `a@title` sees an initial anchor without `title` and a later named anchor. Legado collection extraction skips blank attributes; NovelReader reads only the first selected node, leaving every book unnamed. An equivalent supported index plus an explicit nonblank name returns ten books.

## Other confirmed outcomes

- `upstream_http`: raws 617 and 877 (Cloudflare 521; expired upstream TLS certificate)
- `upstream_dns`: raws 274, 754, and 668
- `blocked_or_auth`: raw 224 requires an undeclared site Referer
- `stale_source_contract`: raws 355 and 295
- `site_drift`: raws 372, 472, 740, 456, and 156
- `source_incomplete_or_invalid`: raws 504, 125, 910, 673, and 633
- `audit_infrastructure`: raw 714; the audit resolver returned only `::`, while public DNS plus the exact rules produced 30 books

## Recommendation

Fix the six shared seams test-first through existing transport, Explore workflow, Analyzer, and Java-bridge interfaces, then exact-rerun raws 251, 197, 163, and 17. Keep source-specific stale, invalid, blocked, and upstream outcomes unchanged. After those fixes, another unrestricted sample is justified because only 171 eligible identities remain unsampled and batches 9–12 continue to expose reusable gaps.
