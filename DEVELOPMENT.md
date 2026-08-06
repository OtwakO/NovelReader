# Development Notes

### [2026-08-06] Chapter image decoder context differs from normal rule evaluation
- **Context**: LC-015 added stored chapter-image fetching and portable `imageDecode` execution.
- **Change**: Chapter image scripts receive downloaded bytes as `result` and the resolved image URL as `src`. JSON-object `jsLib` values that map library names to remote URLs are treated as metadata rather than executable JavaScript for this boundary.
- **Reason**: The general JavaScript evaluator defaults `src` to the same value as `result`, but active Bilibili image rules inspect URL query data through `src`. Prepending a remote-library map as code causes a syntax error; the active portable rules using those maps rely only on existing built-in bridges.
- **Verified**: API tests assert resolved `src`, source/URL headers, bounded byte transformation, and URL redaction. Active-corpus tests distinguish syntax/bridge failures from fixture-dependent AES/format failures.
- **Watch out**: Supporting remote `jsLib` library maps later requires an explicit bounded fetch/cache/trust design; do not silently execute their URLs.
