# Explore live audit v13 — 2026-08-11

## Scope

- Unrestricted deterministic sample: **50** identities
- Seed: `NovelReader-explore-random-v13-2026-08-11`
- Excluded: all **550** stable `(rawIndex, bookSourceUrl)` identities from Explore batches 1–12
- Eligible before selection: **171**
- Ranking: `SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)`
- Corpus: `test_booksource4.json`, SHA-256 `23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c`
- Execution: fresh disposable reader root, unchanged corpus imported through the authenticated production API, four concurrent initial clients, 90-second source timeout, then sequential confirmation of every non-pass or diagnostic
- Stopping point: page 1 of the first selectable Explore category
- Source/parser changes during sampling: **none**

The first harness attempt omitted the newly required browser `Origin` header and received uniform HTTP 403 before any source executed. That invalid capture was discarded. The unchanged frozen identities were rerun with the disposable session cookie and canonical origin; only the valid run is evidence.

## Results

| Classification | Count |
|---|---:|
| `credible_nonempty` | 42 |
| `source_incomplete_or_invalid` | 3 |
| `upstream_dns` | 2 |
| `upstream_http` | 2 |
| `blocked_or_auth` | 1 |

The 42 credible sources returned **1,211 distinct book URLs**. All eight non-passes and raw 32's HTTP-status diagnostic were replayed sequentially. Raw 32 remained credible: its rules extracted ten distinct books while production explicitly reported the received upstream 404 response.

## Shared compatibility gaps

**None confirmed.** Every non-pass was explained by current upstream behavior or an incomplete/invalid imported contract. No source-specific workaround or parser change is recommended from this batch.

## Other confirmed outcomes

- `blocked_or_auth`: raw 795. Direct requests with desktop/mobile User-Agents and a real Chromium session all showed Cloudflare's explicit blocked page with no configured `.box` result cards.
- `upstream_dns`: raws 883 and 873. Both local resolution and Cloudflare/Google DNS-over-HTTPS found broken or refused authoritative delegation for `m.babahome.net` and `www.frxsw.com`.
- `upstream_http`: raw 681 repeatedly returned Cloudflare 521; raw 316 repeatedly returned Cloudflare 522.
- `source_incomplete_or_invalid`:
  - raw 252 redirects to a working Tencent catalog showing 3,811 results in Chromium, but the imported `ruleExplore` is empty.
  - raw 73 fails before category generation because its script uses malformed arrow-function destructuring such as `map([title,b]=>...)`; Node syntax checking confirms the same error.
  - raw 158 initially serves a JavaScript cookie gate. Chromium satisfies it and receives a current 20-item JSON response, but the imported `ruleExplore` is empty.

## Browser evidence

Playwright was used only where direct HTTP left a browser-behavior ambiguity:

1. raw 795 proved the site remains blocked in a real Chromium session, rather than merely requiring ordinary page JavaScript;
2. raw 252 proved current rendered catalog content exists despite the source declaring no result rules;
3. raw 158 proved its JavaScript cookie gate leads to 20 live records, while the missing result contract still prevents classification as an engine gap.

No browser automation was used as an automatic fallback or to bypass authentication/captcha policy.

## Recommendation

Do not implement a compatibility fix from v13. Preserve the evidence, continue treating the eight non-passes as source/upstream outcomes, and prioritize previously proven shared gaps or the remaining ownership-boundary work. Another unrestricted Explore sample would leave only 121 currently eligible identities unsampled; perform it only when additional breadth is worth the live-audit cost.
