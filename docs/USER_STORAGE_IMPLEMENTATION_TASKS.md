# User Storage and Authentication Implementation Tasks

This checklist tracks implementation of the accepted design in
[`AUTHENTICATION_DESIGN.md`](./AUTHENTICATION_DESIGN.md). The design document owns the security and
storage contracts; this file owns execution order and completion status.

## Working rules

- Keep each task to the smallest complete vertical change.
- Do not expose authentication while any Reader Data route still uses global storage.
- Do not hardcode `data/users/...` paths outside `readerstore`.
- Keep Reader Data and files unencrypted. Encrypt only reversible source credentials.
- A stopped-server copy of the complete `data/` directory must remain a valid backup.
- Add abstractions only when required by a task below or by a second real use case.
- Treat this as a clean pre-release replacement: no production-user compatibility layer is needed.
- Do not dual-write old and new storage or add pass-through adapters around global stores.
- Delete superseded schema, migrations, startup wiring, configuration, paths, modules, and tests in
  the same completed slice that replaces them.
- Temporary transition code must be explicitly marked and removed at the Phase 2 cutover.
- Update this checklist and `PLAN.md` when a phase changes state.

## Phase 1 — Reader storage foundation

**Goal:** establish the portable directory/database layout and fail-closed startup gate without
adding account routes or changing current public behavior.

### 1.1 Data-root classification

- [x] Define the supported data-root states: empty, current per-user layout, legacy non-empty,
      unsupported newer schema, interrupted staging, and invalid current layout.
- [x] Run classification before opening the current database and therefore before existing store
      initialization, workers, frontend serving, or the network listener.
- [x] Allow an empty deployment to initialize versioned root metadata and `users/` safely.
- [x] Refuse legacy non-empty data by default without modifying it. Detailed backup/reset
      instructions remain Phase 1.3.
- [x] Refuse unsupported newer schema versions without modifying files.
- [x] Require the current database path to remain inside `DATA_DIR` so cold-copy backup is complete.
- [x] Add focused tests for every state and prove refusal paths do not create the database or alter
      existing data.

**Complete when:** startup can safely decide whether the new storage layout may be opened before
any existing store touches the database.

### 1.2 Minimal `readerstore` module

- [ ] Add `backend/internal/readerstore/` with typed immutable `UserID` validation.
- [ ] Implement safe resolution beneath the configured `data/users/` root.
- [ ] Implement staged, idempotent creation of a reader directory.
- [ ] Create and validate `manifest.json` with a format version and immutable user ID.
- [ ] Initialize ordinary plaintext `reader.db` and its schema-version metadata.
- [ ] Initialize `credentials.db` as a separate store, without credential encryption behavior yet.
- [ ] Create the initial `files/fonts`, `files/covers`, and `files/chapter-assets` directories only
      where current features require them.
- [ ] Implement `Open` and explicit close/lifecycle behavior with a small bounded connection cache.
- [ ] Add tests for path traversal, invalid IDs, idempotent creation, two-user isolation, database
      readability with normal SQLite, and clean handle shutdown.

**Complete when:** callers can create and open isolated reader homes without knowing filesystem
paths or database setup details.

### 1.3 Cold backup and reset gate

- [ ] Document the stopped-server copy/restore procedure, including SQLite WAL/SHM files after a
      crash.
- [ ] Implement the explicit pre-release reset confirmation for legacy data.
- [ ] Create a timestamped, cold-compatible backup before resetting anything.
- [ ] Verify the backup can be enumerated and its SQLite databases opened before proceeding.
- [ ] Stage the new layout and publish it by same-filesystem rename where supported.
- [ ] Handle interrupted staging by retrying safely or quarantining it with an actionable error.
- [ ] Add stop-copy-restore, interrupted-reset, backup-failure, and rollback tests.

**Complete when:** a legacy reset cannot destroy the only copy of data and a stopped deployment can
be restored by replacing the complete `data/` directory.

