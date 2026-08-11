# BookSource conformance fixtures

These are small deterministic inputs for automated compatibility tests. They isolate a Legado contract so live-site changes cannot hide regressions.

- `core/manifest.json` indexes baseline Search, Book Info, TOC, content, JSONPath, XPath, regex, request, cookie, pagination, and WebView-option fixtures.
- `explore/explore-sources.json` pins exact raw Explore sources by raw index, source URL, source SHA-256, and corpus SHA-256. Companion `explore-*` files exercise their HTML, JSONPath, XPath, POST/header, paging, and WebView contracts.

A fixture change requires a deterministic test update. Do not replace a fixture with a live URL or put dated audit output here.
