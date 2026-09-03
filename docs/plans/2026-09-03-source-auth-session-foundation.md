---
status: completed
updated: 2026-09-04
---

# Source Authentication and Session Foundation

## Goal

Make BookSource login state correct, inspectable, and maintainable through shared NovelReader seams rather than source-specific patches.

Done means:

- source-defined login actions produce durable reader-owned authentication that every later Search, Explore, Book Info, catalog, content, and controlled-browser workflow hydrates consistently;
- a reader can explicitly inspect and edit the runtime cookies owned by one installed Source ID without editing the imported BookSource definition;
- cookie values, login headers, tokens, credentials, and private source programs never enter ordinary source-list responses, logs, portable backups, test fixtures, or committed diagnostic evidence;
- interactive-browser and backend HTTP cookie transfer preserves the scopes needed by real multi-origin login workflows;
- source runtime identity and demonstrated Legado Java bridge behavior are stable and correctly scoped rather than represented by process-global placeholders;
- Explore/login failures expose a useful secret-safe classification instead of collapsing distinct causes into one generic message;
- source-generated route diagnostics work through a bounded backend-owned browser networking contract, without globally disabling browser security;
- regressions are covered at shared production seams with minimal synthetic tests, followed by explicit optional live verification using ignored local BookSources.

## Scope

Included:

- the authentication document and persistence interface in `backend/internal/sourceprofile/`;
- session hydration/capture in `backend/internal/sourceexec/` and reader-runtime adapters in `backend/internal/api/`;
- source-defined actions and browser continuation state in `backend/internal/sourceinteraction/`;
- shared JavaScript bridge behavior in `backend/internal/analyzer/` where demonstrated by the login/status workflow;
- controlled browser cookie synchronization and the smallest safe browser-network seam required by source-generated diagnostics;
- an authenticated typed runtime-cookie management interface;
- a focused source-interaction UI for viewing, revealing, editing, and removing runtime cookies;
- secret-safe diagnostics for login and Explore category failures;
- Git-history comparison with the earlier active-device-compatible implementation;
- focused deterministic tests and optional local compatibility checks against ignored private fixtures.

Excluded unless later evidence makes them necessary:

- an aggregate-source type or provider-specific production branch;
- storing credentials in imported BookSource JSON or portable Reader Data;
- exposing runtime authentication through `GET /api/sources`;
- unrestricted raw credential-database editing;
- frontend execution or interpretation of BookSource JavaScript;
- globally disabling Chromium web security or CORS;
- automatic captcha solving, WAF bypass, or continuous source-health monitoring;
- a general browser proxy framework, credential vault product, or migration framework without a demonstrated second requirement;
- committing real BookSources, credentials, cookies, tokens, private endpoints, or unsanitized live responses.

## Accepted Approach

### Start from the shared authentication lifecycle

Treat authentication as one reader-owned lifecycle:

```text
Source action or controlled browser
  → SourceSession mutations
  → bounded authentication capture
  → credentials store for one immutable Source ID
  → transient-session invalidation
  → hydration into every later source workflow
```

The implementation must repair this lifecycle at its owning seam. It must not special-case the current aggregate fixture or duplicate authentication handling in Search, Explore, reading, and browser callers.

### Keep definition, settings, and authentication distinct

Continue the existing ownership split:

- imported BookSource definition: external source program and static source headers;
- Source Profile settings: reader-owned, non-secret, portable opaque settings;
- Source Authentication: reader-owned, secret, backup-excluded runtime login state;
- SourceSession: transient hydrated execution state.

The runtime-cookie editor operates on Source Authentication. It does not rewrite the BookSource definition and must clearly distinguish runtime cookies from source-defined headers.

### Put cookie management behind a typed credential interface

Extend the existing source-profile/authentication module rather than allowing frontend or API code to manipulate credential JSON directly. The module should own validation, normalization, bounded storage, and replacement/removal semantics.

The HTTP interface will be authenticated and Source-ID-owned. Ordinary reads remain redacted. Secret values are returned only by an explicit credential-management operation and must never be cached in frontend storage or written to logs.

