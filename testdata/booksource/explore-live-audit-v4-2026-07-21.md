# Explore Live Compatibility Audit — Random Batch 4

## Scope

- **Date:** 2026-07-21
- **Corpus:** `test_booksource4.json`
- **SHA-256:** `23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c`
- **Seed:** `NovelReader-explore-audit-v4-2026-07-19`
- **Sample:** 50 unique enabled Explore source URLs, disjoint from all 150 raw indices in batches 1–3.
- **Ranking:** SHA-256 of `seed + NUL + rawIndex + NUL + sourceUrl`; first 50 of 556 eligible candidates.
- **Execution:** fresh isolated database, public Explore API, first selectable category, page 1, four clients. Every initial non-pass was rerun sequentially. Browser checks were used to separate upstream/source drift from parser behavior.

## Result

| Classification | Count |
|---|---:|
| Credible non-empty | 31 |
| Shared engine gaps | 2 |
| Upstream HTTP/DNS/timeout | 5 |
| Source/site/catalog drift | 5 |
| Missing Explore result rules | 3 |
| Source/category script incompatibility | 2 |
| Sampling artifact | 1 |
| Stale source URL-pattern contract | 1 |
| **Total** | **50** |

The 31 credible sources returned **927 books** total, with per-source counts from 5 to 100. No duplicate-only or suspicious single-result success occurred.

## Engine gaps found

### Raw 830 — 腐书网

The live page returns HTTP 200 and contains 24 data rows. `tbody@tr!0` correctly selects them, but field extraction such as `td.1@a@text` fails after each `<tr>` is serialized and reparsed without table context. Wrapping that fragment in `<table><tbody>…</tbody></table>` restores the expected name. This is a shared HTML-fragment context gap, not an upstream failure.

### Raw 615 — 书客吧

The live page returns HTTP 200 and contains 30 `.flex li` books. List selection succeeds, but `img.0@title` fails while `img@title` succeeds on the same selected row. This is a shared positioned Default-element traversal gap.

## Representative non-engine outcomes

- Raw 829 is live with 15 browser rows and its extraction rules work, but its stale `bookUrlPattern` rejects the site's current `/2924/` URL form.
- Raw 322 and 840 redirect to replacement domains that return HTTP 403.
- Raw 664 and 330 fail DNS resolution.
- Raw 58 now serves an unrelated quotes site; raw 774 redirects to another domain with incompatible markup.
- Raw 26, 115, and 248 have no Explore result rules.
- Raw 671's first selectable category is intentionally random; the stable category is live, but the source has no Explore result rules.

## Deterministic verification

- `go test ./...` — pass
- `go test -race ./internal/analyzer ./internal/book` — pass
- Temporary live-body probes were removed after confirming the two engine gaps.

Machine-readable evidence: `testdata/booksource/explore-live-audit-v4-2026-07-21.json`.
