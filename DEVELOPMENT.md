# Development Notes

### [2026-08-11] Pre-public development data is reset, not migrated
- **Context**: A bookshelf identity change led to append-only upgrade logic, backfills, migration-order wiring, compatibility regressions, and partial preservation of local reader data even though NovelReader is rapidly iterating and has no public or irreplaceable user data.
- **Change**: Pre-public local accounts, reader homes, databases, and user data are explicitly disposable test state. Data-model changes update the canonical fresh schema directly; incompatible development databases are deleted and recreated. Do not add upgrade migrations, compatibility adapters, backfills, backup/rollback workflows, or migration-only tests unless the user explicitly approves an exception for a specific core architectural boundary.
- **Reason**: Historical schema support created during rapid iteration quickly becomes redundant production code and test debt while the model is still changing. Clean initialization is the relevant contract at this stage.
- **Verified**: This policy is now recorded in `PLAN.md` under **Pre-public development schema policy**. The append-only logical-identity upgrade, backfill, all persistence `ALTER TABLE` helpers, the reader migration registry/history, partial-version opening, and old-schema upgrade tests were removed. Fresh reader feature schemas and the single version epoch now commit in one transaction; each feature owns one authoritative current-DDL function. Published homes pass generic read-only epoch-and-complete-catalog validation—built by running those same DDL functions in isolated in-memory SQLite—before writable open, feature stores expose no post-open schema initializer, and mismatches are never repaired or modified. The disposable `backend/data` root was deleted so the next local run initializes only the current schema.
- **Watch out**: Existing schema-initialization code should represent only the current clean schema. Public release, real user adoption, or explicitly irreplaceable data is the gate for reconsidering migrations.

### [2026-08-11] Bookshelf identity is normalized title plus author
- **Context**: Search-result UUIDs allowed the same title/author to appear as multiple shelf books, splitting sources and reading state. Legado's official source confirms one logical shelf book per name+author and a separate change-source search.
- **Change**: The canonical fresh book schema uses a unique normalized title+author identity. User-facing add/enrich atomically inserts or merges into the existing shelf ID, preserving current source, progress, chapters, cache, and bookmarks while merging unique source bindings. Book details show the human-readable current source title and provide batched **Find more sources / Search 50 more** discovery filtered to exact normalized title+author; discovery never switches automatically.
- **Reason**: A book is the shelf entity; source URL/book URL pairs are replaceable bindings beneath it, not separate books.
- **Verified**: Concurrent store regressions converge on one row; API regressions cover normalized add/merge, source-list merge, and source title persistence. With the real 907-source catalog, adding `异度旅社` from 大唐 and 快眼 returned the same shelf ID, preserved `大唐小说（优+）` as current, and added 快眼 as an alternate. The detail UI found four more exact matches in the first 50-source batch while leaving current source unchanged.
- **Watch out**: Do not preserve pre-correction databases with migration logic. Delete the disposable development data and verify this model against a freshly initialized database.


### [2026-08-11] Shelved books keep source switching available while reading
- **Context**: Source recovery existed during shelf addition and on book details, but an active reader had to leave the chapter to change source.
- **Change**: Book details and the reader now share one source switcher. The reader saves pending progress, uses the existing atomic switch endpoint, follows backend title-first/index-fallback mapping, updates the chapter URL when the mapped index changes, and reloads content from the switched source. Failed choices preserve the current source when backend validation fails and keep the selector available for another choice.
- **Reason**: Source availability changes over time; source choice is a continuing property of a shelved book, not only an import-time decision.
- **Verified**: Frontend compile/regression checks cover both switch surfaces and progress-before-switch ordering; all 37 frontend tests and the production build pass. Existing backend source-switch and chapter-mapping tests pass, and the UI detector reports no findings.