The initial UI belongs in the existing Source Interaction sheet because login, reset, browser authentication, and source settings already meet there. It should provide a focused session-data section instead of expanding the raw BookSource editor.

### Preserve browser cookie scope

Investigate and correct cookie transfer at the browser/client seam. The current worker can return cookies from multiple domains, while the Go client currently imports them against one final URL. The corrected representation must preserve enough domain/path/security information for later requests without leaking browser implementation details into feature callers.

Do not add browser storage persistence unless the demonstrated login contract requires it. Cookies, login headers, and source login information remain separate concepts.

### Use stable reader-owned runtime identity

Replace the process-global placeholder with one reader-owned identity injected through reader-scoped analyzer state. The user selected per-Reader scope shared across that reader's sources: it matches NovelReader's isolation boundary without registering one reader as many provider devices or correlating separate reader accounts. It must be stable across restarts and requests and not regenerated per action.

The current compatibility target is an identity that the provider can register and report through its normal active-device/status workflow. This gives an observable end-to-end proof that login, token, device identity, and status requests agree. Preserve the exact cause and compatibility switch point once identified so a reader can intentionally reset or rotate NovelReader's local identity and understand that this may register a new provider-side device or require reauthentication.

History shows the static bridge value existed throughout both the earlier reportedly-counted behavior and the later credential split, so no exact active-device regression commit can be claimed. Do not add a mode intended to evade an upstream device limit; local identity lifecycle controls must remain explicit account/session management, while provider-side counting and policy stay authoritative.

### Mediate browser-originated source requests safely

The confirmed all-offline route check is caused by cross-origin `fetch` from an opaque `data:` document. Solve this through a bounded backend-owned request path or request-interception contract at the controlled-browser seam. Reuse existing URL validation, transport policy, cookies, timeouts, and redirect handling where possible.

Do not use global `--disable-web-security`, unrestricted arbitrary proxying, or a source-specific rewrite of the generated page.

### Improve observability without exposing source secrets

Keep raw causes server-side, but classify failures at the owning seam. Useful demonstrated categories include timeout, transport failure, JavaScript runtime incompatibility, and invalid result. Add authentication-required only when a typed internal producer can prove it; do not infer it from provider text. Logs and client errors may include operation, stage, source ID, and classification; they must not include cookie/token values, credentials, private URLs, source programs, or response bodies.

### Verification strategy

Use the smallest authoritative seam for each risk:

- authentication capture/hydration: sourceprofile/sourceexec integration tests;
- action login semantics and invalidation: sourceinteraction/API tests;
- browser cookie scope: webview client and source-browser API tests;
- cookie management: credential-module and authenticated HTTP tests plus one focused Vue interaction test;
- route checks: synthetic controlled-browser test with a reachable cross-origin endpoint;
- Explore diagnostics: deterministic JavaScript/transport failures through the production Explore interface;
- live compatibility: explicit local run using ignored private fixture and credentials only after deterministic tests pass.

Do not create a synthetic duplicate of the private aggregate source. Reduce each demonstrated defect to the smallest reusable contract.

## Decisions

### Runtime cookies are user-manageable credentials

**Decision:** Readers may explicitly inspect and edit the runtime cookies owned by their installed Source ID.

**Why:** Runtime cookies affect source behavior but are currently invisible, making login diagnosis and recovery impossible without database access. They are reader-owned state, not an immutable implementation secret.

**Alternatives:** Reset-only management was rejected because it cannot diagnose or repair partial/multi-domain sessions. Editing imported source JSON was rejected because runtime authentication is not source-definition state and synchronized collections may overwrite definition edits.

**Consequences:** Secret reads need an explicit authenticated operation, masked-by-default UI, no client persistence, bounded validation, and session invalidation after changes.

### Restore provider-visible device monitoring first

**Decision:** Make the current login workflow register and appear through the provider's normal active-device/status contract before considering identity reset or rotation controls.

