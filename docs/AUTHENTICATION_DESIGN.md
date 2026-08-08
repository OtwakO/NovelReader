# Authentication, Reader Ownership, and Portable Storage Design

## Goal and invariants

Add a real application-user boundary before durable source login is implemented. A signed-in Reader Account may access only its own sources, files, shelf, chapters, caches, progress, bookmarks, history, preferences, source login state, and live source sessions.

The following are hard architectural invariants:

1. **Reader Data is portable plaintext.** NovelReader never encrypts book sources, original source JSON, shelf data, chapters, caches, progress, bookmarks, history, preferences, fonts, covers, or chapter assets.
2. **Only credential material is protected.** Passwords are one-way hashed. Application/reset tokens are stored only as hashes. Reversible source-site cookies, login headers, and future source credentials are encrypted.
3. **Cold copy is a supported backup.** With NovelReader stopped, copying the complete `data/` directory must produce a restorable deployment backup without running NovelReader.
4. **Each reader directory is independently recoverable.** Losing authentication state or the source-credential encryption key must not make `reader.db` or ordinary files unreadable.
5. **Ownership is enforced at the storage seam.** HTTP input never supplies authoritative user IDs, and feature code never constructs reader paths.
6. **Authentication and ownership ship atomically.** Existing globally keyed Reader Data routes remain unavailable until per-user storage is active; authentication-looking routes must never front global stores.
7. **The cutover removes the legacy architecture.** This pre-release project has no production users, so completed replacement slices delete superseded global schemas, startup wiring, stores, paths, configuration, adapters, and tests instead of maintaining dual modes or compatibility remnants.

This is a pre-release structural change with no production users or irreplaceable legacy Reader Data. Existing unowned development state is discarded by stopping NovelReader, optionally copying it for manual inspection, deleting or renaming `DATA_DIR`, and re-importing test BookSources. NovelReader intentionally provides no legacy migration or automatic reset machinery. Temporary transition code is permitted only when a phase cannot remain runnable without it, must be explicitly marked, and is removed at the Phase 2 atomic cutover.

## Product decisions

- Sign-in uses a unique, case-insensitive username and password.
- Registration is deployment-controlled and may require one invite code.
- The first Administrator is created through a one-time browser setup protected by an environment bootstrap token.
- Account roles are stored in `system.db`; the initial roles are only `reader` and `admin`.
- Administrators may manage ordinary Reader Accounts. Administrator disable/reset/delete is not exposed through the web interface initially.
- Administrators may disable/re-enable ordinary accounts, issue one-time password reset tokens, and permanently delete ordinary accounts and their Reader Data.
- A temporary environment recovery token can create a replacement Administrator or reset an Administrator password without modifying Reader Data.
- Browser sessions rotate during use and have a 30-day absolute lifetime.
- Source credentials are never included in portable per-reader exports.

## Storage layout

```text
data/
├── system.db
└── users/
    └── <immutable-user-id>/
        ├── manifest.json
        ├── reader.db
        ├── credentials.db
        └── files/
            ├── fonts/
            ├── covers/
            └── chapter-assets/
```

Directories use an immutable random user ID, never a username. Usernames may change and are unsafe filesystem identities.

### `system.db`

Deployment identity and control data only:

- Reader Accounts, normalized usernames, roles, status, and password hashes;
- Application Session token hashes;
- password-reset token hashes;
- setup/recovery state;
- durable account-deletion jobs.

It must not contain sources, books, reading activity, preferences, or file metadata.

### Per-user `reader.db`

An ordinary unencrypted SQLite database containing all meaningful Reader Data:

- imported sources and lossless original BookSource JSON;
- shelf books and chapter lists;
- progress and current reading position;
- bookmarks and future reading history;
- cached chapter text and image-block metadata;
- preferences and font/file metadata;
- future non-secret reader-owned feature data.

No application secret is required to open or inspect `reader.db` with standard SQLite tooling.

### Per-user files

Large or naturally file-based data remains under `files/`. Database records use relative logical paths or stable file IDs, never host-specific absolute paths. A reader directory can therefore move between hosts or mount points unchanged.

### Per-user `credentials.db`

This database contains only source-site credential records and non-secret lookup metadata. Every reversible secret value is individually encrypted with AES-256-GCM using a versioned `NOVELREADER_SECRET_KEY`; the SQLite file itself is not treated as opaque encrypted storage.

