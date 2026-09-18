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

Reader schema epoch 13 composes library-owned shared metadata/state/bookmarks and independent last-read timestamps, BookSource-owned
bindings/catalog/cache, managed TXT files/interpretations/indexes and the other reader modules. Foreign keys are enabled on every pooled reader
connection. Epoch-12 or older homes and portable archives are incompatible; there is no automatic migration or
reset. Preservation and rollback instructions live in the [development reset runbook](../runbooks/development-data-reset.md).

The backend inbox capability uses `data/inbox/<reader-id>/`, outside replaceable homes and portable Reader Data. `FileStore` resolves it from the home identity; callers do not supply another reader's path. This permits bind mounts without moving them during restore. Unclaimed inputs are not deleted by home replacement/removal. TXT intake, review, reading and removal are connected. Custom patterns and explicit published-reparse controls are available through the shared interpretation workflow. See the [multi-provider plan](../plans/2026-09-10-multi-provider-library.md).

`credentials.db` is separate. Reversible source credentials are encrypted using the installation-level credential key configured by NovelReader. Losing that key requires source reauthentication but must not make Reader Data unreadable.

Runtime initialization reserves a per-reader slot before opening storage or running feature initialization. In-flight initialization counts against capacity; competing requests wait rather than constructing losing instances. Quiesce/shutdown wait until initialization and any rejected-instance cleanup finish. Initialization and cleanup execute outside the manager mutex so other readers are not blocked by that mutex.

### TXT interpretations

`txt_files` owns acquisition/removal and the never-reused generation counter. `txt_interpretations`
owns the candidate/active roles and their options, diagnostics and result metadata; `txt_sections`
is keyed by file and generation. The initial candidate is created atomically when acquisition
finishes. Workers claim it without changing its generation, and acceptance promotes it alongside
library publication. Reading uses only the active role and a library-owned revision read in the
same database snapshot. `txt_receipts` projects existing import response states without duplicating
them in storage; a publication stays published while another interpretation is prepared.

Supersession checks occur before decoding, every roughly 1 MiB of source reads, and at the final
result transaction. Quiescent recovery requeues interrupted candidates without touching active data.
Removal drops both interpretation roles when hiding the library item; failed byte cleanup retains
only the file/cleanup record.

Published reparse is implemented at the `txtstore` boundary and exposed through authenticated HTTP
controls and the Book Detail re-analysis flow.
`QueueReparse`/`DiscardReparse` compare the active content revision and exact candidate generation
(zero means observed absence when queueing). They change neither readable content nor library revisions.
`ReviewReparse` reuses bounded preview sampling and rechecks the candidate role after file I/O.
`ReparseImpact` reads interpretation and reading state in one snapshot, resolving only referenced
sections by exact byte-range equality and equal encoding. It does not infer correspondence from titles
or ordinal proximity, and never revives older/orphaned bookmarks.

`ApplyReparse` validates the managed original under the file-mutation gate, then reserves SQLite's
writer through a TXT-owned row before reading library state. It recomputes mappings, checks reviewed
versions, updates library-owned progress/bookmarks, and swaps interpretation roles in one transaction.
Unmapped progress requires an explicit section choice at position zero; unresolved bookmarks retain
original location data. Content/state revisions advance once; an already-active generation returns
current state without another mutation. Revision-coherent reader loading and qualified navigation are
implemented and connected to the reparse controls.

The shared reader loads book state and catalog as a revision-matched pair, retrying that pair once
before requiring an explicit reopen. Catalog failures retain BookSource recovery metadata without
initializing reading state. Catalog links, saved resumes and bookmark actions carry their content
revision. Books without a first catalog open the current resume without inventing a section location;
unqualified legacy URLs similarly mean current-catalog navigation, not historical recovery.
A stale qualification is rejected before fetching chapter content. Later content, prefetch or state-write
conflicts stop navigation/writes, dispose the session's chapter cache and offer **Reopen current saved
location**. Already displayed prose may remain visible with a warning; it is not silently remapped.
Write invalidation is per book and retains its pending barrier so reopening drains old work before
binding fresh state. No background polling, cross-tab event bus or generic invalidation framework.

Analysis failures persist stable categories in the existing interpretation error column. The parser
identifies encoding, empty/non-text input and section-limit failures with typed errors; txtstore adds
storage failures and returns raw causes to the worker for server diagnostics. Initial-import and
reparse responses expose only an allowlisted `errorCode`, retaining `hasError` for compatibility.
Older raw error text receives generic guidance, never string-based classification or disclosure.
Both review screens share translated correction guidance. Cancellation remains queued work, not a
terminal analysis failure; no schema change or migration is needed.

