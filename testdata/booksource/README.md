# Booksource conformance fixtures

These are small, deterministic response bodies used by compatibility tests. They are intentionally not live-site snapshots: each fixture isolates one Legado contract so a site outage cannot hide a regression.

`manifest.json` is the index. Every required category has one response file and a named rule mode:

- search, detail, TOC, content: normal HTML workflow stages
- JSON, XPath, Regex: non-CSS rule evaluation
- JS POST: JavaScript request construction and response handling
- POST/GBK: request charset contract (the response body is represented as UTF-8 here; byte-decoding is tested in transport fixtures)
- cookie: session continuity
- pagination: multi-page TOC/content continuation
- WebView option: capability classification without browser execution

Add a deterministic test when changing a fixture. Do not replace a fixture with a live URL.