Associated data binds each ciphertext to the immutable user ID, source identity, record kind, and format version. Losing the key makes only source logins unusable. NovelReader must continue opening `reader.db`, quarantine or ignore undecryptable credential records, and report that source reauthentication is required.

`credentials.db` is excluded from all portable per-reader exports. It may be included in a complete deployment backup because its secret values remain encrypted.

## Storage module and future features

Create `backend/internal/readerstore/` as the deep module that owns reader-directory discovery, path validation, database lifecycle, schema versions, cold-copy invariants, import/export staging, and deletion coordination.

Its external interface should remain small and use a typed immutable account ID:

```go
type UserID string

type Home interface {
    ID() UserID
    DB() *sql.DB
    Files() FileStore
}

type Manager interface {
    Open(ctx context.Context, userID UserID) (Home, error)
    Create(ctx context.Context, userID UserID) error
    Export(ctx context.Context, userID UserID, dst io.Writer, options ExportOptions) error
    Import(ctx context.Context, userID UserID, src io.Reader, policy ConflictPolicy) error
    BeginDelete(ctx context.Context, userID UserID) (DeletionID, error)
}
```

The exact Go shapes may be tightened during implementation, but the seam rules are fixed:

- API handlers and feature packages never join `data/users/...` paths.
- Authentication returns a typed `UserID`; it does not open feature databases.
- Book/source/font features operate on a `readerstore.Home` or its database/file interfaces, not global stores.
- One manager owns bounded connection caching and closes idle user databases; do not create an unbounded database pool or one store graph per request.
- File operations reject traversal and remain inside the opened reader directory.
- Future features add their own tables/migrations to `reader.db` and optional namespaced file directories without changing authentication tables or handler path logic.
- Cross-user joins are deliberately absent. Administrator account lists read `system.db`; administrators do not query reader databases merely to enumerate accounts.
- Do not add a generic key/value dumping ground. A feature owns its schema and public interface; `readerstore` owns location, lifecycle, migration execution, backup, and recovery mechanics.

Reader-schema migrations are versioned and coordinated by `readerstore`. A future feature contributes a named, ordered migration through one migration registry rather than adding startup SQL in unrelated handlers or stores. Unsupported newer schema versions fail clearly and never trigger destructive downgrade behavior.

## Ownership and request flow

Authentication middleware places one immutable identity in request context:

```go
type Identity struct {
    UserID   readerstore.UserID
    Username string
    Role     Role
}
```

Handlers obtain it only through `auth.IdentityFromContext`; they never parse identity cookies themselves or accept an authoritative `user_id` from route/query/body input.

A request opens the authenticated reader’s `Home` through `readerstore.Manager`. Existing feature interfaces are revised to operate within that home. Resource IDs remain useful locators but are not authorization.

In-memory Source Session keys begin with immutable `UserID`, followed by source and workflow identity. Account disable/deletion purges all matching Source Sessions.

Tests must prove that equal source URLs and resource IDs in two reader databases remain isolated and that cross-user reads, writes, deletes, covers, chapter images, caches, and files return not-found without revealing existence.

## Authentication module

Create `backend/internal/auth/` as a deep module over `system.db`. Ordinary account operations and administrator operations use separate interfaces so feature handlers cannot call destructive account methods accidentally.

Do not introduce generic RBAC initially. HTTP authorization has two seams: `RequireIdentity` and `RequireAdmin`. Roles are stored in `system.db` and limited to `reader` and `admin`; future permissions can deepen this module without changing reader storage ownership.

## Initial administrator setup

When no Administrator exists, normal registration and all Reader Data routes fail closed. The deployment enters setup mode only when `ADMIN_BOOTSTRAP_TOKEN` is configured.

- `/setup` requires the random bootstrap token and same-origin HTTPS outside explicit development mode.
- The browser form chooses the first Administrator username and password; actual account passwords are never environment variables.
- Setup uses a singleton row in `system.db` and `BEGIN IMMEDIATE` to serialize claims. The transaction rechecks that no active Administrator or claimed setup exists, records one immutable proposed user ID, and rejects every concurrent loser.
- Reader storage is created under a staging directory for that proposed ID, initialized and validated, then published by same-filesystem rename. A second system transaction activates the Administrator and closes setup only after the directory is published.
- Crash recovery is state-driven and idempotent: an unexpired claim with valid published storage rolls forward to activation; a staged/incomplete directory is retried or quarantined; an active Administrator permanently closes setup. No second initial Administrator can be admitted because a durable claim remains authoritative until completed or explicitly recovered.
- The bootstrap token cannot sign in, access the admin interface, or be reused as an account credential.
- Operators remove the token after setup. NovelReader never persists or logs it.

