# Explore live audit v9 — 2026-07-22

## Scope

- Unrestricted deterministic sample: **50** enabled Explore identities.
- Seed: `NovelReader-explore-random-v9-2026-07-22`.
- Corpus: `test_booksource4.json`, SHA-256 `23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c`.
- Ranking: `SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)`.
- Excluded all **350** stable identities from audit batches 1–8; **373** eligible identities remained before selection.
- Fresh isolated SQLite database and production Explore APIs. Initial pass used four clients and a 90-second per-request timeout. Every non-pass was replayed sequentially.
- No source, parser, transport, or production behavior changed during sampling.

## Results

| Classification | Identities | Count |
|---|---|---:|
| `credible_nonempty` | 250, 792, 349, 777, 811, 70, 279, 886, 160, 256, 42, 333, 258, 887, 773, 693, 554, 588, 918, 600, 782, 619, 122, 880, 159, 501, 606, 593, 824, 810, 582, 632, 672 | 33 |
| `engine_gap` | 66, 57, 581 | 3 |
| `stale_source_contract` | 926, 340, 435, 310, 753, 686, 566 | 7 |
| `upstream_dns` | 858 | 1 |
| `upstream_http` | 347, 837, 131 | 3 |
| `blocked_or_auth` | 631, 259 | 2 |
| `source_incomplete_or_invalid` | 19 | 1 |

The exact credible identity list is authoritative in the JSON manifest. The 33 credible sources returned **932 distinct book URLs**.

## Shared engine gaps

### 1. Java Unicode escape regex compatibility — raw 66

The live page contains 20 valid book cards. Its imported rules use Java regex ranges such as `\u4e00-\u9fa5`; production forwards these to Go regexp unchanged and fails with `invalid escape sequence: \u`. This is a shared regex-normalization seam.

### 2. Multi-parent Default shorthand traversal — raw 66

On the same populated page, `.row@.col-12` returns descendants only from the first matching row/header context, while explicit `.row.1@.col-12` exposes the 20 book cards. The shorthand must preserve traversal across all matching parents.

### 3. Dotted JSON array wildcard compatibility — raw 57

The live API returns 15 records. Legado expressions `$.data.[*]` and `$.authors.[*].name` are rejected as invalid JSONPath, while the undotted forms return the expected records and author. The normalization must be generic rather than source-specific.

### 4. Root-array wildcard predicate compatibility — raw 581

The live endpoint returns a populated root array of 51 records. `$.*[?(@.novelName)]` returns no elements while `$[*]` returns all 51. The existing narrow wildcard-filter compatibility path handles object roots only and must also preserve this Legado expression over root arrays.

## Other outcomes

- **Stale contracts:** parked domains, generic-homepage redirects, changed hosts rejected by old `bookUrlPattern` values, and removed ranking routes.
- **Upstream:** one unresolved domain; two true 404 routes; one DedeCMS database failure carried inside HTTP 200.
- **Blocked/auth:** raw 631 receives 403 for its exact catalog-script requests; raw 259 receives nginx 444 in direct and Chromium requests.
- **Invalid source:** raw 19 contains malformed destructured arrow-function parameters rejected by goja and Node.

Full per-identity initial observations, sequential replays, samples, classifications, and evidence are in the companion JSON manifest.

## Recommendation

Request approval for four focused compatibility fixes through captured-fixture TDD: Java Unicode regex normalization, multi-parent Default shorthand traversal, dotted JSON array wildcards, and root-array wildcard predicates. Rerun raw 66, 57, and 581 plus adjacent syntax-family identities after implementation. Do not rewrite stale source contracts or weaken `bookUrlPattern` enforcement.