### Phase 1 exclusions

Do not add login/register routes, setup UI, sessions, roles, credential encryption, portable ZIP
export/import, or migrate feature stores in this phase.

## Phase 2 — Atomic authentication and ownership cutover

**Goal:** add accounts and convert every global Reader Data/storage/session path in one release
boundary. Existing Reader Data routes remain closed until the entire phase is complete.

### 2.1 System identity storage

- [ ] Add versioned `system.db` schema for users, roles, session hashes, reset-token hashes, setup
      state, and durable deletion jobs.
- [ ] Add only `reader` and `admin` roles.
- [ ] Keep Reader Data and feature metadata out of `system.db`.
- [ ] Add account status transitions for active, disabled, and deleting.

### 2.2 Authentication core

- [ ] Implement username normalization and uniqueness rules.
- [ ] Implement Argon2id password hashing and verification with a process-wide admission limit.
- [ ] Apply request-size limits, timeouts, generic login errors, and bounded rate limits.
- [ ] Implement opaque server-side sessions with current-token hash, one previous-token hash, a
      fixed five-minute grace period, and a 30-day absolute expiry.
- [ ] Implement login, logout, password change, session revocation, and request-context identity.
- [ ] Replace wildcard credentialed CORS with the documented same-origin policy.
- [ ] Add tests for password parameters, overload, session rotation/replay, logout, expiry,
      revocation, and unsafe cross-origin requests.

### 2.3 First Administrator setup and recovery

- [ ] Implement one-time browser setup guarded by `ADMIN_BOOTSTRAP_TOKEN`.
- [ ] Serialize setup with a durable singleton claim and `BEGIN IMMEDIATE`.
- [ ] Stage and publish the first Administrator reader directory before activating the account.
- [ ] Recover deterministically from every interrupted setup state without creating two initial
      Administrators.
- [ ] Implement environment-token Administrator recovery without reading or modifying Reader Data.
- [ ] Ensure public registration can create only `reader` accounts.
- [ ] Add concurrent setup, crash recovery, token redaction, and recovery-session revocation tests.

### 2.4 Convert all Reader Data stores

- [ ] Move book sources and original JSON into the authenticated reader's `reader.db`.
- [ ] Move shelf books, chapters, progress, bookmarks, and chapter caches.
- [ ] Move font metadata and files into the reader home.
- [ ] Update covers and chapter-image endpoints to resolve only through the authenticated home.
- [ ] Key in-memory Source Sessions by immutable `UserID` and purge them on disable/deletion.
- [ ] Update Search and Explore to use only the authenticated reader's sources.
- [ ] Delete the old global storage initialization path; do not retain a disabled fallback.
- [ ] Delete superseded global table migrations, store constructors/interfaces, database/path
      configuration, and legacy-only tests as each replacement becomes complete.
- [ ] Search for old database/table/path names and either remove each occurrence or document why it
      is still current.
- [ ] Add cross-user isolation tests for every store and public resource endpoint, including equal
      resource IDs and source URLs.

### 2.5 Account administration and deletion backend

- [ ] Allow Administrators to list, disable/re-enable, reset, and delete ordinary accounts only.
- [ ] Implement one-time 30-minute reader password-reset tokens.
- [ ] Implement durable, retryable account deletion with write quiescence and Source Session purge.
- [ ] Resolve deletion paths only through immutable `UserID` and `readerstore`.
- [ ] Add tests for protected Administrator accounts, issuer deletion, reset replay/expiry,
      in-flight writes, filesystem failure, and deletion retry.

**Complete when:** every non-public route requires identity, no global Reader Data store or
fallback remains, obsolete schema/configuration/tests are removed, and cross-user access fails
without revealing whether another reader's resource exists.

## Phase 3 — Frontend account shell

**Goal:** expose the completed backend identity boundary through a minimal usable interface.