**Why:** A provider-visible device is a stronger end-to-end compatibility signal than successful book reading alone. It proves the source's login, token, device identity, request, persistence, and status paths agree.

**Alternatives:** Keeping the uncounted behavior as the default was rejected because its cause is unknown and may conceal an incorrect login implementation. Treating non-counting as a supported bypass was rejected because NovelReader should not deliberately evade an upstream device limit.

**Consequences:** The workstream must record the regression cause and the exact identity lifecycle behavior. A later reader-facing control may inspect, unregister, reset, or rotate local identity with clear reauthentication/provider-device consequences, but not promise avoidance of provider policy.

### No source-specific compatibility patch

**Decision:** Fix only demonstrated shared sourceprofile, SourceSession, analyzer, WebView, or sourceinteraction contracts.

**Why:** The aggregate source reveals the defects but must not become a production type or conditional branch.

**Alternatives:** Rewriting its login or line-check scripts inside NovelReader was rejected as brittle and unmaintainable.

### Credentials remain local and uncommitted

**Decision:** Do not request or use real credentials until history analysis and synthetic reproduction cannot answer the remaining login question. If live credentials become necessary, use them only in an ignored local workflow with sanitized output.

**Why:** Most architecture and compatibility behavior can be established without exposing user secrets, and credentials must never become durable repository context.

### Require current-password reauthentication for runtime-cookie access

**Decision:** Revealing or saving runtime cookie values requires current-password confirmation. Ordinary source interaction, login actions, masked cookie metadata, and authentication reset remain available through the normal signed-in session.

**Why:** Runtime cookies are reusable source credentials. An unlocked NovelReader browser session should not be sufficient to silently exfiltrate or replace them.

**Alternatives:** Signed-in-session-only access was rejected as too weak. Password-to-reveal but session-only save was rejected because asymmetric authorization would make the interface harder to understand and audit.

**Consequences:** Reuse the existing account password-verification seam; do not duplicate password hashing or credential checks inside sourceprofile/sourceinteraction. Passwords are request-only, never persisted or logged, and successful confirmation authorizes only the specific credential operation.

## Progress

- [x] Create isolated workstream branch `feat/source-auth-session-foundation`.
- [x] Establish current source-authentication, browser, Explore, and cookie-management architecture.
- [x] Reproduce the all-offline route symptom at the controlled-browser boundary: cross-origin `fetch` from a `data:` page fails with `TypeError` under Chromium CORS.
- [x] Verify with a synthetic endpoint that the current aggregate login path can capture qttoken cookies and login information and invalidate later source sessions.
- [x] Confirm runtime/login cookies currently have no reader-facing inspection or editing interface.
- [x] Compare Git history to the earlier implementation and record that no exact active-device regression commit is provable.
- [ ] Restore provider-visible active-device monitoring through the normal login/status contract.
- [ ] Define an explicit local device-identity reset/rotation lifecycle without promising provider-limit avoidance.
- [x] Finalize the initial runtime-cookie representation and typed credential-module interface without changing the deployed authentication document shape.
- [x] Implement the current-password-protected runtime-cookie HTTP interface.
- [x] Implement and verify cookie-scope-preserving browser synchronization.
- [x] Implement runtime-cookie inspection/editing API and UI.
- [x] Correct demonstrated runtime identity and Java bridge semantics.
- [x] Add secret-safe login and Explore failure classifications.
- [x] Implement the bounded browser request seam for source-generated route checks.
- [x] Run deterministic verification and complete branch-first code/security review; live provider compatibility remains explicitly pending.
- [ ] Update current architecture and complete this plan.

## Current State

The current architecture already separates portable Source Profile settings from backup-excluded Source Authentication and hydrates both into SourceSession. Source actions capture settings/authentication and API actions invalidate cached source sessions. Interactive-browser Finish captures authentication and invalidates source sessions.

Confirmed remaining gaps:

- live verification against the updated ignored aggregate source reported switch-line probes changing from uniformly offline to HTTP 502. A controlled curl differential then proved the gateway root returns success only when the reader-owned account cookie is present; the source's generated fetch omits `credentials: include`, so ordinary Chromium does not send hydrated cookies from an opaque `data:` origin. The mediator must therefore attach only cookies selected by the browser context for the exact destination URL;
- sandbox probes remained environment-dependent and were not authoritative for provider availability. The user-side cookie differential supplies the missing causal evidence: authenticated normal WebView requests work, while mediated opaque-page probes lacked the scoped account cookie;
- sanitized latest-source Explore reproduction showed category generation fails only when `source.getVariable()` is empty; a minimally valid current-schema variable produces two categories with or without login info. Legado-E also defines missing variables as the empty string, so the engine must not invent source-specific defaults; the user should reapply the updated source's settings to initialize/migrate its variable document;
- existing aggregate tests cover control rendering, settings actions, and Explore settings hydration against a substituted synthetic gateway, but are not red-capable for the latest source's real Explore, device-registration, or route-response contracts.

The first implementation slice adds `sourceprofile.Store.RuntimeCookies` and `ReplaceRuntimeCookies` behind the existing credential store. It keeps the deployed URL-scope-to-cookie-header representation, validates bounded HTTP(S) scopes and cookie syntax, returns stable ordering, and replaces only cookies while preserving login information and login headers. This avoids a speculative structured-cookie migration before domain/path attribute fidelity is demonstrated as necessary.

The authenticated HTTP interface exposes masked scope/name metadata through the normal reader session and requires current-password confirmation for both value reveal and complete replacement. It reuses `auth.AccountService.VerifyPassword` through an auth-owned method, prevents credential response caching, and invalidates the affected source runtime after replacement. Sourceprofile and sourceinteraction do not receive password or account-authentication responsibilities.

The Source Interaction sheet now contains a focused session-cookie editor. It loads only scopes and names initially, reveals values after current-password confirmation, reuses that in-memory confirmation for editing and the immediately following save, retains the editing session after a failed save, clears secret state on close/cancel/successful save, and immediately returns to masked metadata after replacement. The UI edits the existing complete scope-to-cookie-header document rather than source JSON or global state.

Interactive browser close imports each returned cookie against its own protocol URL or domain-derived HTTP(S) scope, falling back to the browser URL only for host-only cookies without scope metadata. Multi-domain browser sessions therefore remain separated instead of assigning every cookie to the final page URL. The current durable scope-URL-to-header document does not claim complete path, expiry, SameSite, or overlapping-cookie fidelity; evolving to structured cookies remains a separate evidence-backed model change. Source-generated `data:` browser contexts hydrate all stored scopes into Chromium without global login/source headers. For each mediated fetch/XHR, the worker asks Chromium for cookies applicable to that exact destination and forwards only those cookies unless the request already supplied an explicit Cookie header.

History establishes that the static `goja-android-id` value existed unchanged from the bridge's July 3 introduction through both the earlier reportedly-counted behavior and the August 30 durable credential split. Provider-visible device counting was never a first-class NovelReader contract, so no single later regression commit can be claimed honestly. The new contract is explicit instead: NovelReader derives one opaque stable device identifier from the immutable Reader ID, shares it across that reader's sources, and injects it into both `java.androidId()` and `java.deviceID()` through reader-scoped analyzer state. This needs no credential schema migration, reveals no Reader UUID, survives runtime recreation, and remains account-neutral across portable restore.

Controlled base64 HTML `data:` documents now receive a narrow WebView-worker mediator for fetch/XHR probes. It accepts only HTTP(S), rejects hostname resolutions containing any non-public address, verifies the connected server address before exposing the response, follows no redirects, preserves the worker timeout/body limits, and grants access only to the opaque `null` origin. Other browser resources use normal Chromium handling. A real Chromium regression proves a generated opaque document can read a reachable route result through this seam without globally disabling browser security.

