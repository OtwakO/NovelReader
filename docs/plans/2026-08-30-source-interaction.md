---
status: completed
updated: 2026-08-31
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

Interactive browser actions extend the existing private Patchright WebView sidecar rather than introducing a second browser deployment. The existing one-shot `/execute` transport remains unchanged; a small interactive protocol keeps an isolated context alive only for screenshot, input, finish, or cancellation operations.

A reader-runtime-owned browser-session module mediates every interactive session. The worker knows only opaque worker session IDs and browser state; the Go boundary owns Reader/Source authorization, durable authentication synchronization, source-session invalidation, and public session IDs.

Lifecycle is a correctness invariant: every created browser context immediately receives one registry owner and one capacity slot; close removes ownership before awaiting browser cleanup; close is idempotent; inactivity and absolute deadlines are enforced by a worker-side sweeper independent of client behavior; worker shutdown closes the sweeper, all contexts, then Chromium; Reader runtime eviction, source reset/removal, and server shutdown also request closure. Sessions are process-local and never restored after a restart.

The transport deliberately uses bounded screenshot polling plus click, text, key, and scroll input. It does not add WebRTC/VNC, persistent workflows, multiple tabs, or file transfer. `startBrowserAwait` continuation is implemented as a one-use replay recipe owned by the Reader runtime: the Goja stack is never retained while a browser is open; Finish returns bounded final HTML and re-evaluates the same exposed action with a StrResponse-compatible body; Cancel discards the recipe. Recipes are revision-bound, limited to four await steps and two MiB cumulative returned HTML, and remain process-local.

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
- [x] Implement authenticated remote browser sessions and cookie synchronization.
  - [x] Extend the existing worker with bounded expiring interactive contexts and idempotent cleanup.
  - [x] Add the Go reader/source ownership module and authenticated routes.
  - [x] Add the screenshot/input/finish UI and synchronize cookies into Source Profile authentication.
  - [x] Verify expiry, cancellation, runtime/source cleanup, capacity release, and worker shutdown.

## Current State

Reader-owned BookSource interaction is implemented. Each immutable Source ID owns bounded portable settings and backup-excluded authentication state; runtime hydration feeds Search, Book Info, TOC, Content, Explore, and source actions through the shared SourceSession seam. Source deletion, collection replacement, restore reconciliation, definition edits, explicit reset operations, and runtime eviction preserve or remove state according to Source ID ownership and invalidate transient sessions where required.

The backend normalizes source-defined controls and actions into typed authenticated interfaces. The frontend renders controls and effects without receiving raw action programs or authentication documents. Explore refreshes only the affected selected source after settings or login changes.

Bounded interactive browser sessions run through the shared WebView worker protocol v4. Source-emitted URLs and JavaScript remain backend-only behind opaque one-use request IDs. Browser contexts are reader/source owned, share worker capacity, expire independently of frontend traffic, release capacity on every terminal path, render source-provided bounded HTML documents, support remote wheel/swipe input, and persist cookies only on explicit Finish. `startBrowserAwait` continuations resume once with bounded returned HTML; Cancel persists no returned state.

Candidate TOC parsing now respects stage cancellation throughout fetch, page parsing, extraction, deduplication, and title formatting. Explore source selection requires both global source enablement and Explore enablement; cached frontend selection clears when the refreshed source list no longer contains it.

## Next Action

This workstream is complete. Further candidate/catalog lifecycle work belongs to [`2026-08-31-catalog-synchronization.md`](2026-08-31-catalog-synchronization.md), not this historical plan. Future Source Interaction compatibility should begin from a demonstrated shared Legado gap and use a new focused workstream when substantial.

## Verification

Verified at completion:

- complete backend suite passes: `go test ./...`;
- complete frontend suite passes: 44 files / 143 tests;
- frontend `vue-tsc --noEmit` passes;
- frontend production Vite build succeeds;
- WebView worker suite passes: 23 tests;
- deterministic conformance fixtures use the authoritative WebView protocol v4;
- aggregate workflow, Source Profile lifecycle, action normalization, settings replay, browser ownership/expiry/capacity, continuation, cookie synchronization, Explore refresh, and source/runtime cleanup regressions pass;
- scoped diagnostics report no errors or warnings; Go LSP was unavailable, so the complete Go test run is the authoritative compile gate.