Public registration cannot create an Administrator.

## Password and Argon2 controls

- Hash passwords with Argon2id and store algorithm/parameters in a PHC-style encoded hash.
- Fixed self-hosted baseline: 19 MiB memory, 2 iterations, parallelism 1, 16-byte random salt, and 32-byte output. This keeps password work modest while retaining memory-hard Argon2id protection.
- Passwords are 12–128 Unicode code points and are not silently trimmed or normalized.
- Usernames are trimmed, NFKC-normalized, and case-folded into `username_normalized`; display spelling is stored separately. Initial normalized usernames permit letters, numbers, `_`, `-`, and `.` and are 3–32 code points.
- Unknown user, wrong password, and disabled account return one generic invalid-credentials response. Unknown users perform bounded dummy verification.
- A process-wide semaphore allows at most two Argon2 operations across registration, login, dummy verification, password change, setup, and reset completion. Overload fails quickly with a retryable response rather than queuing unbounded work or exhausting memory.
- Authentication endpoints enforce small request-body limits, deadlines, and rate limits. Proxy-derived addresses are trusted only from explicitly configured proxy networks.
- Password change/reset revokes every Application Session for the account.

## Application sessions

The browser receives a 32-random-byte opaque base64url token. `system.db` stores only SHA-256 hashes.

- Cookie: `novelreader_session`, `HttpOnly`, `SameSite=Lax`, `Path=/`; `Secure` is required for HTTPS public deployments.
- Absolute expiry is 30 days from login. Rotation after 24 hours does not extend absolute expiry.
- Rotation stores a new current-token hash plus one previous-token hash with a fixed five-minute grace expiry. The grace token can authenticate the same session but cannot rotate again or create token families.
- Logout, disable, password change/reset, and deletion revoke both current and previous hashes immediately.
- Concurrent rotation, old-token replay, logout during grace, and disabled-account behavior require deterministic tests.
- Raw tokens and request cookies are never logged.

## Registration, bootstrap, and recovery configuration

- `REGISTRATION_ENABLED` — defaults false outside development.
- `REGISTRATION_INVITE_CODE` — optional deployment admission secret; never persisted or logged.
- `ADMIN_BOOTSTRAP_TOKEN` — temporary first-Administrator setup authority.
- `ADMIN_RECOVERY_TOKEN` — temporary disaster-recovery authority.
- `PUBLIC_URL` — canonical external origin for secure cookies and origin checks.
- `NOVELREADER_SECRET_KEY` — versioned 32-byte source-credential encryption key.

Secrets are compared in constant time. Setup and recovery routes are rate-limited, same-origin, and unavailable unless their corresponding environment secret is configured.

The recovery flow can create a replacement Administrator or reset an existing Administrator password. It cannot browse, decrypt, claim, export, rewrite, or delete Reader Data. Successful reset revokes that account’s sessions. Successful creation allocates a new empty reader directory. Recovery actions do not silently attach existing directories; attachment after `system.db` loss is a separate explicit recovery/import operation with directory inspection and confirmation.

## Administrator policy and password reset

Web administrator actions apply only to ordinary `reader` accounts initially. Administrators cannot disable, reset, or delete themselves or another Administrator through the web interface.

An Administrator-issued reader reset token:

- is random, short-lived (30 minutes), and single-use;
- is stored only as SHA-256 hash;
- is returned exactly once to the Administrator for delivery to the reader;
- lets the reader choose the replacement password;
- atomically consumes the token and revokes all sessions;
- invalidates earlier unused reset tokens for that reader.

Reset-token issuer identity is retained as non-secret text/audit metadata rather than a blocking foreign key. Deleting an issuer cannot prevent deletion or token cleanup.

## HTTP authorization and origin policy

Public routes are limited to health, setup policy/setup, registration policy/register, login, reset completion, configured recovery, and static authentication assets.

All source, search, Explore, book, chapter, bookmark, progress, cover/image, font, preference, export/import, logout, and account-self-service routes require identity. Account management routes require Administrator authority.

