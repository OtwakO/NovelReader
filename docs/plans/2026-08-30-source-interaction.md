---
status: active
updated: 2026-08-30
---

# Reader-owned BookSource interaction

## Goal

Support frequently changing BookSources with portable reader-owned settings, isolated login state, source-defined settings/login controls, working source-native Explore, and optional interactive browser actions without introducing an aggregate-source type or frontend rule execution.

Done means:

- each installed immutable Source ID owns at most one bounded non-secret profile and one bounded authentication record;
- source definition updates preserve owned state while source removal deterministically removes it;
- backup/restore transfers non-secret settings but never source credentials, cookies, or login headers;
- the backend normalizes source-defined controls/actions and the frontend only renders typed controls;
- Explore and normal source execution hydrate the same reader-owned state;
- browser-required actions use authenticated, short-lived remote Patchright sessions;
- unsupported source interactions fail explicitly rather than silently becoming empty results.

## Scope

Included:

- non-secret Source Profiles in `reader.db`;
- source authentication state in the backup-excluded `credentials.db`;
- lifecycle cleanup/reconciliation keyed by immutable Source ID;
- normalized generic controls and backend-owned action execution;
- Explore integration and setup-required diagnostics;
- controlled remote browser sessions with cookie synchronization;
- management actions to clear login data or reset all state for one source.

Excluded:

- an `AggregatedSource` entity or provider-specific aggregate pipeline;
- frontend execution or interpretation of BookSource JavaScript;
- source-version-specific profile migrations;
- automatic captcha solving or WAF bypass;
- full Android `RowUi` layout emulation;
- application-level credential encryption in the initial implementation.

## Accepted Approach

### Ownership

A Source Profile is owned by exactly one installed immutable Source ID within one Reader home. `bookSourceUrl`, source name, collection position, website host, and provider labels are never identity keys.

Durable state is split by portability:

- `reader.db`: one bounded opaque non-secret settings document per Source ID; included in Reader backups;
- `credentials.db`: one bounded login/cookie/header document per Source ID; excluded from Reader backups.

Rendered controls, action descriptors, Source Sessions, and browser sessions are transient and never persisted.

### Source changes and removal

A definition replacement that preserves Source ID keeps durable state but invalidates transient state. A removed Source ID makes all owned state inaccessible immediately and schedules idempotent physical deletion. A newly installed source receives a new Source ID and empty state.

Normal deletion removes source/profile Reader Data and authentication state immediately. Because the databases cannot share a transaction, startup and restore reconcile both profile stores against the authoritative installed Source IDs; this deterministically removes remnants after a crash between the two database commits without introducing a second deletion state machine.

### Source interaction

One backend module exposes a small interface to describe the current normalized controls and execute one source-defined action. It owns parsing/evaluating static or dynamic `loginUi`, source-variable mutations, login data, cookies, notices, refresh effects, and browser-session requests. The frontend receives typed controls/effects only.

Initial controls: informational text, text/password/input, button, toggle, and select. Unsupported controls/actions remain visible as explicit compatibility errors.

### Explore

Explore loads the same Source Profile and authentication state as Search/Book Info/TOC/Content. Informational JavaScript operations such as toast become typed notices. Missing required configuration returns a setup-required result linking to source settings rather than an empty catalog.

### Browser sessions

Interactive browser actions are a separate module from request-oriented WebView transport. Sessions are authenticated, reader- and Source-ID-scoped, bounded, short-lived, process-local, closed on source reset/removal/runtime eviction/shutdown, and synchronize only resulting durable cookie/login state.

### Credential security

The initial implementation does not add application-level encryption or a key-management subsystem. `credentials.db` remains permission-protected inside the Reader home, excluded from portable backups, unavailable through general Source APIs, and redacted from logs/errors. Filesystem or volume encryption is the deployment-level at-rest protection. Credential storage stays behind a narrow module so encryption can be added internally later without changing callers.

## Decisions

### Preserve state across definition updates

**Decision:** Preserve bounded profile/authentication documents when immutable Source ID survives; invalidate transient state.

**Why:** BookSource rules and site layouts change frequently. Logging users out on every synchronized definition update would be hostile and unnecessary.

**Alternatives:** Field-level migration was rejected because external BookSources have no stable profile schema. Reset-on-every-update was rejected because it discards valid user state.

**Revisit when:** A source-definition format introduces an authoritative profile schema/version contract.

### Delete state when Source ID disappears

**Decision:** Deterministically delete all owned durable and transient state when its Source ID is removed.

**Why:** Frequently removed/reinstalled sources must not accumulate credentials, settings, sessions, or browser processes. New installations must not inherit old secrets.

**Alternatives:** Indefinite orphan retention was rejected because it creates remnant data and surprising credential reuse.

### No initial application encryption

**Decision:** Keep source authentication state in backup-excluded, permission-protected `credentials.db` without an application encryption key initially.

**Why:** This avoids key rotation, mass logout, re-encryption, and deployment-secret recovery complexity while preserving portability and a small storage interface. The server administrator is already trusted with process and filesystem access.

**Alternatives:** Installation-key encryption was rejected for the first version because operational complexity and lost-key behavior outweigh the current threat-model benefit.

**Revisit when:** NovelReader supports untrusted storage administrators, managed multi-tenant hosting, or a deployment threat model requiring application-layer at-rest encryption.

## Progress

