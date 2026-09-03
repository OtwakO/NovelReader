# Authentication, Reader Storage, and Backup

**Status:** Current architecture

## Purpose

Describe the current ownership, storage, authentication, credential, and backup boundaries. Historical implementation phases and superseded designs are preserved under `docs/archive/`.

## Ownership model

- A **Reader Account** is the authenticated application identity.
- `system.db` owns accounts, roles, password hashes, application sessions, setup/recovery state, reset tokens, backup automation tokens, and deletion jobs.
- Each immutable Reader Account ID owns one self-contained reader home under `data/users/<id>/`.
- HTTP input never supplies the authoritative Reader Account ID for Reader Data access. Authentication resolves identity first; readerstore resolves the home.
- Every authenticated Reader Data operation acquires the target reader runtime. Feature modules do not construct reader paths.

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
```

`reader.db` and ordinary files are portable plaintext Reader Data: BookSources, shelf books, chapters, progress, bookmarks, caches, preferences, source profiles, and file metadata. They remain inspectable without an application secret.

`credentials.db` is separate. Reversible source credentials are encrypted using the installation-level credential key configured by NovelReader. Losing that key requires source reauthentication but must not make Reader Data unreadable.

## Authentication

- Usernames are unique case-insensitively; passwords use Argon2id.
- Browser sign-in uses opaque server-side application sessions stored only as token hashes.
- First-Administrator setup is available only while the temporary bootstrap token is configured and setup is open.
- Public registration is deployment-controlled and may require an invite code.
- Administrators can manage ordinary Reader Accounts but cannot disable, reset, or delete other Administrators through the initial web administration interface.
- Password changes and reset completion revoke existing application sessions.
- Source JavaScript receives one stable opaque device identity per Reader Account through `java.androidId()` and `java.deviceID()`. It is derived from the immutable Reader ID, shared across that reader's sources, and does not expose the Reader ID itself.
- Recovery can restore Administrator access without claiming or rewriting Reader Data.
- Reader deletion is durable, retryable, and coordinated with runtime and filesystem ownership.

## Source interaction state

Each immutable Source ID owns reader-specific state:

- non-secret Source Profile settings in portable Reader Data;
- encrypted login information, login headers, and runtime cookies in the credential store;
- transient SourceSession and browser state in the reader runtime.

Runtime cookies are managed through a typed source-profile interface, not through raw credential JSON or BookSource definition edits. Ordinary interaction responses expose only cookie scope/name metadata. Revealing or replacing cookie values requires current-password reauthentication, prevents response caching, and invalidates the affected transient source runtime after replacement.

Interactive browser closure preserves each returned cookie's URL/domain scope instead of collapsing multi-domain cookies onto the final browser URL. Browser-generated opaque HTML diagnostics use a bounded fetch/XHR mediator that rejects non-public hostname resolutions and connected server addresses, follows no redirects, and preserves timeout/body limits; Chromium web security remains enabled.

Definition edits preserve owned state. Source removal, collection replacement, restore reconciliation, and explicit reset remove state deterministically when the Source ID disappears. Credentials are never included in portable Reader Data archives.

See the completed [source interaction plan](../plans/2026-08-30-source-interaction.md) for implementation history.

## Backup boundaries

### Complete deployment backup

With NovelReader stopped, copying the complete configured `DATA_DIR` is the disaster-recovery boundary. Preserve every file, including SQLite WAL/SHM sidecars after a crash. See [development reset and cold-copy runbook](../runbooks/development-data-reset.md).

### Portable Reader Data backup

The authenticated backup module exports a versioned `.tar.gz` archive containing a consistent SQLite backup of `reader.db`, ordinary reader files, a manifest, and restore instructions. It excludes account authority, passwords, sessions, automation tokens, and source credentials.

Restore behavior:

1. upload and validate the archive in a bounded staging workspace while reading may continue;
2. prepare a complete replacement reader home;
3. briefly quiesce that reader runtime;
4. atomically replace Reader Data on the same filesystem;
5. roll back or reconcile interrupted replacement states on startup.

Prepared restores are reader-owned and expire. Backup routes authenticate before acquiring ordinary Reader Data request leases so replacement never deadlocks against the runtime being quiesced.

Scoped hash-only automation tokens expose separate backup-export and backup-restore authority. Restore-scope issuance requires current-password reauthentication.

## Invariants

- Reader Data never crosses Reader Account ownership boundaries.
- Source credentials never enter portable Reader Data archives or ordinary source-list/interaction responses.
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