Before cookie authentication is enabled, replace permissive wildcard CORS behavior. Unsafe methods require a matching `Origin` from `PUBLIC_URL` or explicit development origins, and credentialed CORS is emitted only for those origins. This same-origin check plus `SameSite=Lax` is the initial CSRF defense.

## System schema

`system.db` contains tables conceptually equivalent to:

```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    username_normalized TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL CHECK (role IN ('reader', 'admin')),
    password_hash TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'disabled', 'deleting')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE auth_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    current_token_hash BLOB NOT NULL UNIQUE,
    previous_token_hash BLOB UNIQUE,
    previous_token_expires_at INTEGER,
    created_at INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    rotated_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    revoked_at INTEGER,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE password_reset_tokens (
    token_hash BLOB PRIMARY KEY,
    user_id TEXT NOT NULL,
    created_by_user_id TEXT,
    created_by_username TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    used_at INTEGER,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE account_deletions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL,
    last_error TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
```

Exact SQL is implementation work. The durable deletion row must survive removal of the user row and support retries.

## Account deletion

Permanent deletion is staged and retryable:

1. Require an Administrator, an ordinary target account, and exact target-username confirmation.
2. Commit target status `deleting`, revoke sessions, and create a durable deletion job.
3. Reject new authentication and Reader Data operations for that user. Serialize/drain in-flight reader-home operations through `readerstore` and purge in-memory Source Sessions.
4. Close that user’s database handles, resolve the directory from immutable `user_id` exclusively through `readerstore`, verify containment under the configured data root, remove it, then remove system-owned account records. Deletion jobs never persist or trust absolute host paths.
5. Mark the independent deletion job complete. On failure, retain error/status and retry; never report complete while files remain.

The implementation must define crash recovery at every transition and never reconstruct paths from a submitted username.

## Backup, restore, export, and import

### Cold deployment copy

The supported emergency backup is:

1. Stop NovelReader or its container.
2. Copy/archive the complete `data/` directory, preserving all files and names.
3. Restore by stopping NovelReader, replacing `data/`, and starting the same or a compatible version.

When the process crashed and cannot restart, operators copy everything as-is, including SQLite `-wal` and `-shm` files. They must not copy only a main `.db` file or delete WAL files manually.

Cold stopped-copy is the only initially supported **complete deployment backup**. Independent live SQLite backups cannot promise one coherent point in time across `system.db`, every reader/credential database, ordinary files, and setup/deletion jobs. A future online deployment-backup feature requires a deployment-wide quiescence or generation protocol and is not implied by per-reader export.

A complete backup contains password hashes and encrypted source credentials but no plaintext password, raw application/reset token, or plaintext source cookie. `NOVELREADER_SECRET_KEY` is backed up separately. Without it, all portable data and password hashes remain usable; only source login state is discarded/quarantined.

### Per-reader portable export

A documented ZIP contains:

```text
manifest.json
reader.db
files/
```

It excludes `credentials.db`, password hashes, sessions, reset tokens, roles, recovery/setup secrets, and authentication authority. The manifest records format/schema versions and informational provenance but cannot grant ownership.

`readerstore.Export` takes a per-reader-home write barrier, creates staged `reader.db` through SQLite's backup API (including committed WAL state), copies the corresponding file generation while writes remain blocked, validates database/file references and checksums, then releases the barrier and streams the completed bundle. Export never copies a live main database file directly. Concurrent-write, interrupted-export, and WAL-mode tests enforce this contract.

Import stages and validates the bundle before changing destination data. The currently authenticated destination account becomes owner. Conflict policy is explicit (for example merge, replace after backup, or reject); imported IDs never overwrite another reader’s directory or confer Administrator status. Caches/assets may be optional for smaller exports, while sources, original JSON, shelf, progress/history, bookmarks, preferences, and user files are always portable.

### Lost `system.db`

Each user directory remains inspectable and importable because `reader.db` is plaintext and self-contained. A newly initialized deployment creates a new Administrator through recovery/setup, then explicitly attaches or imports discovered reader directories after validation and operator confirmation. Directory discovery never automatically grants ownership based on an old username or manifest user ID.

## Cross-database credential consistency

A credential record uses a stable random credential-subject ID assigned to the source record, not only a reusable source URL. `reader.db` is authoritative: credential hydration first confirms that the exact subject ID is still attached to a live source owned by the opened reader home.