### [2026-08-11] Shelf addition validates readability before navigation
- **Context**: Adding `异度旅社` appeared to fail with “Explore request failed,” even though `/api/books/enrich` succeeded.
- **Change**: Generic API failures can no longer become `ExploreApiError`; only Explore endpoints opt into that diagnostic type. Search and Explore now share a post-add readability gate that loads the TOC and first readable chapter. A failed primary stays on the shelf and exposes an inline alternate-source chooser; each selected alternate is switched and validated before navigation.
- **Reason**: Search metadata success does not prove that a source's TOC/content rules still work, and concurrent source completion cannot rank readability reliably. The old generic coded-error check also mislabeled normal TOC failures as Explore failures.
- **Verified**: Against a stopped-copy of real reader data, search found 65 exact `异度旅社` matches. The selected primary returned a real TOC 502; choosing `大唐小说（优+）` returned 100 chapters and readable first/middle/last content. A live browser run confirmed enrich 200 → primary TOC 502 → source switch 200 → TOC/content 200 → rendered chapter text. All 36 frontend tests and the production build pass.

### [2026-08-11] Static fallback must remain path-only beside the API prefix
- **Context**: The first local startup after the authenticated ownership cutover panicked in Go 1.26 while registering `GET /` beside the method-agnostic `/api/` boundary.
- **Change**: The SPA fallback now registers the path-only `/` pattern, explicitly permits only GET/HEAD, and leaves `/api/` as the more-specific route. The Windows launcher generates a temporary setup token on every run so a startup failure after `system.db` creation cannot strand unfinished first setup.
- **Reason**: Go's ServeMux rejects patterns where one is more specific by method while the other is more specific by path. Setup completion—not database-file existence—is the authority for whether the bootstrap token is still needed.
- **Verified**: A regression reproduces the exact pattern conflict and proves API precedence, SPA fallback, and static-method rejection. API/server/auth tests pass, and a fresh temporary-data process serves `/api/healthz` and `/` without panic.

### [2026-08-07] Reader deletion is durable roll-forward, never rollback
- **Context**: Added permanent Administrator deletion of ordinary readers and their isolated Reader Data homes.
- **Change**: Exact username confirmation atomically marks the reader `deleting`, increments authentication version, revokes sessions, and creates an independent job. The API runtime manager then rejects new acquisitions, drains in-flight references, and discards the forked Search/Explore/Source Session state. `readerstore` independently blocks open/create, drains home leases, closes databases, eagerly freezes and verifies the prepared data-root identity before retaining an `os.Root`, rejects later configured-root replacement for path-based operations, opens `users/` only relative to the retained handle, and atomically renames the immutable-ID home to a root-relative quarantine name before validating/removing it. Account removal and job completion share the final system transaction.
- **Reason**: Filesystem deletion cannot be rolled back safely. A durable non-authenticating identity plus independent job makes every crash/failure state retryable without ever claiming restored or complete data incorrectly.
- **Verified**: Regressions cover exact confirmation, protected Administrators, job-creation rollback, atomic session revocation/status transition, in-flight runtime and home draining, racing acquisition rejection, per-reader runtime purge, contained-path/symlink protection, deterministic filesystem failure and retry, concurrent retry convergence, account/job finalization, strict UUID/body/origin/size HTTP boundaries, and frontend retry/confirmation states.
- **Watch out**: Persisted deletion errors are generic and never contain host paths. An ambiguous client failure keeps its previous UI status and requires refresh before retry, preventing typed confirmation from being bypassed by a client-side guess.

