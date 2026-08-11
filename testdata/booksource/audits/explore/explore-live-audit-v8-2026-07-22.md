# Explore live audit v8 — 2026-07-22

## Scope

- Unrestricted deterministic sample: **50** enabled Explore identities.
- Seed: `NovelReader-explore-random-v8-2026-07-22`.
- Corpus: `test_booksource4.json`, SHA-256 `23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c`.
- Ranking: `SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)`.
- Excluded all **300** stable identities from audit batches 1–7; **423** eligible identities remained before selection.
- Fresh isolated SQLite database and production Explore APIs. Initial pass used four clients and a 90-second per-request timeout. Every non-pass was replayed sequentially.
- No source, parser, transport, or production behavior changed during sampling.

## Results

| Classification | Identities | Count |
|---|---|---:|
| `credible_nonempty` | 362, 667, 308, 166, 769, 698, 805, 379, 866, 860, 366, 549, 670, 191, 485, 167, 716, 62, 790, 389, 570, 553, 580, 110, 90, 202, 129, 899, 930, 894, 778, 813, 367, 354 | 34 |
| `upstream_dns` | 848, 538 | 2 |
| `upstream_http` | 477, 530, 6 | 3 |
| `blocked_or_auth` | 64, 863 | 2 |
| `stale_source_contract` | 264, 227, 859, 92, 643, 680 | 6 |
| `source_incomplete_or_invalid` | 179 | 1 |
| `site_drift` | 77, 889 | 2 |
| `engine_gap` | — | 0 |

The 34 credible sources returned **1,045 distinct book URLs**. No duplicate-only or suspicious-diagnostic result survived confirmation.

## Shared gaps

**None confirmed in this batch.** Every non-pass was explained by direct DNS/HTTP, current response content, source JavaScript syntax, or an imported URL-pattern/selector contract that no longer matches the live site. No reusable parser, transport, catalog, or Java-bridge mismatch remained after diagnosis.

## Non-engine evidence

- **DNS:** raw 848 (`www.shitouxs.com`) and 538 (`m.xpxs.net`) have no usable address records.
- **Upstream HTTP:** raw 477 returns nginx 502; raw 530's exact first-page API returns 500 while later pages remain live; raw 6's contracted endpoints return 404.
- **Blocked/auth:** raw 64's signed API contract returns 401 Unauthorized; raw 863 returns nginx 444 in both direct and Chromium requests.
- **Stale contracts:** raw 264 reaches a ParkLogic parking page; raw 227 reaches a shutdown/removed bookstore route; raw 859, 92, 643, and 680 extract current rows but their imported `bookUrlPattern` rejects every current URL.
- **Invalid source:** raw 179's category generator contains malformed arrow-function parameters rejected by both goja and Node.
- **Site drift:** raw 77 returns a suspended-site HTML page instead of JSON; raw 889 redirects to unrelated parking content.

Full per-identity initial observations, sequential replays, samples, classifications, and evidence are in the companion JSON manifest.

## Recommendation

Do **not** add compatibility fixes from this batch. The unrestricted sample found no shared engine gap and achieved a 68% credible-nonempty rate. If more confidence is desired, run another disjoint unrestricted batch from the remaining 373 unsampled identities; otherwise resume planned non-audit work. Stale source contracts should not be rewritten as engine behavior.
