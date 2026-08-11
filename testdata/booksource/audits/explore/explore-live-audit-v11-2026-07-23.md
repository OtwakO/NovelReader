# Explore live audit v11 — 2026-07-23

## Scope

- Unrestricted deterministic sample: **50** exact `(rawIndex, bookSourceUrl)` identities
- Seed: `NovelReader-explore-random-v11-2026-07-23`
- Corpus: `test_booksource4.json` (`23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c`)
- Prior stable identities excluded: **450** from batches 1–10
- Eligible identities before selection: **271**
- Selection: ascending `SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)`
- Execution: fresh isolated SQLite database, unmodified corpus, production Explore catalog/page API, four initial clients, 90-second audit timeout
- Every initial non-pass was rerun sequentially; the sequential observation is authoritative.
- No source or parser behavior changed during sampling.

## Results

| Classification | Count |
|---|---:|
| `credible_nonempty` | 39 |
| `engine_gap` | **4** |
| `upstream_http` | 3 |
| `upstream_dns` | 2 |
| `blocked_or_auth` | 1 |
| `site_drift` | 1 |

The 39 credible sources returned **1,142 distinct book URLs**. Raw 423 failed initially near the source deadline but returned 50 distinct books sequentially and is classified `credible_nonempty`.

## Shared engine gaps

### 1. Inline replacement Java identity escapes — raw 701

The exact PO18 category returns HTTP 200 with 30 current `.bookinfo` cards. Its valid rules contain `class.update@text##\简介：` and `class.cat@text##\更新到：`. Production fails with `invalid escape sequence: \简` / `\更`: inline `##` replacement compilation bypasses the Java identity-escape normalization already used by the regex analyzer.

### 2. Default positional ranges — raw 54

The exact 中华诗词 homepage returns HTTP 200 with 192 current poem links. Its fallback list rule `ul li a[8:]` depends on Legado's positional range syntax. Production does not implement that range and returns zero books.

### 3. Fixed ten-second source deadline — raw 220

The exact 栀子欢子 category serves a valid 52,338-byte response containing 100 matching list rows. Three consecutive production replays alternated solely around the fixed deadline: 502 at 10,002 ms, 200 with 100 books at 3,848 ms, then 502 at 10,002 ms. A completed direct replay returned the same 100-row body (SHA-256 `cf74b47e…7354`). The shared ten-second cutoff converts slow-but-valid responses into failures.

### 4. Structured JSON result lost before direct JavaScript — raw 443

The exact 漫客栈 API returns HTTP 200 JSON with 12 valid list objects. The result name rule directly reads `result.comic_id` and `result.title`. Production serializes each selected JSON object before creating its per-book analyzer, so direct `@js:` sees a string rather than the structured object and yields zero books. Preserving the selected object produces the expected name, author, and URL.

## Other confirmed outcomes

- Raw 885 and 637: Cloudflare HTTP 521.
- Raw 575: production upstream status varied while later direct HTTP succeeded; no stable parser mismatch was demonstrated.
- Raw 925 and 343: current DNS resolution failure.
- Raw 835: HTTP 403/challenge response prevents catalog JavaScript from discovering categories.
- Raw 198: imported domain redirects to a shutdown page; the replacement host is live, so this is site drift.

## Recommendation

Fix the four shared seams test-first at their existing shared boundaries. Keep stale source definitions, access failures, and source-specific redirects out of engine behavior. After fixes, rerun raws 701, 54, 220, and 443 through production Explore and then take another disjoint unrestricted sample.