### [2026-08-07] Reader reset separates password work from token consumption
- **Context**: Added Administrator-issued one-time password resets for ordinary reader accounts.
- **Change**: Issuance stores only a SHA-256 token hash, retains issuer username metadata, expires after 30 minutes, and deletes earlier unused tokens. Completion performs bounded Argon2 work before taking the session guard, then re-checks and consumes the token in the same transaction as password replacement, authentication-version increment, and all-session revocation. Active/disabled status is preserved.
- **Reason**: Holding the session barrier during password hashing would stall unrelated authentication, while trusting the pre-hash token lookup would permit concurrent replay. The second transactional lookup provides single-use behavior without a long authentication lock.
- **Verified**: Regressions cover hash-only storage, non-cacheable plaintext issuance, supersession, exact expiry including expiry during Argon2 work, replay, concurrent completion, protected Administrators, disabled-status preservation, issuer deletion metadata, session revocation, rollback on revocation failure, strict public parsing/origin/size boundaries, Administrator authorization, and public reset routing that clears private account state before any session lookup.
- **Watch out**: A deadline after reset dispatch can be ambiguous because successful completion necessarily consumes the token. The HTTP response tells the reader to try signing in with the new password instead of blindly retrying.

### [2026-08-07] Reader disable is a transactional authentication boundary
- **Context**: Added the first Administrator account-management slice: ordinary-reader list and active/disabled controls.
- **Change**: The AccountService permits only ordinary reader active↔disabled changes. Role validation, status/version update, and all-session revocation occur under the shared session guard and one transaction; desired-state retries are idempotent. The Account page confirms disable and never exposes Administrator targets.
- **Reason**: A role preflight followed by the existing generic lifecycle transition would create a future check/use gap, while treating a timed-out retry as a conflict would make a successful disable look failed.
- **Verified**: Service and HTTP regressions cover filtered listing, protected Administrators, atomic revocation and rollback, preserved credentials/data semantics, re-enable, a committed-timeout idempotent retry, strict UUID/duplicate/missing/wrong-type/trailing/oversized body parsing, origin checks, and Administrator authorization. Frontend behavioral tests cover role visibility and active/disabled/deleting actions; mutations are serialized, while compiler tests and a warning-free production build cover the Account-page controls and confirmation.
- **Watch out**: The lower-level `TransitionAccountStatus` remains strict because registration and future deletion use lifecycle transitions; only the Administrator desired-state operation is idempotent.

### [2026-08-07] Self-service password change rejects stale credentials
- **Context**: Added authenticated password change after controlled reader registration.
- **Change**: The current password is verified, then the replacement commits only if the stored hash and authentication version still match the verified values; every session is deleted in the same transaction. The browser clears its cookie, unmounts private UI, and requires login with the new password.
- **Reason**: A simple verify-then-replace sequence could overwrite a concurrent Administrator recovery or another password change. Optimistic matching preserves the existing small account module without adding workflow state.
- **Verified**: HTTP regressions cover current/other-session revocation, wrong-current rejection, authentication, strict/oversized bodies, ambiguous timeout fail-closed behavior, and old/new credential behavior; a deterministic concurrency regression proves stale verified credentials cannot overwrite an authoritative replacement. Frontend behavioral tests cover forced re-login and clearing every credential field, while compiler tests and the production build cover the Account page.
- **Watch out**: Wrong current passwords return `403`, not `401`; only a lost application session may trigger the global frontend authentication-loss signal.

### [2026-08-07] Registration activates only after reader-home publication
- **Context**: Added optional public ordinary-reader registration after the authenticated ownership cutover.
- **Change**: Registration reserves a recognizable disabled account, creates the immutable-ID reader home, then activates the account and creates a browser session. A retry with the same normalized username and password resumes the same reservation after interruption; normal Administrator-disabled accounts are not resumable.
- **Reason**: No credential may authenticate before its isolated Reader Data home exists; the flow stays small by reusing `readerstore.Manager` rather than introducing a general workflow framework.
- **Verified**: HTTP regressions cover disabled/invite policy, malformed and duplicate requests, provisioning failure retry, home publication, and authenticated session identity; frontend compiler/gate tests and production build cover the registration form.
- **Watch out**: Session issuance is not part of provisioning authority. If a response/session fails after activation, the account and home remain valid and the reader signs in normally; registration does not become an alternate login endpoint.