The demonstrated Java date compatibility gap is corrected against both repository-local references: `reference/legado-E/app/src/main/java/io/legado/app/help/JsExtensions.kt` delegates epoch milliseconds to the application-local `dateFormat`, while `reference/web-legado-rust/src/parser/js.rs` explicitly specifies `yyyy/MM/dd HH:mm`. NovelReader now implements that shared contract in its process local timezone. No speculative extra overload was added.

Source execution failures now use a small shared secret-safe classification vocabulary. Explore keeps its existing operation code/stage/status while adding a classification; source interaction wraps JavaScript and invalid-result failures in a typed safe error so raw source exceptions are retained only as server-side causes. Typed deadline/network failures override the operation fallback. No provider text is inspected to guess authentication state.

No real credentials have been requested or stored.

## Outcome

Completed. User-side live verification against the latest ignored aggregate source confirms Search, reading, user-status WebView, authenticated route switching, and Explore all work. Explore required explicitly saving the updated source settings once so its current variable schema replaced the empty/outdated profile. The runtime-cookie editor uses one current-password entry for reveal/edit/save.

Provider-visible active-device count remains 0, but authenticated operations prove the account cookie is accepted. No NovelReader defect in the count contract is established, and the count is non-blocking; do not add speculative registration calls, provider-specific endpoints, identity rotation, or source-specific behavior. Reopen only if future evidence isolates a reusable compatibility gap.

## Verification

Verified before implementation:

- current branch started from clean `main` at `87a7daf`;
- focused existing aggregate sourceinteraction and Explore hydration tests pass with the ignored local fixture installed;
- WebView worker suite passes: 23 tests;
- a synthetic aggregate login endpoint observed device-cookie submission and durable qttoken/login-info capture;
- a controlled Chromium `data:` page failed a reachable cross-origin CORS fetch with `TypeError`, reproducing the all-offline mechanism;
- diagnostic probes created no tracked files and printed no credentials, token values, private URLs, or source programs.

Completed implementation verification:

- runtime-cookie credential module tests pass for stable listing, replacement isolation, and invalid-input rejection;
- authenticated runtime-cookie HTTP test passes for masked metadata, wrong-password denial, no-store reveal, replacement isolation, and durable storage;
- focused frontend tests pass for masked initial metadata, one-password reveal/edit/save, failed-save retry, and return to masked state;
- frontend typecheck and production build pass;
- WebView client tests pass, including a regression proving cookies from two browser-returned domains remain scoped to those domains;
- analyzer, readerstore, sourceinteraction, book, and API tests pass for the stable per-reader identity slice, including stable/different-reader derivation and both `java.androidId()`/`java.deviceID()` bindings;
- WebView worker suite passes with 32 tests, including hostname and connected-address private-network rejection, mocked policy boundaries, and a real Chromium regression proving an opaque-document mediated fetch receives only the browser-selected destination-scoped cookie;
- analyzer, sourceinteraction, and book tests pass for the upstream-compatible `java.timeFormat(milliseconds)` local date formatting;
- sourceexec, sourceinteraction, book, and API tests pass for failure classification; deterministic API regressions confirm JavaScript failures expose `javascript_runtime` without leaking the raw source cause;
- review regression passes for current-password denial without false application-session loss; later live evidence justified scoped-cookie hydration for opaque-page mediation while global login/source headers remain excluded;
- frontend typecheck plus existing API error, Explore store, and Source Interaction sheet tests pass with the additive classification field;
- full backend `go test ./...` passes;
- full frontend suite passes: 51 files, 166 tests, followed by production build;
- full WebView worker suite passes: 32 tests, followed by Python bytecode compilation;
- `main...HEAD` has no diff-check errors or changed paths under `test-booksources/`; a branch-added-line credential scan found no token, cookie, authorization, or password value literals (only three translated `currentPassword` field labels).

Deferred, not required for completion:

- provider-visible active-device counting: current count remains 0 but all authenticated functional paths work; investigate only with new evidence that identifies a reusable NovelReader compatibility defect;
- structured cookie attributes, browser storage, and identity reset/rotation controls: add only when a demonstrated workflow requires them.
