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

### [2026-08-06] System schema is validated canonically before writable open
- **Context**: Phase 2.1 introduced `system.db`, which stores password/session/reset hashes and durable setup/deletion state.
- **Change**: Startup compares every declared SQLite catalog object against the canonical versioned schema through a read-only connection before enabling WAL or opening feature storage. New databases are staged with owner-only permissions; partial stages are rebuilt, while newer authoritative schemas are refused.
- **Reason**: Version and column-name checks can accept lookalike databases that omit role/foreign-key/uniqueness constraints or hide Reader Data tables. Applying writable pragmas before full validation would modify unsupported state.
- **Verified**: Regressions cover malformed, newer, unconstrained, Reader-Data-polluted, and hidden `sqlite_*` catalog schemas; refusal remains byte-for-byte and creates no WAL/SHM or feature database. Focused race/vet and the complete backend suite pass.
- **Watch out**: Every future `system.db` migration must update the canonical schema and schema version together. Do not loosen validation into a partial table/column allowlist.

### [2026-08-06] Password hashing uses a light fixed Argon2id profile
- **Context**: The original authentication design proposed a 64 MiB Argon2id profile, but NovelReader is a small self-hosted reader and the operator requested lower resource use.
- **Change**: Password hashing now uses Argon2id at 19 MiB, 2 iterations, parallelism 1, 16-byte random salts, and 32-byte output, with two non-blocking process-wide work slots. Username and password Unicode policy is implemented separately from account persistence.
- **Reason**: This keeps modern memory-hard password protection while limiting simultaneous hashing to roughly 38 MiB instead of roughly 128 MiB. A fixed profile avoids unsafe operator configuration.
- **Verified**: Tests cover canonical Unicode usernames, malformed UTF-8, exact password code-point boundaries, fresh PHC salts, wrong/malformed hashes, dummy verification, overload across hasher instances, and cancellation before/during work. Focused repeated/race/vet checks and independent review pass.
- **Watch out**: The fixed dummy PHC hash is deliberately non-secret and only equalizes unknown-user work. All future password paths must use `PasswordHasher`; do not call Argon2 directly or add unbounded constructor/setup hashing.

### [2026-08-06] Account workflows own immutable identity allocation
- **Context**: Phase 2.2 added account persistence before registration and first-Administrator setup exist.
- **Change**: Cross-store workflows supply a canonical UUIDv4 `UserID` to reader-account creation; auth validates and stores it rather than generating another identity. Ordinary creation is reader-only. System transactions use immediate SQLite locking so concurrent identity checks and normalized-username inserts are serialized. Password replacement hashes before its short transaction, then updates the credential and revokes every live session atomically.
- **Reason**: Setup must reserve one durable ID before publishing `data/users/<UserID>/`; generating an unrelated ID inside auth would require a second reservation API. Immediate transactions keep duplicate-ID classification stable without parsing SQLite error text or allowing a concurrent writer to change the conflict.
- **Verified**: Tests cover canonical username races, duplicate ID/username precedence, invalid input before hashing, generic real/dummy credential failures, overload/cancellation propagation, stored role/status/hash corruption, legal account states, complete session revocation, and rollback when revocation fails. Repeated/race/vet checks and independent review pass.
- **Watch out**: Administrator creation must remain inside setup/recovery workflows; do not widen `CreateReaderAccount` with a caller-selected role. Keep expensive Argon2 work outside system transactions.
