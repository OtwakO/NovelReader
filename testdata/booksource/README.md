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

The dated `explore-live-audit-*.json` files are evidence snapshots, not test fixtures. Batch 4 (`explore-live-audit-v4-2026-07-21.{json,md}`) is a deterministic 50-source sample disjoint from batches 1–3; batch 5 (`explore-live-audit-v5-2026-07-21.{json,md}`) adds 15 affected-rule-family sources and 10 controls disjoint from the first 200 identities; batch 6 (`explore-live-audit-v6-2026-07-21.{json,md}`) is an unrestricted deterministic 50-source sample disjoint from all first 225 identities; batch 7 (`explore-live-audit-v7-2026-07-22.{json,md}`) targets the four v6 compatibility families with 25 identities disjoint from all first 275; batch 8 (`explore-live-audit-v8-2026-07-22.{json,md}`) is an unrestricted deterministic 50-source sample disjoint from all first 300 identities and found no shared engine gap; batch 9 (`explore-live-audit-v9-2026-07-22.{json,md}`) is another unrestricted deterministic 50-source sample disjoint from all first 350 and found four shared compatibility seams across three identities. `explore-live-v4-fixes-rerun-2026-07-21.json` records batch 4's targeted shared-parser rerun, `explore-live-v5-fixes-rerun-2026-07-21.json` records the bare-child/`java.t2s` verification, and `explore-live-v6-fixes-rerun-2026-07-22.json` records the lenient-header, source-variable, Jsoup collection, and nullable-field verification, `explore-live-v7-fixes-rerun-2026-07-23.json` records source metadata, executable headers, JSON wildcard/paging, mutable result context, and Jsoup `ownText()` verification, and `explore-live-v9-fixes-rerun-2026-07-22.json` records Java Unicode regex, multi-parent Default traversal, dotted JSON wildcard, and root-array predicate verification. `explore-live-shared-fixes-rerun-2026-07-19.json` and `explore-live-v3-fixes-rerun-2026-07-19.json` record earlier targeted verification; live counts may change, while the reduced fixtures above remain the regression gate.

Add a deterministic test when changing a fixture. Do not replace a fixture with a live URL.