### [2026-08-10] Authentication and Reader Data ownership mounted atomically
- **Context**: Login/setup/recovery could not be exposed while production routes still used one global feature database and process-global workflow state.
- **Change**: Production now mounts public auth/setup/configured-recovery beside an authenticated Reader Data server. Identity selects a bounded per-reader runtime backed by `users/<UserID>/reader.db` and `files/`; reader migrations are centralized and ordered; Search/Explore/source sessions/analyzer cache/JavaScript compatibility state are reader-local while process admission remains shared. `novelreader.db` startup, `DATABASE_PATH`, and global font paths were removed. The frontend resolves setup/account before mounting private routes.
- **Reason**: Resource IDs are locators, not authorization. Selecting the authenticated home before any handler/store lookup makes equal IDs across accounts safe without scattered ownership checks or a legacy fallback.
- **Verified**: Full backend tests, targeted equal-ID/anonymous/font isolation regressions, frontend tests/build, vet/race/cross-platform checks recorded with the commit. Production search confirms no global feature database constructor or `DATABASE_PATH` use remains. Docker Compose and shell configuration validate; the Docker E2E build could not start in the sandbox because Docker Buildx could not write its read-only activity directory.
- **Watch out**: `api.NewServer` and `internal/database` remain isolated test seams for feature tests; production must use `api.NewAuthenticatedServer` and `openStores`. Account disable/delete must later purge the reader runtime before removing a home.

### [2026-08-11] Search → Book Info v2 found two shared workflow gaps
- **Context**: Ran another 25-source deterministic Search → Book Info audit, disjoint from v1, with exact frozen raw definitions and query `凡人修仙传`.
- **Change**: Preserved JSON/Markdown evidence and versioned scripts under the Search/Book Info audit directories. No production source or parser behavior changed.
- **Reason**: Raw 49 proved URL-valued list fields must keep the first extracted value rather than join multiple hrefs; raw 179 proved empty Search lists need Legado's detail-page fallback on the final response. Raw 396 is explicitly WebView-dependent and remains deferred.
- **Verified**: Eight sources completed both stages. Every non-pass replayed sequentially; direct HTTP/DNS, one targeted Chromium check, reduced production probes, and upstream Legado source established two shared gaps and classified all other outcomes. The verifier recomputes ranking, disjointness, exact imported definitions, corpus/import hashes, replay coverage, and summary totals.
- **Watch out**: Keep the two fixes separate and approval-gated. Do not patch raw sources, and do not implement `java.webView` as a fake regular-JS method.

### [2026-08-11] Explore v13 found no shared compatibility gap
- **Context**: Sampled another 50 unrestricted, deterministic, disjoint Explore identities after excluding all 550 prior samples.
- **Change**: Preserved v13 JSON/Markdown evidence and versioned scripts under the Explore-owned audit directories. No production source/parser code changed.
- **Reason**: Forty-two sources returned 1,211 distinct books; all eight non-passes were explained by incomplete/invalid imported contracts, broken DNS, upstream 521/522, or Cloudflare blocking.
- **Verified**: Every non-pass and diagnostic was replayed sequentially. Direct HTTP/DNS and targeted Playwright checks confirmed the classifications; the verifier proves corpus hash, ranking, identity disjointness, counts, and evidence completeness.
- **Watch out**: Authenticated audit POSTs must send both the session cookie and canonical `Origin`; omitting Origin yields pre-source HTTP 403 and is an invalid audit capture.

### [2026-08-11] BookSource test data gained operation ownership
- **Context**: Core fixtures, pinned Explore fixtures, Explore audit history, and Search → Book Info evidence had accumulated in one flat directory.
- **Change**: Moved deterministic inputs to `testdata/booksource/conformance/{core,explore}/` and dated observations to `testdata/booksource/audits/{explore,search-bookinfo}/`. Updated all executable and documentary paths and added a shared BookSource audit-workflow skill.
- **Reason**: Purpose-first and operation-second ownership makes future placement obvious without adding a registry or audit framework.
- **Verified**: Conformance tests and audit verifiers pass from the new paths; searches find no stale root-level operation references.
- **Watch out**: New operations create their own conformance/audit subdirectory. Live evidence never becomes a deterministic fixture dependency.

