# Development Notes

### [2026-08-06] Chapter image decoder context differs from normal rule evaluation
- **Context**: LC-015 added stored chapter-image fetching and portable `imageDecode` execution.
- **Change**: Chapter image scripts receive downloaded bytes as `result` and the resolved image URL as `src`. JSON-object `jsLib` values that map library names to remote URLs are treated as metadata rather than executable JavaScript for this boundary.
- **Reason**: The general JavaScript evaluator defaults `src` to the same value as `result`, but active Bilibili image rules inspect URL query data through `src`. Prepending a remote-library map as code causes a syntax error; the active portable rules using those maps rely only on existing built-in bridges.
- **Verified**: API tests assert resolved `src`, source/URL headers, bounded byte transformation, and URL redaction. Active-corpus tests distinguish syntax/bridge failures from fixture-dependent AES/format failures.
- **Watch out**: Supporting remote `jsLib` library maps later requires an explicit bounded fetch/cache/trust design; do not silently execute their URLs.

### [2026-08-06] Data-root containment must resolve symlinks
- **Context**: Phase 1.1 added the fail-closed storage-root classifier before the legacy SQLite database opens.
- **Change**: The root, manifest, `users/` directory, and database path reject symlink escapes; `DATABASE_PATH` must resolve inside `DATA_DIR`.
- **Reason**: Lexical `filepath.Rel` checks can classify `data/db-link/file.db` as contained even when `db-link` targets another filesystem location. That would make a stopped copy of `data/` incomplete.
- **Verified**: Symlinked root/manifest/users/database regressions pass, concurrent root initialization passes under the race detector, and the complete backend Go suite passes.
- **Watch out**: Future reader file paths must use the `readerstore` containment seam rather than reimplementing lexical path checks.
