# Explore live audit v14 — 2026-08-11

## Scope

- Unrestricted deterministic sample: **50** identities
- Seed: `NovelReader-explore-random-v14-2026-08-11`
- Excluded: all **600** stable `(rawIndex, bookSourceUrl)` identities from Explore batches 1–13
- Eligible before selection: **121**
- Ranking: `SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)`
- Corpus: `test_booksource4.json`, SHA-256 `23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c`
- Execution: fresh disposable reader root, exact 50 frozen raw-index definitions imported through the authenticated production API, four concurrent initial clients, 90-second source timeout, then sequential confirmation of every non-pass or diagnostic
- Stopping point: page 1 of the first selectable Explore category
- Source/parser changes during sampling: **none**

The exact frozen definitions are preserved in `explore-live-audit-v14-frozen-sources-2026-08-11.json`. All 50 have unique runtime storage keys.

A preliminary run imported the whole compilation and was discarded. `bookSourceUrl` is the runtime storage key, and duplicate URLs caused later definitions to replace sampled raws 107 and 126. The authoritative run imported only the exact frozen definitions; raw 126 then returned ten books. The audit workflow skills now require exact-contract imports when compilations contain duplicate storage keys.

## Results

| Classification | Count |
|---|---:|
| `credible_nonempty` | 31 |
| `engine_gap` | 3 |
| `source_incomplete_or_invalid` | 6 |
| `upstream_http` | 4 |
| `site_drift` | 3 |
| `upstream_dns` | 3 |

The 31 credible identities returned **710 distinct book URLs**. Every initial non-pass or diagnostic was replayed sequentially. Raw 488's apparent single result was excluded: an HTTP 502 page was parsed as a synthetic book named `502 Bad Gateway`.

## Shared compatibility gap

### `bookUrlPattern` incorrectly filters parsed Search/Explore results

NovelReader compiles `BookURLPattern` inside the shared result parser and discards every parsed result whose resolved book URL does not match it (`backend/internal/book/search.go`, result loop). Upstream Legado does not use `bookUrlPattern` as a per-result Search/Explore filter. Legado uses it for Search final-detail detection and manual URL/source association; `BookList.getSearchItem` assigns parsed result URLs without this rejection.

Three independent live identities prove the shared seam:

| Raw | Current configured results | NovelReader filtered | Same parser without filter |
|---:|---:|---:|---:|
| 669 | 60 `.book-li` entries | 0 | 60 |
| 703 | 15 `.c_row` entries | 0 | 15 |
| 80 | 30 current list cards | 0 | 30 |

The patterns are stale or over-specific, but that should not erase otherwise valid Search/Explore results under Legado semantics. A future fix should remove the filter from the shared parsed-result path while preserving Search final-detail detection separately. It must not branch on source identity.

This finding also means earlier audit reasoning that classified otherwise parseable URL-pattern mismatches exclusively as stale source contracts should be reconsidered if those identities are revisited.

## Other confirmed outcomes

- `upstream_dns`: raws 864, 609, and 13. Their exact sampled hosts return SERVFAIL or NXDOMAIN through local and public DNS checks.
- `upstream_http`:
  - raws 329, 107, and 172 repeatedly return HTTP 404 for the exact sampled first-category routes; this does not claim every category is unavailable;
  - raw 488 repeatedly returns HTTP 502, and its error-page pseudo-result is not credible.
- `site_drift`:
  - raw 246 redirects to current Tencent desktop markup with 12 `ret-search-item` cards, while its fallback rules expect legacy `comic-link`/`comic-title` classes;
  - raw 707 redirects to a parked domain-sale page;
  - raw 67's current page no longer contains its `#ppluck` list contract, and detail fallback fails.
- `source_incomplete_or_invalid`:
  - raws 297 and 775 have no effective Explore `bookList`; their HTTP 444/JavaScript challenge observations are secondary;
  - raw 648 has no selectable category and no effective Explore `bookList`;
  - raws 136 and 467 use malformed arrow-function destructuring in category JavaScript and fail before requests;
  - raw 83's current body has `li.tjxs` entries, but the imported rule treats the enclosing `ul.xbk` as one item while per-item XPath fields search descendants incompatibly. Removing `bookUrlPattern` still produces zero, so it is not included in the shared-gap proof.

## Browser evidence

Playwright was **not used**. Captured production observations, direct DNS/HTTP, current response bodies, upstream Legado inspection, and reduced production-runtime probes resolved all ambiguities. Browser rendering would not have added evidence needed for these classifications.

## Resolution

After explicit approval, the shared `bookUrlPattern` result-filter divergence was fixed separately from the audit using public-seam TDD. The shared Search/Explore result parser no longer rejects complete list items by `bookUrlPattern`; no source-specific exceptions were added. A clean authenticated production replay against the exact frozen definitions returned 60, 15, and 30 distinct books for raws 669, 703, and 80 with no diagnostics.

Post-fix evidence: `explore-live-v14-fixes-rerun-2026-08-11.json`.

Keep the exact-contract import rule for all future audits because the corpus contains duplicate `bookSourceUrl` values.
