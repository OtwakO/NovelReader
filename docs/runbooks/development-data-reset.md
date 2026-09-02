# Development data reset and cold copy

NovelReader has no production users yet. The old global SQLite layout is development-only and is
not migrated into the per-reader architecture. Start fresh when the server reports a legacy data
root.

## Reset old development data

1. Stop NovelReader and any WebView worker or test process using its files.
2. If the old test state might still be useful, copy the complete `data/` directory somewhere
   outside the repository. This copy is optional and is not consumed by NovelReader.
3. Delete or rename the configured `DATA_DIR` (the default is `./data` from the server working
   directory).
4. Start NovelReader. It creates the current root metadata and `users/` directory.
5. Re-import the BookSource JSON used for the test through the normal import interface after the
   per-user cutover is available.

Do not copy `novelreader.db`, global font files, or other old state into a new reader directory.
There is intentionally no legacy migration, reset flag, dual-write mode, or automatic attachment.
The ignored `test-booksources/` corpora and tracked `testdata/booksource/` fixtures are import
inputs/test evidence, not user data, and remain independent of `DATA_DIR`.

## Complete deployment cold copy

For the current layout, a stopped copy of the complete `data/` directory is the supported complete
deployment backup:

1. Stop NovelReader and wait for the process to exit.
2. Copy/archive the complete configured `DATA_DIR`, preserving every file and name.
3. Restore by stopping NovelReader, replacing the complete `DATA_DIR`, and starting a compatible
   NovelReader version.

If NovelReader crashed and cannot restart, copy everything as-is, including SQLite `-wal` and
`-shm` files. Never copy only a main `.db` file or manually delete WAL/SHM files from the copy.
Per-reader portable backup/restore is a separate implemented feature and is not a replacement for this
complete stopped deployment copy. Portable archives exclude account authority and source credentials;
use the complete stopped deployment copy when those deployment-owned records must also be preserved.