### [2026-08-11] Lenient Legado URL options required preservation before parsing
- **Context**: The Search → Book Info audit proved that a live source returned detail links ending in a valid lenient option object, but NovelReader requested a percent-encoded option suffix and received 404.
- **Change**: Search-result URL resolution now preserves balanced trailing option objects verbatim. The shared URL builder normalizes single-quoted strings and bare object keys before its existing typed option decoder. It does not add general JSON5 or arbitrary expression support.
- **Reason**: Fixing only the parser was insufficient because `net/url` had already encoded the suffix during search-result resolution. Both changes belong to one shared URL-contract boundary; neither is source-specific.
- **Verified**: Red-green regressions cover raw-267-style `{Cookie:"..."}` detail links and raw-151-style `{method:"post",body:"..."}` requests. A fresh raw-267 production search preserved the suffix and Book Info succeeded with live detail data. Playwright was unnecessary because ordinary HTTP returned the complete page.
- **Watch out**: Whole-URL `<js>` remains separate and unimplemented; its sampled source required authentication, so it was not bundled without working-source evidence.

### [2026-08-11] Search and Book Info audit separated engine gaps from dead sources
- **Context**: Many imported sources appeared to fail during search or Book Info, but stale sites, WAFs, authenticated APIs, and parser incompatibilities needed different treatment.
- **Change**: Ran a deterministic 25-identity sample from `test_booksource3.json` with one fixed query, production search, detail-only follow-up, and sequential replay of every non-pass. Preserved complete evidence under `testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v1-2026-08-11.{json,md}`.
- **Reason**: One live working source proved a shared lenient URL-option parsing gap; the other failures were not valid reasons for source-specific fixes. A whole-URL `<js>` difference remains only an observation because its sampled source requires login and denied anonymous access.
- **Verified**: 10 credible Search → Book Info passes, 1 confirmed detail engine gap, and 14 upstream/blocked/stale/drift/empty outcomes. The verifier checks all 25 identities, all 15 sequential replays, corpus SHA-256, frozen indices, and summary counts.
- **Watch out**: Fix only the shared lenient option parser after approval. Do not use raw 151 as working-source proof while its correctly formed upstream POST returns 500, and do not add whole-URL `<js>` support from raw 50 alone.

### [2026-08-10] Recovery required one-process data-root ownership
- **Context**: Administrator recovery must revoke sessions atomically and create a replacement Administrator only with a genuinely new empty reader home. Process-local guards cannot prove either property when two server processes share one writable data root.
- **Change**: Server startup now acquires an OS-level exclusive lock for `DATA_DIR` and holds it through `auth.Store` lifetime. Recovery uses a generation-bound durable claim, rejects homes that predate provisioning authorization, resumes its own interrupted home creation, and supersedes an open or claimed initial-setup authority when replacement creation commits.
- **Reason**: The single-process-per-root constraint already existed operationally; enforcing it is simpler and safer than adding distributed filesystem/SQLite coordination to a self-hosted application that does not support active-active deployment.
- **Verified**: Unix and Windows lock implementations cross-compile; a second server open fails until the first store closes; repeated recovery tests cover stale claim generations, pre-existing-home rejection, interrupted provisioning, setup-claim supersession, password/session revocation, and retry-safe auto-login.
- **Watch out**: Do not bypass `openStores` in another production entry point. Direct `OpenSystemStore` calls are test/library seams and do not acquire the process lock themselves.

