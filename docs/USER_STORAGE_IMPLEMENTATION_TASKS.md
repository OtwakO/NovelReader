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
- [x] Refuse legacy non-empty data without modifying it and direct internal developers to a clean
      reset rather than a compatibility migration.
- [x] Refuse unsupported newer schema versions without modifying files.
- [x] Require the current database path to remain inside `DATA_DIR` so cold-copy backup is complete.
- [x] Add focused tests for every state and prove refusal paths do not create the database or alter
      existing data.

**Complete when:** startup can safely decide whether the new storage layout may be opened before
any existing store touches the database.

### 1.2 Minimal `readerstore` module

- [x] Add `backend/internal/readerstore/` with canonical lowercase UUID v4 `UserID` validation.
- [x] Implement safe resolution beneath the configured `data/users/` root.
- [x] Implement staged, idempotent creation of a reader directory.
- [x] Create and validate `manifest.json` with a format version and immutable user ID.
- [x] Initialize ordinary plaintext `reader.db` and its schema-version metadata.
- [x] Initialize `credentials.db` as a separate store, without credential encryption behavior yet.
- [x] Create the initial `files/fonts`, `files/covers`, and `files/chapter-assets` directories only
      where current features require them.
- [x] Implement `Open` and explicit close/lifecycle behavior with a small bounded connection cache.
- [x] Add tests for path traversal, invalid IDs, idempotent creation, two-user isolation, database
      readability with normal SQLite, cache pressure/wake-up, and clean handle shutdown.

**Complete when:** callers can create and open isolated reader homes without knowing filesystem
paths or database setup details.

### 1.3 Development reset and cold-copy contract

The project has no production users or irreplaceable legacy Reader Data. Automated legacy reset,
migration, backup verification, and rollback machinery were removed from scope to avoid permanent
pre-release compatibility debt.

- [x] Document the clean development reset: stop the process, optionally copy old state for manual
      inspection, delete/rename `DATA_DIR`, restart, and re-import test BookSources.
- [x] Keep old non-empty layouts fail-closed and provide an actionable startup error without a reset
      flag or automatic mutation.
- [x] Document that ignored `test_booksource*.json` corpora and tracked `testdata/booksource/`
      fixtures are import inputs/evidence rather than user data and do not need database migration.
- [x] Document complete stopped-server copy/restore, including preserving SQLite WAL/SHM files after
      a crash.
- [x] Retain tests proving a refused old root is not modified and no database is created.

**Complete when:** internal developers can start fresh without migration code, and operators know
how to copy/restore a complete current-layout `data/` directory while NovelReader is stopped.

### Phase 1 exclusions

Do not add login/register routes, setup UI, sessions, roles, credential encryption, portable ZIP
export/import, or migrate feature stores in this phase.

## Phase 2 — Atomic authentication and ownership cutover

**Goal:** add accounts and convert every global Reader Data/storage/session path in one release
boundary. Existing Reader Data routes remain closed until the entire phase is complete.

### 2.1 System identity storage

- [x] Add versioned `system.db` schema for users, roles, session hashes, reset-token hashes, setup
      state, and durable deletion jobs.
- [x] Add only `reader` and `admin` roles.
- [x] Keep Reader Data and feature metadata out of `system.db`.
- [x] Add account status transitions for active, disabled, and deleting.

### 2.2 Authentication core

- [x] Implement username normalization and validation rules.
- [x] Enforce normalized username uniqueness in account storage.
- [x] Implement Argon2id password hashing and verification with a process-wide admission limit.
- [x] Implement request-size limits, bounded response deadlines, generic login errors, and bounded direct-peer rate limits in the fail-closed HTTP auth module.
- [x] Implement persistent opaque server-side sessions with one hash-only token per login and no automatic expiry or rotation.
- [x] Implement login, logout, `RequireIdentity`, `RequireAdmin`, request-context identity, and browser-session token lifecycle behind an unmounted fail-closed HTTP module.
- [x] Implement storage-level credential lookup, generic credential failure, and privileged password replacement with transactional session revocation.
- [x] Replace wildcard credentialed CORS with host-matched browser-origin policy and an optional exact `PUBLIC_URL` override for host-rewriting proxies.
- [x] Add module-level tests for HTTP login/cookie behavior, independent devices, logout idempotency, identity/admin middleware, strict request limits, bounded deadlines/rate limits, and unsafe cross-origin requests.
- [x] Mount authentication routes in the atomic ownership cutover; every non-public Reader Data route authenticates before selecting per-account stores.
- [x] Add storage-level tests for password parameters, overload, independent device sessions, logout-this-device, logout-all-devices, and concurrent revocation.