- [ ] Add first-run setup screen.
- [ ] Add login and environment-controlled registration screens.
- [ ] Load the current account before entering authenticated application routes.
- [ ] Add logout and password change.
- [ ] Handle session expiry without losing unsaved reader state.
- [ ] Add ordinary-account administration with explicit destructive confirmation.
- [ ] Add the configured Administrator recovery page.
- [ ] Verify keyboard use, error states, loading states, and narrow-screen behavior.
- [ ] Add targeted frontend tests and one setup → login → read → logout browser workflow.

**Complete when:** a new self-hoster can create the first Administrator in the browser and readers
can use the existing application only within their own storage.

## Phase 4 — Portable export, import, and recovery

**Goal:** make individual Reader Data movable without authentication or source credentials.

- [ ] Define and version the portable bundle manifest.
- [ ] Implement per-reader export using a write barrier, SQLite backup operation, staged files, and
      validation before streaming.
- [ ] Exclude `credentials.db`, password/session/reset hashes, roles, and recovery/setup secrets.
- [ ] Implement staged import into the currently authenticated account.
- [ ] Define the first conflict policies: reject and replace-after-backup. Add merge only when a
      concrete merge requirement is specified.
- [ ] Reject malformed archives, traversal paths, unsupported newer formats, and oversized input.
- [ ] Implement explicit discovered-directory attach/import recovery after `system.db` loss.
- [ ] Add export allowlist, concurrent-write, WAL, interrupted operation, conflict, and
      stop-copy-restore tests.
- [ ] Document cold deployment backup separately from portable per-reader export.

**Complete when:** one reader directory can be exported from one deployment and imported into a
new account on another deployment without carrying credentials or Administrator authority.

## Phase 5 — Durable source login (LC-016)

**Goal:** persist source login state per reader without compromising portable Reader Data.

- [ ] Add stable random credential-subject IDs to source records.
- [ ] Encrypt source cookies/login headers individually in `credentials.db` with AES-256-GCM and a
      versioned `NOVELREADER_SECRET_KEY`.
- [ ] Bind ciphertext to user ID, source subject, record kind, and format version.
- [ ] Hydrate credentials only after confirming the subject still exists in `reader.db`.
- [ ] Implement manual credential import, status, logout, deletion, and bounded orphan cleanup.
- [ ] Ensure same-URL re-import receives a new subject and cannot revive old credentials.
- [ ] Treat key loss/decryption failure as source reauthentication, never Reader Data loss.
- [ ] Add crash, orphan, logout, deletion, re-import, key-loss, and export-exclusion tests.

**Complete when:** source login survives restart for one reader, cannot cross readers, and can be
lost or deleted without affecting sources, books, reading state, or files.

## Phase 2 legacy-removal gate

Before Phase 2 is marked complete:

- [ ] Remove the transitional `novelreader.db` startup path and `DATABASE_PATH` compatibility if it
      is no longer part of the final layout.
- [ ] Remove global source/book/font store initialization and all global Reader Data tables.
- [ ] Remove old font/data paths outside reader directories.
- [ ] Remove compatibility adapters and temporary feature flags introduced only for the cutover.
- [ ] Remove or rewrite tests that instantiate global stores directly.
- [ ] Run a repository search for `novelreader.db`, old global table names, and old global paths;
      record any intentional remaining references.
- [ ] Verify an old global data root fails clearly and a fresh current-layout deployment has no
      code path capable of opening it as Reader Data.

## Deferred until requested

- Interactive browser/WebView login handoff.
- Execution of source-defined `loginUi` credential forms.
- Generic RBAC or custom Administrator permissions.
- Online complete-deployment backup across all databases and files.
- Portable source-credential export.
- Automatic merge of complex reader exports.

## Current checkpoint

- [x] Architecture and security decisions accepted.
- [x] Domain glossary and ADR recorded.
- [x] Independent architecture review completed without blockers.
- [x] Phase 1.1 data-root classifier and startup gate complete.
- [ ] Phase 1.2 minimal `readerstore` module started.