### [2026-08-07] Setup HTTP and UI remain unmounted until ownership cutover
- **Context**: The one-time first-Administrator flow now has a strict HTTP handler and a Svelte setup form, but Reader Data routes still use global stores.
- **Decision**: Keep `/api/setup`, authentication routes, and the setup screen unmounted until the same atomic change selects Reader Data by authenticated account identity.
- **Reason**: Mounting setup/authentication before protecting all non-public Reader Data would create authentication theater. The dormant boundary is compiled and fully tested so the cutover can be wiring rather than new security logic.
- **Verified**: Setup tests cover strict request parsing/origin policy, token admission, claim recovery, closed-state rate limiting, response deadlines, immutable-initial-admin retry, persistent cookie creation, and bounded late-session revocation. A targeted Svelte compiler test protects the unmounted setup component.
- **Watch out**: Successful setup auto-login is retry-safe only with the valid bootstrap token plus the immutable initial Administrator credentials; remove `ADMIN_BOOTSTRAP_TOKEN` after setup is complete.

### [2026-08-06] Renamed the active branch for its actual scope
- **Context**: The branch began as `audit/explore-live-compat`, then the compatibility queue was deliberately paused while the accepted reader-storage and authentication prerequisite was implemented on the same unpushed branch.
- **Change**: Renamed the branch in place to `feat/user-storage-auth` rather than merging an intentionally incomplete authentication cutover into `main` and branching again.
- **Reason**: Renaming preserves the coherent commit sequence and hashes while making the branch and future PR accurately describe the work. Authentication/setup routes remain intentionally unmounted until the ownership cutover.
- **Watch out**: Before opening the PR, verify whether the inherited Explore-audit organization commit belongs in this branch or should be integrated separately.

### [2026-08-06] Writable data roots have one server owner
- **Context**: First-Administrator setup spans a durable SQLite claim and atomic publication of a reader-home directory.
- **Decision**: One NovelReader server process exclusively owns a writable `DATA_DIR`. In-process guards coordinate multiple Store/Manager instances, but shared writable multi-process access is unsupported.
- **Reason**: SQLite transactions serialize database records, not adjacent staged filesystem publication. A setup-only file lock would imply safety that the rest of the reader-storage workflows do not provide.
- **Verified**: Setup concurrency tests cover separate Store and Manager instances in one process; README and PLAN document the deployment constraint.
- **Watch out**: If multi-process deployment is ever required, add a root-wide startup ownership lock and subprocess tests rather than local workflow locks.

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

### [2026-08-06] Browser sessions are persistent and independently revocable
- **Context**: The earlier design used 30-day rotating sessions, but periodic re-login was judged disruptive for a multi-device self-hosted reader.
- **Change**: Schema version 2 stores one constrained 32-byte SHA-256 token hash per persistent login session. Tokens do not expire or rotate. Logout deletes one row; logout-all, password replacement, and active-to-disabled/deleting transitions delete every account session. Authentication updates activity at most hourly.
- **Reason**: Independent persistent sessions keep each browser/device revocable without refresh tokens, grace windows, rotation races, or forced login prompts. A canonical-path process-wide read/write barrier guarantees revocation completion orders against authentication without turning every request into a SQLite write transaction.
- **Verified**: Tests cover random token shape, hash-only persistence and schema constraints, independent devices, malformed/unknown tokens, stored identity corruption, coarse monotonic activity, idempotent logout, logout-all, concurrent authentication/logout, password/status atomic revocation, multiple Store instances, symlinked-parent aliases, close/reopen, schema-v1 byte-preserving refusal, repeated/race/vet checks, and independent review.
- **Watch out**: The guard map intentionally retains one tiny mutex per opened canonical `system.db` path for process lifetime; do not add refcount removal that can split the revocation barrier during close/reopen. HTTP cookies must remain `HttpOnly` and must never log raw tokens.

