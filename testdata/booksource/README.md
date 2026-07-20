# Booksource conformance fixtures

These are small, deterministic response bodies used by compatibility tests. They are intentionally not live-site snapshots: each fixture isolates one Legado contract so a site outage cannot hide a regression.

`manifest.json` indexes the baseline conformance responses. `explore-sources.json` contains exact raw source entries for Explore compatibility; each entry is guarded by raw index, source URL, per-entry SHA-256, and the SHA-256 of the untracked 939-source compilation it came from. Companion `explore-*` bodies execute those pinned HTML, XPath, JSON/template/JS, POST/header, paging, and WebView-option rules without live-site traffic.

Every required category has one response file and a named rule mode:

- search, detail, TOC, content: normal HTML workflow stages
- JSON, XPath, Regex: non-CSS rule evaluation
- JS POST: JavaScript request construction and response handling
- POST/GBK: request charset contract (the response body is represented as UTF-8 here; byte-decoding is tested in transport fixtures)
- cookie: session continuity
- pagination: multi-page TOC/content continuation
- WebView option: capability classification without browser execution

The dated `explore-live-audit-*.json` files are evidence snapshots, not test fixtures. Batch 4 (`explore-live-audit-v4-2026-07-21.{json,md}`) is a deterministic 50-source sample disjoint from batches 1–3; `explore-live-v4-fixes-rerun-2026-07-21.json` records its targeted shared-parser rerun. `explore-live-shared-fixes-rerun-2026-07-19.json` and `explore-live-v3-fixes-rerun-2026-07-19.json` record earlier targeted verification; live counts may change, while the reduced fixtures above remain the regression gate.

Add a deterministic test when changing a fixture. Do not replace a fixture with a live URL.
