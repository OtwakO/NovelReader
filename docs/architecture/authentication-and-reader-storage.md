# Authentication, Reader Storage, and Backup

**Status:** Current architecture

## Purpose

Describe the current ownership, storage, authentication, credential, and backup boundaries. Historical implementation phases and superseded designs are preserved under `docs/archive/`.

## Ownership model

- A **Reader Account** is the authenticated application identity.
- `system.db` owns accounts, roles, password hashes, application sessions, setup/recovery state, reset tokens, backup automation tokens, and deletion jobs.
- Each immutable Reader Account ID owns one self-contained reader home under `data/users/<id>/`.
- HTTP input never supplies the authoritative Reader Account ID for Reader Data access. Authentication resolves identity first; readerstore resolves the home.
- Ordinary authenticated Reader Data requests acquire the target reader runtime. Feature modules do not construct reader paths; backup/restore uses the separate boundary described below.

`api.Server` owns authentication, health, backup/restore, TXT worker lifecycle and process shutdown. Each `readerRuntime` owns one `readerAPI` with routes registered at runtime construction; the handler binds directly to that runtime and borrows explicitly assembled `readerServices`. Authenticated requests acquire a lease, invoke the cached handler and release the lease—no Server copy or per-request dependency replacement. A replacement runtime gets a new handler and reader-specific cover scope. Candidate operations acquire their own additional lease so they can outlive the starting request. Standalone `NewServer` binds one reader explicitly and preserves its existing HTTP wrapper.

## Reader home

```text
data/users/<immutable-reader-id>/
  manifest.json
  reader.db
  credentials.db
  files/
    fonts/
    covers/
    chapter-assets/
    txt/<readable-name>--<id>/original.txt
    .work/txt/                 # disposable transfer work, not portable
```