Source deletion first makes the source unavailable in `reader.db`, then idempotently tombstones/removes its credential record. A crash can leave an orphan ciphertext but cannot make it usable. Re-importing the same URL receives a new subject ID and never silently reattaches old credentials. Credential logout/update operations are idempotent, and `readerstore` runs bounded orphan reconciliation on open/migration. Tests cover crashes between source/credential writes, deletion, logout, and re-import.

## Schema gate and atomic cutover

The schema decision runs immediately after opening the data root and before current store initialization, migrations, workers, frontend serving, or network listener startup.

- Refuse old globally keyed non-empty data without modifying it.
- Direct internal developers to stop NovelReader, optionally retain a manual copy, delete or rename `DATA_DIR`, restart, and re-import test BookSources.
- Do not add a legacy reset flag, database converter, automatic backup verifier, rollback engine, or dual-layout mode.
- Never partially expose authenticated routes while any global store remains active.
- Old binaries are incompatible with the new layout. Current-layout disaster recovery uses a stopped complete-data-directory copy, not legacy migration.

## Legacy removal discipline

- Prefer reset and re-import of internal test data over schema converters that would survive the cutover.
- Replace old modules directly when all callers can be updated in the same slice; do not wrap global stores with user-aware pass-through adapters.
- Do not dual-write global and per-user databases.
- When a per-user store replaces a global store, delete the old initialization, schema/migrations, configuration, filesystem path handling, dead interfaces, and obsolete tests in the same completed slice.
- Retain tests that describe still-supported behavior by moving them to the new public seam; delete tests whose only purpose is legacy compatibility.
- At the Phase 2 release gate, search for and remove legacy database/table/path names and document any intentionally retained occurrence.
- No dormant fallback may reopen global Reader Data after the authenticated cutover. Startup either opens the current architecture or fails clearly.

## Implementation phases

1. **Storage foundation and cutover gate** — `readerstore`, data-root layout, manifests, reader schema versioning, clean development-reset guidance, cold-copy documentation, and fail-closed startup. No account routes are exposed yet.
2. **Authentication plus ownership conversion as one release boundary** — setup, users, roles, Argon2 controls, sessions, middleware, origin policy, and conversion of every source/book/cache/font/session interface to the authenticated reader home. Existing Reader Data routes open only when both halves are complete.
3. **Frontend account shell and administration** — setup/login/register, current account, logout/change-password, expiry recovery, ordinary-account management, environment recovery, and destructive confirmations.
4. **Portable export/import and system-loss recovery** — per-reader bundles, conflict policies, directory validation/attachment, and documented cold restore.
5. **LC-016 manual source login** — encrypted per-user cookie/login-header state, hydration, status, logout, key-loss behavior, and credential-store deletion.
6. **Later login capabilities** — interactive browser login or source-defined `loginUi` only after separate capability and credential-execution designs.

## Verification gates

- Complete-data-root cold-copy documentation and later restore integration coverage; cold copy is the only initial complete-deployment backup contract.
- Concurrent-write per-reader export tests proving SQLite WAL contents and the matching ordinary-file generation form one validated bundle.
- Concurrent and crash-interrupted first-Administrator setup tests proving one durable winner and idempotent roll-forward/quarantine.
- Source/credential crash, orphan, deletion, logout, and same-URL re-import tests proving stale credentials never hydrate.
- Schema-newer and old-layout refusal tests proving startup does not modify unsupported data.
- Password hashing/verification, Argon2 admission/overload, setup/bootstrap, registration invite, generic login failure, and recovery tests.
- Session creation, concurrent rotation/grace, expiry, replay, revocation, logout, password change/reset, and disabled/deleting-account tests.
- Origin/CORS tests for safe and unsafe methods.
- Cross-user isolation tests for every store and resource endpoint, including equal IDs/source URLs and filesystem traversal attempts.
- Administrator protection, ordinary-account reset/disable/delete, issuer deletion, deletion-job crash/retry, and in-flight write-quiescence tests.
- Export-content allowlist tests proving credentials/authentication authority are absent; import conflict and malformed/archive-traversal tests.
- Encryption-key-loss tests proving `reader.db` and ordinary files remain usable while source credentials are rejected.
- Restart/race tests for bounded per-user database handles and user-isolated Source Sessions.
- Targeted integration: setup → login → import source → add/read book → cold backup/restore → export/import → logout.