### [2026-08-06] Authentication HTTP remains unmounted until ownership cutover
- **Context**: Login/logout and identity middleware were ready before global Reader Data routes had been converted to per-account stores.
- **Change**: Added a tested fail-closed HTTP auth module with strict 4 KiB duplicate-rejecting JSON, bounded login response deadlines, bounded direct-peer rate limiting, persistent cookie lifecycle, `RequireIdentity`, and `RequireAdmin`. Replaced wildcard CORS in the live server with normalized browser-origin/request-host matching and an optional exact `PUBLIC_URL` override. The module does not mount `/api/auth/*` in production.
- **Reason**: Mounting authentication while global source/book routes remained unauthenticated would create authentication theater and violate the accepted atomic ownership-cutover boundary. Argon may finish after a timed-out response while retaining its admission slot; a writer-preferring context-aware session guard preserves revocation progress, and any session committed as cancellation wins is retried for token-free cleanup within a bounded background window.
- **Verified**: Public-handler tests cover login/account/logout, independent devices, logout idempotency, reader/admin middleware, secure and development cookies, missing/foreign origins, strict fields and duplicate keys, oversized and blocked bodies, direct-peer limits despite spoofed forwarded addresses, Argon deadline response, revocation-barrier cancellation, and commit-at-deadline orphan cleanup. Server tests cover configured and development origin policy; full backend, targeted race/repetition/vet, Windows compilation, shell syntax, and rendered randomized-port Compose configuration pass. The Docker image E2E could not build in the sandbox because `/home/huang/.docker/buildx/activity` is read-only.
- **Watch out**: Do not mount the auth handler as a standalone follow-up. Mount it only in the same working change that gates all non-public routes and selects Reader Data by `IdentityFromContext` account ID. Dynamic host matching is for trusted localhost/LAN/tailnet access and is not a DNS-rebinding defense; set `PUBLIC_URL` when exposed to untrusted browser networks.

### [2026-08-11] Explore v14 exposed result filtering and exact-import audit risk
- **Context**: The fourteenth deterministic Explore sample froze 50 raw-index identities after excluding 600 prior identities.
- **Change**: Preserved the exact frozen source definitions and updated the audit skills to import only those definitions when runtime storage is keyed by `bookSourceUrl`. Recorded a shared gap where NovelReader applies `bookUrlPattern` to each parsed Search/Explore result.
- **Reason**: A preliminary whole-corpus import let duplicate source URLs replace raws 107 and 126, so raw-index identity did not guarantee the executed contract. After correcting execution, three independent live bodies (raws 669, 703, and 80) parsed 60, 15, and 30 results only when the per-result URL-pattern filter was disabled; upstream Legado does not perform that filtering.
- **Verified**: Exact 50-source import with 50 unique storage keys; authenticated production run plus sequential replay; captured-body counterfactuals; upstream `BookList` comparison; independent reviewer challenge; v14 evidence verifier.
- **Watch out**: `(rawIndex, bookSourceUrl)` remains the corpus/audit identity, but whole-compilation import is not exact-contract execution when duplicate URLs exist. Preserve Search final-detail detection when removing the shared per-result filter.

### [2026-08-11] Removed `bookUrlPattern` list-result filtering
- **Context**: Explore v14 proved that three independent live sources had valid current list entries but NovelReader discarded all of them solely because their URLs did not match stale or over-specific `bookUrlPattern` values.
- **Change**: Removed `bookUrlPattern` compilation and rejection from the shared Search/Explore result loop. Added public-boundary Search and Explore regressions that retain complete resolved results despite a stale pattern.
- **Reason**: Upstream Legado does not apply `bookUrlPattern` as a per-result list filter. NovelReader had no separate final-detail detector using this field, so the smallest compatible fix was deleting only the divergent rejection behavior.
- **Verified**: Full backend tests and vet; race tests for `internal/book`; clean authenticated production replay with exact frozen raws 669, 703, and 80 returned 60, 15, and 30 distinct books respectively with no diagnostics.