`reader.db` and ordinary files are portable plaintext Reader Data: BookSources, shelf books, chapters, progress, bookmarks, caches, preferences, source profiles, and file metadata. They remain inspectable without an application secret. Browser-only Reader preferences are outside
this storage/backup boundary; see [Reader state](discovery-and-reading.md#reader-state).

Reader schema epoch 11 composes library-owned shared metadata/state/bookmarks, BookSource-owned
bindings/catalog/cache, managed TXT receipts/indexes and the other reader modules. Foreign keys are enabled on every pooled reader
connection. Epoch-10 or older homes and portable archives are incompatible; there is no automatic migration or
reset. Preservation and rollback instructions live in the [development reset runbook](../runbooks/development-data-reset.md).

The backend inbox capability uses `data/inbox/<reader-id>/`, outside replaceable homes and portable Reader Data. `FileStore` resolves it from the home identity; callers do not supply another reader's path. This permits bind mounts without moving them during restore. Unclaimed inputs are not deleted by home replacement/removal. TXT storage and recovery are registered; intake and provider-reading routes are not yet exposed. See the [multi-provider plan](../plans/2026-09-10-multi-provider-library.md).

`credentials.db` is separate. Reversible source credentials are encrypted using the installation-level credential key configured by NovelReader. Losing that key requires source reauthentication but must not make Reader Data unreadable.

Runtime initialization reserves a per-reader slot before opening storage or running feature initialization. In-flight initialization counts against capacity; competing requests wait rather than constructing losing instances. Quiesce/shutdown wait until initialization and any rejected-instance cleanup finish. Initialization and cleanup execute outside the manager mutex so other readers are not blocked by that mutex.

### TXT background ownership

`txtimport` runs two independent workers, at most one file per reader, with fair reader turns and durable receipt work. Idle hints retire; queued readers hold no home lease or per-file job object. `api.ReaderHomeCapacity` budgets API runtime, analysis-worker and transfer homes separately. Capacity waits are cancelled by quiesce/shutdown rather than dropping accepted work after a fixed wait.

Before serving, TXT recovery visits retained account homes (including disabled accounts, excluding deleting accounts), then starts workers. Login disabling retains accepted local work. Missing/corrupt homes or failed per-file cleanup are logged without stopping unrelated homes; no inbox originals are replayed or swept. New accounts start empty. Recovery never runs on ordinary runtime initialization or before each job. After restore it runs while that reader remains quiescent. Browser-upload admission and transfers are composed outside the API runtime cache. Bounded receipt review/control handlers use ordinary reader runtimes; no HTTP handler performs analysis.

`txtimport.Admission` owns only bounded, reader-fair transfer tickets and cancellation. It neither
opens homes nor reads files. Waiting/granted tickets expire; active transfers keep their slot until
I/O and the home lease have ended, even after cancellation. Tickets are reader-bound, single-use,
process-local permission to start a transfer—not durable receipts or inbox cleanup proofs. The
[accepted admission contract](../plans/2026-09-10-multi-provider-library.md#accepted-txt-intake-admission)
owns scheduling limits and the remaining inbox/UI work.

Restore/deletion stops and drains intake, then API runtimes and TXT workers. Successful deletion
forgets the drained barriers; failure keeps them for retry. Restore resumes fresh admission without
replaying old tickets. Shutdown joins transfers before workers and runtimes, even when another
service reports a cleanup error.

### TXT browser upload HTTP

Authenticated reader-owned routes live under `/api/imports/txt`:

- `POST /admission` and `GET /admission/{id}` allocate/refresh bounded metadata-only admission;
  waiting responses supply `Retry-After`. `DELETE /admission/{id}` cancels and joins that transfer.
- After a grant, `PUT /uploads/{id}?filename=<encoded-name>` streams one raw file with
  `Content-Type: application/octet-stream`. The grant response supplies the maximum byte size.
  Filename/size/admission checks happen before file consumption. The handler opens its own home
  lease, interrupts blocked HTTP reads on cancellation, and ends I/O/home ownership before release.
- The grant ID identifies the durable receipt once intent is recorded. A `201` means acquisition,
  not publication; background analysis may already have advanced the returned receipt state.
  A lost response is resolved with `GET /receipts/{id}`, not a blind second upload. Warnings can
  report retained analysis work or request-cleanup attention after successful acquisition.
- `GET /receipts` uses bounded ID-cursor pages and optional state filtering. Preview at
  `/receipts/{id}/preview?analysisVersion=<version>` returns a saved heading page plus a bounded
  literal sample (`start` selects its first section). No managed paths or byte offsets are exposed.
- `POST /receipts/{id}/analysis` queues version-guarded encoding/preset changes; workers do the
  actual analysis. `POST /receipts/{id}/accept` explicitly approves the reviewed version and metadata.
- `DELETE /receipts/{id}` joins any upload for that ID before pending-only discard. It cannot remove
  a currently published book; use the common book removal route. Cleanup-pending responses preserve
  the removal record and warning instead of claiming complete deletion.

Inbox scan/acquisition/confirmation routes and the import UI remain unexposed. Inbox confirmation
must retain a bounded server-owned proof; client-visible fields are not deletion authority.

## Authentication

- Usernames are unique case-insensitively; passwords use Argon2id.
- Browser sign-in uses opaque server-side application sessions stored only as token hashes.
- First-Administrator setup is available only while the temporary bootstrap token is configured and setup is open.
- Public registration is deployment-controlled and may require an invite code.
- Administrators can manage ordinary Reader Accounts but cannot disable, reset, or delete other Administrators through the initial web administration interface.
- Password changes and reset completion revoke existing application sessions.
- Source JavaScript receives one stable opaque device identity per Reader Account through `java.androidId()` and `java.deviceID()`. It is derived from the immutable Reader ID, shared across that reader's sources, and does not expose the Reader ID itself.
- Recovery can restore Administrator access without claiming or rewriting Reader Data.
- Reader deletion is durable, retryable, and coordinated with runtime and filesystem ownership. Closing runtimes remain tracked and capacity-counted until cleanup completes; same-reader acquisition/quiescence waits are cancellable, while browser/catalog draining runs outside the runtime-manager lock.
- In the frontend, `app/reader-state.ts` owns account transitions and post-restore cache resets: it aborts prior-identity requests, resets reader-owned discovery/candidate/progress state, and prevents late responses from repopulating it. Tab restoration is retained only for the recorded Reader Account ID; browser-owned appearance preferences are not cleared.

## Source interaction state

Each immutable Source ID owns reader-specific state:

- non-secret Source Profile settings in portable Reader Data;
- encrypted login information, login headers, and runtime cookies in the credential store;
- transient SourceSession and browser state in the reader runtime.

Runtime cookies are managed through a typed source-profile interface, not through raw credential JSON or BookSource definition edits. Ordinary interaction responses expose only cookie scope/name metadata. Revealing or replacing cookie values requires current-password reauthentication, prevents response caching, and invalidates the affected transient source runtime after replacement.

Interactive browser closure preserves each returned cookie's URL/domain scope instead of collapsing multi-domain cookies onto the final browser URL. The current durable URL-to-cookie-header representation does not claim complete path, expiry, SameSite, or overlapping-cookie fidelity; those require an explicit structured-cookie model if a real workflow demonstrates the need. Source-generated opaque HTML contexts hydrate stored cookie scopes into Chrome but receive no global source/login headers. For each mediated fetch/XHR, the worker asks Chrome for cookies applicable to the exact target URL and forwards only those destination-scoped cookies unless the source supplied an explicit Cookie header. Normal HTTP(S) browser sessions retain source cookie/header hydration. Browser-generated opaque HTML diagnostics use a bounded fetch/XHR mediator that rejects non-public hostname resolutions and connected server addresses, follows no redirects, and preserves timeout/body limits; Chrome's Chromium web security remains enabled.

Definition edits preserve owned state. Source removal, collection replacement, restore reconciliation, and explicit reset remove state deterministically when the Source ID disappears. Separately stored source authentication is never included in portable Reader Data archives.
Imported BookSource definitions remain lossless and may themselves contain sensitive headers,
scripts, or embedded values; portable archives are not secret-sanitized source distributions.

See the completed [source interaction plan](../plans/2026-08-30-source-interaction.md) for implementation history.

## Font file ownership

Font replacement/deletion records obsolete filenames in `font_cleanup` in the same SQLite transaction as the metadata change. Cleanup runs immediately and retries at reader-runtime opening; failures remain recorded and visible without blocking unrelated reading. SQLite write transactions serialize mutations and cleanup across Store instances. Missing files are safe to acknowledge on retry.

This covers committed file retirements, not atomicity between SQLite and new file writes: a process interruption before publishing a new file's metadata can still leave an unreferenced file. No filesystem garbage collector or general job framework is installed.

## Backup boundaries

### Complete deployment backup

With NovelReader stopped, copying the complete configured `DATA_DIR` is the disaster-recovery boundary. Preserve every file, including SQLite WAL/SHM sidecars after a crash. See [development reset and cold-copy runbook](../runbooks/development-data-reset.md).

### Portable Reader Data backup

The authenticated backup module exports a versioned `.tar.gz` archive containing a consistent SQLite backup of `reader.db`, ordinary reader files, a manifest, and restore instructions. It excludes account authority, passwords, sessions, automation tokens, and the separate source
credential store. Reserved `files/.work/` transfer work is excluded from both snapshot copying and replacement staging; it is not durable Reader Data. The imported-definition caveat above still applies.

`FileStore.LockMutation(ctx)` coordinates composite metadata/file changes with snapshots through one gate shared by all leases of a reader home. Font add/delete/cleanup acquire it before their SQLite transaction. `SnapshotHome` holds the same gate from database snapshot through local file copying and validation; archive compression/transmission happens afterward. Ordinary reads and progress writes do not acquire this gate, and other reader homes are independent. Waiting and file copying honor cancellation between I/O operations. Once font publication begins, its short metadata transaction completes independently of request cancellation to avoid rolling back metadata during a file write.

`FileStore.OpenRoot()` provides a caller-closed, confined `os.Root` for streaming and range I/O without loading entire files. It does not acquire the mutation gate implicitly.

New durable-file writers must join this boundary around the whole metadata/file operation, not individual raw file calls. The gate prevents concurrent snapshot mismatches; it does not itself provide crash recovery, reference validation, or garbage collection.

`ReaderSchema.PreparePortable` strips installation-local operational authority from the copied database on export and import, without modifying live records. TXT uses it for unresolved inbox claims; even a discarded receipt's leftover claim remains local until explicitly resolved. Copies containing such authority are rejected at publication validation.

Features can contribute `ReaderSchema.ValidatePortableFiles` to check references against the copied read-only database and confined files. Checks run after snapshot copying, after replacement staging, and before replacement publication—not on ordinary home opens. TXT supplies receipt/publication ownership and original-file checks in production; further intake work is tracked in the [multi-provider plan](../plans/2026-09-10-multi-provider-library.md). No live records are repaired or deleted by these checks.

Restore behavior:

1. upload and validate the archive in a bounded staging workspace while reading may continue;
2. prepare a complete replacement reader home;
3. quiesce and drain that reader's API runtime and TXT workers;
4. atomically replace Reader Data on the same filesystem;
5. reconcile unfinished TXT records against the new home, bounded independently of request disconnects;
6. resume the reader, returning `restored: true` plus `warnings: ["txt_recovery_incomplete"]` if TXT reconciliation could not finish. Raw diagnostic details remain in server logs; pending records remain available for retry. The frontend clears old reader caches and keeps the translated warning visible instead of reloading it away.

Replacement itself remains atomic; provider reconciliation warnings do not undo a committed replacement or bypass archive validation. Interrupted filesystem replacement states are reconciled on startup.

Prepared restores are reader-owned and expire. Backup routes authenticate before acquiring ordinary Reader Data request leases so replacement never deadlocks against the runtime being quiesced.

Scoped hash-only automation tokens expose separate backup-export and backup-restore authority. Restore-scope issuance requires current-password reauthentication.

## Invariants

- Reader Data never crosses Reader Account ownership boundaries.
- Separately managed source credentials never enter portable Reader Data archives or ordinary source-list/interaction responses.
- Source execution errors exposed to clients use stable secret-safe classifications; raw JavaScript, transport, credential, and response causes remain server-side.
- One writable `DATA_DIR` has one server owner.
- Reader-home paths are resolved only by readerstore and reject traversal/symlink escape.
- A stopped complete-data copy remains restorable.
- Online restore either commits one complete validated replacement or leaves the previous home authoritative.
- Authentication failure never reveals whether another reader's resource exists.

## Evidence and history

- Durable decision: [ADR 0001](../decisions/0001-user-owned-data-and-local-authentication.md)
- Historical design: [authentication/reader-storage design](../archive/designs/authentication-reader-storage-design-legacy.md)
- Historical checklist: [storage/authentication implementation checklist](../archive/plans/user-storage-and-authentication-checklist.md)
- Dated verification: [clean-root account workflow](../verification/ACCOUNT_SHELL_CLEAN_ROOT_2026-08-13.md)
