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

### [2026-08-06] Reader storage keeps SQLite and files behind owned seams
- **Context**: Phase 1.2 introduced portable per-reader homes and a bounded database-handle cache.
- **Change**: SQLite filenames are escaped `file:` URIs with driver-supported `_pragma` parameters, staged homes are published only after both databases validate at the supported schema version, and reader files expose rooted read/write/remove operations instead of filesystem paths.
- **Reason**: Appending query parameters to a raw filename truncates valid paths containing `?` or `#`, and returning a prevalidated path cannot prevent later symlink traversal by a caller.
- **Verified**: Hostile root names, effective SQLite pragmas, interrupted staging recovery, symlink escapes, cache pressure/wake-up, race tests, the complete backend suite, and Linux/Windows compilation pass.
- **Watch out**: Feature stores must use `Home.DB()` and `Home.Files()` directly; do not reconstruct reader database/file paths outside `readerstore`.

### [2026-08-06] Legacy development databases are reset, not migrated
- **Context**: Phase 1.3 originally planned an automated backup-and-reset path for the old global database layout.
- **Change**: Automated legacy backup, migration, reset flags, verification, and rollback machinery were removed from scope. Startup remains fail-closed and points developers to a manual fresh reset; test BookSource JSON remains independent import input.
- **Reason**: NovelReader has no production users or irreplaceable legacy Reader Data. Permanent migration code would preserve debt for disposable internal test state.
- **Verified**: The old-root refusal test proves no database is created or existing file changed, the startup message includes reset/re-import guidance, and the complete backend suite passes.
- **Watch out**: Do not reintroduce legacy conversion during Phase 2. Replace global stores directly and delete their schema/configuration once the authenticated per-reader cutover is complete.
