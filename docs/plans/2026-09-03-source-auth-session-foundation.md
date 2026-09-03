---
status: active
updated: 2026-09-03
---

# Source Authentication and Session Foundation

## Goal

Make BookSource login state correct, inspectable, and maintainable through shared NovelReader seams rather than source-specific patches.

Done means:

- source-defined login actions produce durable reader-owned authentication that every later Search, Explore, Book Info, catalog, content, and controlled-browser workflow hydrates consistently;
- a reader can explicitly inspect and edit the runtime cookies owned by one installed Source ID without editing the imported BookSource definition;
- cookie values, login headers, tokens, credentials, and private source programs never enter ordinary source-list responses, logs, portable backups, test fixtures, or committed diagnostic evidence;
- interactive-browser and backend HTTP cookie transfer preserves the scopes needed by real multi-origin login workflows;
- source runtime identity and required Legado Java bridge behavior are stable and correctly scoped rather than represented by process-global placeholders;
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

Replace process-global placeholder device identity only after Git-history and contract analysis establish the earlier behavior and required scope. The likely owner is the reader runtime or Source Authentication module, exposed to JavaScript through SourceSession. It must be stable across restarts and requests, isolated between readers, and not regenerated per action.

Whether identity is per reader or per reader/source remains an open design decision until the historical and upstream contracts are compared.

### Mediate browser-originated source requests safely

The confirmed all-offline route check is caused by cross-origin `fetch` from an opaque `data:` document. Solve this through a bounded backend-owned request path or request-interception contract at the controlled-browser seam. Reuse existing URL validation, transport policy, cookies, timeouts, and redirect handling where possible.

Do not use global `--disable-web-security`, unrestricted arbitrary proxying, or a source-specific rewrite of the generated page.

### Improve observability without exposing source secrets

Keep raw causes server-side, but classify failures at the owning seam. Useful categories include timeout, transport failure, JavaScript runtime incompatibility, invalid result, and authentication required. Logs and client errors may include operation, stage, source ID, and classification; they must not include cookie/token values, credentials, private URLs, source programs, or response bodies.

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

### No source-specific compatibility patch

**Decision:** Fix only demonstrated shared sourceprofile, SourceSession, analyzer, WebView, or sourceinteraction contracts.

**Why:** The aggregate source reveals the defects but must not become a production type or conditional branch.

**Alternatives:** Rewriting its login or line-check scripts inside NovelReader was rejected as brittle and unmaintainable.

### Credentials remain local and uncommitted

**Decision:** Do not request or use real credentials until history analysis and synthetic reproduction cannot answer the remaining login question. If live credentials become necessary, use them only in an ignored local workflow with sanitized output.

**Why:** Most architecture and compatibility behavior can be established without exposing user secrets, and credentials must never become durable repository context.

## Progress

- [x] Create isolated workstream branch `feat/source-auth-session-foundation`.
- [x] Establish current source-authentication, browser, Explore, and cookie-management architecture.
- [x] Reproduce the all-offline route symptom at the controlled-browser boundary: cross-origin `fetch` from a `data:` page fails with `TypeError` under Chromium CORS.
- [x] Verify with a synthetic endpoint that the current aggregate login path can capture qttoken cookies and login information and invalidate later source sessions.
- [x] Confirm runtime/login cookies currently have no reader-facing inspection or editing interface.
- [ ] Compare Git history to the earlier implementation that registered as an active device.
- [ ] Finalize the runtime authentication representation and typed management interface.
- [ ] Implement and verify cookie-scope-preserving browser synchronization.
- [ ] Implement runtime-cookie inspection/editing API and UI.
- [ ] Correct demonstrated runtime identity and Java bridge semantics.
- [ ] Add secret-safe login and Explore failure classifications.
- [ ] Implement the bounded browser request seam for source-generated route checks.
- [ ] Run focused deterministic verification and optional sanitized live compatibility checks.
- [ ] Update current architecture and complete this plan.

## Current State

The current architecture already separates portable Source Profile settings from backup-excluded Source Authentication and hydrates both into SourceSession. Source actions capture settings/authentication and API actions invalidate cached source sessions. Interactive-browser Finish captures authentication and invalidates source sessions.

Confirmed gaps:

- runtime authentication is not exposed through any typed management interface or UI;
- interactive browser cookies returned with domain metadata are imported against only one final URL, losing original scope;
- controlled Chromium rejects the aggregate route page's cross-origin probes because the page has an opaque `data:` origin;
- `java.androidId()` is a process-global placeholder rather than a stable reader-owned identity;
- `java.timeFormat()` returns an unformatted integer and does not satisfy status-panel date formatting;
- Explore category failures redact the underlying secret cause correctly but provide no safe failure classification;
- existing aggregate tests cover control rendering, settings actions, and Explore settings hydration, but not active-device registration, multi-domain browser cookie persistence, or real line status.

The working tree was clean before this workstream. No real credentials have been requested or stored.

## Next Action

Trace the historical evolution of SourceSession identity, login actions, cookie persistence, and aggregate compatibility to identify when active-device registration stopped working and which shared contract changed. Record only semantic findings and commit references; do not recover or copy historical private source data into this plan.

Then propose the smallest authentication representation/interface that supports full cookie scope and explicit credential management while preserving current reader ownership and backup boundaries.

## Verification

Verified before implementation:

- current branch started from clean `main` at `87a7daf`;
- focused existing aggregate sourceinteraction and Explore hydration tests pass with the ignored local fixture installed;
- WebView worker suite passes: 23 tests;
- a synthetic aggregate login endpoint observed device-cookie submission and durable qttoken/login-info capture;
- a controlled Chromium `data:` page failed a reachable cross-origin CORS fetch with `TypeError`, reproducing the all-offline mechanism;
- diagnostic probes created no tracked files and printed no credentials, token values, private URLs, or source programs.

Still needed:

- historical behavior comparison;
- red-capable deterministic tests for each accepted implementation slice;
- focused backend/frontend/WebView verification after changes;
- optional local live login/status/Explore/route verification with sanitized evidence;
- broader affected-area tests only where shared-session coupling warrants them.

## Open Questions

- Should durable device identity be scoped per reader or per reader/source under the upstream Legado contract and historical working behavior?
- Should the authentication document evolve from URL-to-cookie-header strings to structured cookies, or can a normalized scoped-cookie interface preserve compatibility without a storage-shape change?
- Should explicit secret reveal require only the current authenticated reader session, or current-password reauthentication? This affects UX and security and must be confirmed before implementing the frontend flow.
- Does the real login/device workflow require browser storage in addition to cookies and login information? Add it only with evidence.
- What is the smallest safe request-interception interface that supports source-generated browser diagnostics without becoming an unrestricted browser proxy?