### TXT background ownership

`txtimport` runs two independent workers, at most one file per reader, with fair reader turns and durable candidate work. Idle hints retire; queued readers hold no home lease or per-file job object. `api.ReaderHomeCapacity` budgets API runtime, analysis-worker and transfer homes separately. Capacity waits are cancelled by quiesce/shutdown rather than dropping accepted work after a fixed wait.

Before serving, TXT recovery visits retained account homes (including disabled accounts, excluding deleting accounts), then starts workers. Login disabling retains accepted local work. Missing/corrupt homes or failed per-file cleanup are logged without stopping unrelated homes; no inbox originals are replayed or swept. New accounts start empty. Recovery never runs on ordinary runtime initialization or before each job. After restore it runs while that reader remains quiescent. Browser-upload admission and transfers are composed outside the API runtime cache. Bounded receipt review/control handlers use ordinary reader runtimes; no HTTP handler performs analysis.

`txtimport.Admission` owns only bounded, reader-fair transfer tickets and cancellation. It neither
opens homes nor reads files. Waiting/granted tickets expire; active transfers keep their slot until
I/O and the home lease have ended, even after cancellation. Tickets are reader-bound, single-use,
process-local permission to start a transfer—not durable receipts or inbox cleanup proofs. The
[accepted admission contract](../plans/2026-09-10-multi-provider-library.md#accepted-txt-intake-admission)
owns the scheduling limits.

Restore/deletion stops and drains intake, then API runtimes and TXT workers. Successful deletion
forgets the drained barriers; failure keeps them for retry. Restore resumes fresh admission without
replaying old tickets. Shutdown joins transfers before workers and runtimes, even when another
service reports a cleanup error.

### TXT published-reparse HTTP

`/api/books/{id}/txt/reparse` is a reader-runtime, no-store TXT control resource with the existing
30-second deadline and strict bounded JSON decoder. `GET` returns book name, active generation/options
and optional candidate status, not raw storage errors. `POST` prepares/replaces a candidate using
`contentRevision`, `generation` (explicit zero means absence) and interpretation options; it wakes the
existing analysis pool rather than parsing inside HTTP. `DELETE` discards only the named candidate.

- `GET /preview?generation=…&start=…&limit=…` shares the bounded heading/sample DTO with initial
  import. Its existing `analysisVersion` field identifies the interpretation generation, not the
  library content revision; no native paths or byte offsets are exposed.
- `GET /impact?generation=…` returns the coherent reviewed generations/revisions, resume correspondence
  and preserved/unresolved bookmark counts. The client never supplies a mapping table.
- `POST /apply` requires `generation`, `activeGeneration`, `contentRevision` and `stateVersion`, plus
  an optional `resumeChapter` choice. Reading-state conflicts return `409 state_changed`; candidate
  conflicts return `409 txt_state_changed`. Missing/invalid resume choices have explicit safe codes.
  Successful retries report `alreadyApplied` without advancing state again.

The Book Detail entry opens a separate `TXTReparseView`, sharing only interpretation-option and preview
presentation with initial import. Preparation, discard and Apply are distinct actions; Apply/discard
have explicit confirmations. Polling occurs only while the visible page has processing work. Unknown
mutation outcomes block further mutations until refresh; active-generation status resolves a lost Apply
response. Refresh preserves unsent drafts and renews impact/confirmation rather than silently accepting
new state. After Apply, only the affected book's writer state is invalidated; normal reader loading
selects the newly saved resume. No extra worker, lease owner, revision history or schema change.

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
- `POST /receipts/{id}/analysis` queues version-guarded encoding/preset changes; `preset: "custom"`
  requires a `pattern` string, rejected with other presets. Go/RE2 syntax, the 2 KiB UTF-8 bound and
  non-empty-match rule are checked before replacing a candidate. Invalid patterns return HTTP 400
  `txt_invalid_pattern` without revoking the saved result. Workers compile once per analysis and use
  longest whole-line matching on bounded, trimmed complete lines; captures never replace titles.
  Receipt `pattern` retains the exact requested expression, separately from the preview's resolved
  method (which may be generated fallback). `POST /receipts/{id}/accept` explicitly approves the
  reviewed version and metadata. Published files cannot be re-analyzed through this pending-only API.
- `DELETE /receipts/{id}` joins any upload for that ID before pending-only discard. It cannot remove
  a currently published book; use the common book removal route. Cleanup-pending responses preserve
  the removal record and warning instead of claiming complete deletion.

### TXT inbox HTTP

Inbox controls share `/api/imports/txt` and the authenticated reader boundary:

- `GET /inbox` streams directory entries in chunks, retaining only a bounded name-ordered page;
  `after`/`limit` select pages. Its `directory` is relative to `DATA_DIR`. Invalid/temporary suffixes
  are ignored; directories, symlinks and oversized TXT entries are not consumable. Claims for the
  selected names are joined in one query. `GET /inbox/claims` separately pages the durable journal,
  including missing inputs and discarded receipts. A scan does not consume files or prove completion.
- The producer finishes copying before scan/acquire. After an admission grant,
  `POST /inbox/acquisitions/{id}?filename=<encoded-name>` acquires one completed file outside API
  runtime capacity. It preserves rename-first and cross-device streaming fallback. A `201` with
  `txt_inbox_cleanup_pending` means managed bytes were acquired but the retained claim needs review;
  it is not permission to import the same name again.
- `POST /inbox/claims/{id}/review` requires that claim's acquisition to have ended. It may settle only
  that receiving receipt with existing managed-file recovery, never recover the whole live home.
  The result contains display fields and an opaque token for the retained native `InboxReview`.
- `POST /inbox/reviews/{token}/confirm` deletes only a still-matching duplicate with a surviving
  managed original. `/release` instead preserves external bytes and releases the claim for a fresh
  acquisition. `DELETE /inbox/reviews/{token}` abandons approval without touching files or claims.
  Resolution consumes the token; a changed/expired proof requires explicit fresh review.

The HTTP inbox owner bounds concurrent filesystem controls independently of transfers and retains
only bounded reader-owned proof metadata, not home leases or file contents. Native checks still
bind approvals to the exact database lifetime; cache eviction or reader replacement can invalidate
a proof before its maximum expiry. Runtime drain invalidates reader proofs before restore/removal;
shutdown clears them after controls finish. Client JSON/flags never authorize deletion. Limits are
recorded in the [accepted inbox checkpoint](../plans/2026-09-10-multi-provider-library.md#inbox-http-checkpoint).
### TXT frontend ownership

`frontend/src/features/imports/ImportWorkspace.vue` belongs to the dedicated Local Import page:
choose files, follow one progress list, handle exceptions inline and open the current saved reader
location directly. Shelf provides only a navigation action; it does not mount the workspace.
History and server-inbox controls use the shared Search-style disclosure and load only when opened.
The Vue Options API and [shared UI owners](../../frontend/src/ui/README.md) are reused. Review takes a
receipt prop rather than owning navigation; legacy `/imports/:id` links redirect to `/imports?review=id`.

`import-queue.ts` is the only browser import owner. Its Pinia lifetime survives page navigation,
holds lightweight File references or inbox names, and starts one admitted byte transfer at a time.
One independent round-robin loop checks at most 16 owned receipts per round, with a 1.5-second pause;
there is no timer per file. It automatically accepts only warning-free, ready initial generations from
this tab's newly selected files queued with review-before-adding disabled. The default is review on;
`import-preferences.ts` owns the browser-local preference shared by Settings and Local Import. Each
queue entry snapshots it at selection; later toggles never approve existing work. Ready entries awaiting
confirmation leave the background loop. Different generations, review warnings and failures stop automatic
approval. Uncertain acquisitions/additions are not replayed; inline review reads authoritative status.
The server API still requires explicit acceptance—there is no server-side auto-publication policy.

File references are released at acquisition. Outages preserve unsent selections and pause uploads;
analysis/addition of acquired work may continue. Closing/reloading loses unsent files and this tab's
automatic-approval intent, not durable receipts. Earlier receipts require explicit Add. No browser file
decoding/copies, persistent browser file storage, generic workflow framework or schema change.

The application reader-state reset cancels both queue activities alongside existing reader features.
A library revision counter refreshes a visible shelf quietly after additions; stale list responses
cannot overwrite a newer refresh. Views own bounded page/preview requests through `import-task.ts`
and cancel them on unmount. History filters/cursors remain local, and review expands in the same
workspace. History displays all returned receipts, including this tab's transfers; a revision change
during a list request triggers a fresh read rather than dropping completion notifications.
Explicit mutations exclude background refresh. One-click bulk acceptance of historical
ready receipts retains selected analysis versions rather than silently approving a refreshed version. Inbox UI sends only retained opaque tokens and requires fresh explicit
review after an invalid/changed approval; displayed flags are not authority. The custom-pattern
field is shown only for that heading choice, retains drafts and displays server validation beside
its input. Pattern edits invalidate acceptance until options and saved preview match. Switching to
another preset submits no pattern; the browser never tries to validate Go syntax with JavaScript
regular expressions.

Cleanup remains provider-owned. Unpublished import discard uses its pending-only API. Retained
publication-removal records are reachable from Imports, but retry through common library removal,
not pending discard; hiding a library item does not make its original an unpublished acquisition.

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

Archive export and restore share the fixed limits defined in `backup/archive_limits.go` (operator limits are listed in the [README](../../README.md#back-up-the-deployment)). Both count logical tar entries, including directories and manifest/help files, and their payload sizes; export also bounds all compressed output, including gzip finalization. Exceeding a limit returns an error, even if part of a download has already been written. These archive-format checks do not replace provider-specific portable validation.

`FileStore.LockMutation(ctx)` coordinates composite metadata/file changes with snapshots through one gate shared by all leases of a reader home. Font add/delete/cleanup acquire it before their SQLite transaction. `SnapshotHome` holds the same gate from database snapshot through local file copying and validation; archive compression/transmission happens afterward. Ordinary reads and progress writes do not acquire this gate, and other reader homes are independent. Waiting and file copying honor cancellation between I/O operations. Once font publication begins, its short metadata transaction completes independently of request cancellation to avoid rolling back metadata during a file write.

`FileStore.OpenRoot()` provides a caller-closed, confined `os.Root` for streaming and range I/O without loading entire files. It does not acquire the mutation gate implicitly.

New durable-file writers must join this boundary around the whole metadata/file operation, not individual raw file calls. The gate prevents concurrent snapshot mismatches; it does not itself provide crash recovery, reference validation, or garbage collection.

`ReaderSchema.PreparePortable` strips installation-local operational authority from the copied database on export and import, without modifying live records. TXT uses it for unresolved inbox claims; even a discarded receipt's leftover claim remains local until explicitly resolved. Copies containing such authority are rejected at publication validation.

Features can contribute `ReaderSchema.ValidatePortableFiles` to check references against the copied read-only database and confined files. Checks run after snapshot copying, after replacement staging, and before replacement publication—not on ordinary home opens. TXT validates file/publication ownership, legal interpretation roles/generations, completed section ranges and original files. Interrupted acquisition/removal records retain their lifecycle-specific allowances for missing or damaged originals. Validation never decodes the novel or executes stored patterns; pending reparse work is tracked in the [multi-provider plan](../plans/2026-09-10-multi-provider-library.md). No live records are repaired or deleted by these checks.

Restore behavior:

1. upload and validate the archive in a bounded staging workspace while reading may continue;
2. prepare a complete replacement reader home;
3. quiesce and drain that reader's API runtime and TXT workers;
4. atomically replace Reader Data on the same filesystem;
5. reconcile unfinished TXT records against the new home, bounded independently of request disconnects;
6. resume the reader, returning `restored: true` plus warnings when recovery or cleanup cannot finish (`txt_recovery_incomplete`, `reader_cleanup_pending`, or `restore_staging_cleanup_pending`). Raw diagnostics remain server-side; retained records/files remain recoverable. A typed readerstore cleanup error distinguishes committed replacement from a failure before publication, so cleanup failure cannot invite replay of a completed restore. The frontend keeps the translated warning visible instead of reloading it away.

Replacement itself remains atomic; provider reconciliation warnings do not undo a committed replacement or bypass archive validation. Interrupted filesystem replacement states are reconciled on startup.

Restore operations are reader-owned and process-local. The existing status GET is no-store and returns
`prepared`, `committing`, `committed` (with the original result/warnings), or `failed`. One operation per
reader is retained; preparation and terminal results expire after 30 minutes, and a fresh preparation
can supersede a terminal result. A committing operation cannot be canceled, superseded, expired or
replayed; shutdown joins it before cleaning staging. Commit attempts consume their preparation even
on failure. Status recovery never invokes replacement again. A missing record means unavailable
outcome evidence—not success or failure. No durable operation ledger or schema change is involved.
Backup routes authenticate before acquiring ordinary Reader Data request leases so replacement never
deadlocks against the runtime being quiesced.

In the initiating browser tab, a reader-bound session marker is retained before commit. The existing
reader-state reset retires uploads and cached feature state, and the transport's reader request lifetime
stays aborted while outcome is unresolved. Auth/restore controls use a separate lifetime that still
cancels on reader identity/reset changes. Navigation/reload returns to recovery. A `prepared` status
alone cannot prove that an earlier POST will not arrive later: explicit cancellation retires that
preparation before releasing the client barrier. Missing records require acknowledgement and fresh
state; connection failures keep the barrier. This is tab-local recovery, not cross-client coordination
or a durable completion guarantee for automation clients.

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
