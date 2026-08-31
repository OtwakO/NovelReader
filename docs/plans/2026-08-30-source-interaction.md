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

Interactive browser actions extend the existing private Patchright WebView sidecar rather than introducing a second browser deployment. The existing one-shot `/execute` transport remains unchanged; a small interactive protocol keeps an isolated context alive only for screenshot, input, finish, or cancellation operations.

A reader-runtime-owned browser-session module mediates every interactive session. The worker knows only opaque worker session IDs and browser state; the Go boundary owns Reader/Source authorization, durable authentication synchronization, source-session invalidation, and public session IDs.

Lifecycle is a correctness invariant: every created browser context immediately receives one registry owner and one capacity slot; close removes ownership before awaiting browser cleanup; close is idempotent; inactivity and absolute deadlines are enforced by a worker-side sweeper independent of client behavior; worker shutdown closes the sweeper, all contexts, then Chromium; Reader runtime eviction, source reset/removal, and server shutdown also request closure. Sessions are process-local and never restored after a restart.

The first transport deliberately uses bounded screenshot polling plus click, text, key, and scroll input. It does not add WebRTC/VNC, persistent workflows, multiple tabs, file transfer, or `startBrowserAwait` continuations.

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

The lean interactive-browser slice is implemented on the existing WebView worker. Interactive source pages render in a portrait mobile-width viewport on every client (CSS width 390–430); its height follows the actual visible screenshot panel within a 470–900 bound, avoiding browser-side raster scaling while preserving the phone-WebView layout most Legado sources target. The frontend also supplies a bounded client device-pixel ratio (1–3) for sharp capture. Patchright screenshots explicitly use device-pixel scale and JPEG quality 95, retrying through lower bounded qualities only when a frame would exceed the response budget; this prioritizes polling latency while retaining full client-density resolution. Mouse-wheel and touch-swipe input scroll the remote Patchright page rather than the static screenshot container; document scrolling uses deterministic `window.scrollBy` with a mouse-wheel fallback only when the main document cannot move (nested custom scrollers). The frontend removes local screenshot scrolling, coalesces wheel deltas, sequences polling/input frames so stale polls cannot overwrite newer interaction results, and keeps scroll activity separate from the overlay controls' disabled/loading state. Source-emitted URLs are replaced with one-use opaque browser-request references before reaching the frontend; one active browser context is owned per Reader runtime; worker-side idle/absolute expiry and shutdown cleanup remain independent of client behavior; source reset/removal/update, Reader runtime eviction/quiesce, and UI cancellation request cleanup. Cookies synchronize into the Source Profile only on explicit Finish login. Both `startBrowser` and `startBrowserAwait` now open the browser, including bounded `data:text/html;base64,...` documents used by source-owned in-memory settings pages; arbitrary data schemes remain rejected. True await continuation/body-return semantics remain intentionally unsupported, so settings pages that persist by parsing the returned HTML can be displayed but cannot yet apply their returned values. Action-only Explore URL templates (`{{java.startBrowser(...)}}`) route through the same typed action/browser seam instead of being treated as fetch categories.

## Verification

Verified:

- deterministic aggregate workflow test passes;
- unmodified aggregate source live Search → Detail → 106-chapter TOC → first/middle/last Content succeeds;
- current production candidate validation succeeds for one real aggregate result;
- frontend typecheck passes after group-selector containment.

Still needed:

- worker: 14 lifecycle/protocol tests pass, covering inactivity expiry, idempotent close, shutdown cleanup, launch-failure and partial-context capacity release, and existing one-shot behavior;
- backend: `go test ./internal/webview ./internal/sourceinteraction ./internal/api ./cmd/server` passes, including opaque request, ownership, cookie sync, and source/runtime cleanup seams;
- frontend: SourceInteractionSheet tests pass (5/5), `vue-tsc --noEmit` passes, and the production Vite build succeeds; browser unmount cancellation is pinned;
- `aft_inspect` reports no diagnostics in the affected scope; Go LSP was unavailable, so the targeted Go test run is the authoritative compile gate;
- exact evidence for the user-observed bookshelf failure if it persists on current code.

## Open Questions

- Which exact aggregate result failed bookshelf validation, at which stage, and with what displayed attempt reason?