- [x] Core aggregate Search → Book Info → TOC → Content compatibility through shared execution seams.
- [x] Long Source Group selector is contained within its responsive layout.
- [x] Product/security decisions accepted for generic controls, remote browser sessions, portable state, cleanup, and credential storage.
- [x] Implement Source Profile/authentication storage and startup lifecycle reconciliation.
- [x] Integrate source deletion, collection synchronization, restore reconciliation, and runtime invalidation.
- [x] Add separate clear-login, reset-settings, and full-reset interaction operations.
- [x] Implement normalized read-only source controls and authenticated description API.
- [x] Implement revision-checked source action execution and typed effects.
- [x] Implement the responsive source management interaction UI.
- [x] Hydrate Explore and normal execution from durable source state.
- [ ] Implement authenticated remote browser sessions and cookie synchronization.

## Current State

The unmodified 光遇 aggregate source completes the core text workflow on `feat/aggregated-booksources`. Its Explore currently fails at the first missing interaction bridge (`java.longToast`) and also expects durable source settings. The current Patchright WebView transport is request-oriented and cannot display an interactive browser.

The Source Profile foundation contributes one bounded `source_profiles` document in `reader.db` and one bounded `source_auth_state` document in the backup-excluded `credentials.db`, keyed by installed immutable Source ID. The Reader runtime reconciles both stores against installed IDs when it opens. Reader and credential schema validation remain authoritative, and portable snapshots create an empty credential store while retaining non-secret settings.

Single-source deletion, collection replacement/deletion, and scheduled collection sync coordinate Source Profile cleanup and source-session invalidation at the Reader API/runtime seam. Surviving immutable Source IDs retain durable settings/authentication; removed IDs are reconciled away. Source definition edits invalidate transient sessions without resetting durable state. Restore is covered by runtime-open reconciliation after the replacement runtime is recreated.

The backend evaluates strict JSON, JavaScript object-literal, and dynamic `@js:` `loginUi` definitions into a bounded typed interaction view. The authenticated `GET /api/sources/{id}/interaction` endpoint returns labels, input/select/toggle values, positional action IDs, unsupported-control diagnostics, and a definition/settings revision; it never returns authentication documents or raw action JavaScript. The unmodified aggregate fixture normalizes all 34 controls.

The authenticated action endpoint re-evaluates the current definition, verifies the revision and exposed positional action, supplies Legado's current control-name → string-value map as `result`, and runs `loginUrl` before the selected action. Durable settings/login state and session cookies are hydrated and captured around execution. JavaScript UI side effects are returned as typed notices, search requests, Explore refresh requests, external links, or `browser_required`; raw action programs never cross the API. Successful mutations invalidate transient Search/Book/TOC/Content and opaque Explore sessions for that Source ID. A `refresh_explore` effect reopens the selected catalog only when it belongs to the changed source, including after route restoration from tab-scoped Explore state.

Three explicit destructive operations preserve the installed Source Definition while clearing owned state: clear login removes authentication/cookies only, reset settings removes portable settings only, and full reset removes both. Each operation is idempotent, returns a freshly evaluated interaction view, and invalidates transient sessions after durable cleanup succeeds.

The existing BookSource management page now progressively discloses a dedicated interaction sheet. It renders normalized info, text/password/input, select, toggle, button, and unsupported controls without receiving source JavaScript or raw authentication documents. The sheet is an anchored side panel on desktop and a bounded bottom sheet on mobile, with loading/error/effect feedback, keyboard Escape/initial focus, background scroll locking, and separate confirmations for the three reset scopes. Typed search effects open the existing Search surface with the requested query; browser effects remain explicit unavailable states.

Each reader runtime now injects one Source Profile hydrator into its reader-scoped `book.Searcher`. Search, Book Info, TOC, Content, and Explore call the same preparation seam before evaluating source headers or scripts. A `SourceSession` hydrates at most once after a successful load, so multi-stage workflows retain cookie/variable mutations instead of overwriting them with stale durable state; hydration failures remain retryable. Explore reports a typed `source_setup_required` error when its profile cannot be loaded.

A deterministic production-runtime regression now loads the unmodified 光遇 aggregate fixture, injects only generic reader-owned source settings through the runtime hydrator, and verifies that changing `发现页来源` from 番茄 to 七猫 changes the generated Explore catalog after normal Source ID invalidation. The fixture remains unchanged and production code contains no aggregate/provider-specific branch.

A live candidate-resolution diagnostic on the current branch verified one real aggregate Search result through Book Info, a 106-chapter TOC, and readable content under the production 15-second stage policy. The reported bookshelf failure therefore still needs the exact title/stage/reason before changing candidate logic.

## Next Action

The normalized non-browser source-management workflow is complete. The next milestone is the separately scoped authenticated remote browser-session design and implementation.

## Verification

Verified:

- deterministic aggregate workflow test passes;
- unmodified aggregate source live Search → Detail → 106-chapter TOC → first/middle/last Content succeeds;
- current production candidate validation succeeds for one real aggregate result;
- frontend typecheck passes after group-selector containment.

Still needed:

- remote browser-session contract tests and authenticated end-to-end verification;
- remote browser authorization, expiry, cleanup, and cookie synchronization tests;
- exact evidence for the user-observed bookshelf failure if it persists on current code.

## Open Questions

- Which exact aggregate result failed bookshelf validation, at which stage, and with what displayed attempt reason?