### 2.3 First Administrator setup and recovery

- [x] Mount one-time browser setup guarded by `ADMIN_BOOTSTRAP_TOKEN`; successful setup creates the first Administrator home and authenticated browser session.
- [x] Serialize setup/recovery with durable singleton claims, immediate SQLite transactions, shared in-process guards, and a server-lifetime OS lock that permits only one NovelReader process per writable data root.
- [x] Stage and publish the first Administrator reader directory before activating the account.
- [x] Recover deterministically from interrupted claim and directory-publication states without creating two initial Administrators; activation and setup closure share one transaction.
- [x] Implement environment-token Administrator recovery without reading or modifying existing Reader Data; reset reactivates an Administrator and revokes sessions, while replacement creation allocates a new empty home through a generation-bound durable claim.
- [x] Ensure public registration can create only `reader` accounts and fails closed until initial setup closes.
- [x] Add environment-token recovery and session-revocation tests covering active/disabled/deleting targets, stale credential/session races, durable claim generations, interrupted replacement-home provisioning, pre-existing-home rejection, setup-claim supersession, strict/rate-limited HTTP requests, bounded deadlines, retry-safe auto-login, and late-session cleanup.

### 2.4 Convert all Reader Data stores

- [x] Move book sources and original JSON into the authenticated reader's `reader.db`.
- [x] Move shelf books, chapters, progress, bookmarks, and chapter caches.
- [x] Move font metadata and files into the reader home through its path-safe `FileStore`.
- [x] Update covers and chapter-image endpoints to resolve only through the authenticated home.
- [x] Isolate Source Sessions, Explore sessions, source throttling, analyzer cache, and JavaScript compatibility state inside bounded per-reader runtimes while sharing only process admission capacity.
- [x] Update Search and Explore to use only the authenticated reader's sources.
- [x] Delete the old global storage initialization path; no production fallback remains.
- [x] Delete `DATABASE_PATH` configuration and global font/data path construction.
- [x] Search production startup for old database/path names; remaining `novelreader.db` references are reset guards or isolated legacy test seams, not production readers.
- [x] Add authenticated equal-ID cross-user isolation regressions for Reader Data and fonts; all resource endpoints share the same authenticated-home boundary.

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

- [x] Add first-run setup screen.
- [x] Add environment-controlled ordinary-reader registration with an optional deployment invite code, durable retry-safe reader-home provisioning, automatic login, and a minimal frontend form.
- [x] Load the current account before entering authenticated application routes.
- [ ] Add password change; logout is complete.
- [x] Handle mid-session revocation by centrally unmounting private UI on `401`; explicit logout also unmounts immediately and warns/retries instead of claiming success if server revocation fails.
- [ ] Add ordinary-account administration with explicit destructive confirmation.
- [x] Add the configured Administrator recovery page.
- [x] Add keyboard-accessible forms, loading/error states, and narrow-screen layouts for setup/login/recovery.
- [ ] Add one setup → login → read → logout browser workflow; targeted compiler/state tests are complete.

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

- [x] Remove the transitional `novelreader.db` startup path and `DATABASE_PATH` compatibility.
- [x] Remove production global source/book/font store initialization and global Reader Data tables.
- [x] Remove old font/data paths outside reader directories.
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
- [x] Phase 1.2 minimal `readerstore` module complete.
- [x] Atomic authentication/Reader Data ownership cutover complete.
- [x] Environment-controlled ordinary-reader registration complete.
- [ ] Next core account slice: authenticated password change.
