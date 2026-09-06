# NovelReader — Project Plan

`PLAN.md` is the current-state router. Load only the linked document relevant to the task; completed plans and archived material are historical evidence, not live backlogs.

## Objective

Build a self-hosted, web-first novel reader with strong practical compatibility with Legado BookSource JSON while keeping reader data isolated, portable, and understandable.

NovelReader must:

- preserve imported BookSource definitions losslessly, including unknown fields;
- execute source-defined requests, rules, JavaScript, sessions, cookies, typed data, and WebView work on the backend;
- expose typed Search, Explore, Book Detail, catalog, source-recovery, and Reader workflows to the Vue frontend;
- isolate every Reader Account's sources, books, progress, files, server-owned settings, and source interaction state;
- fail explicitly without source-specific production patches, captcha solving, or automatic WAF bypass.

## System Map

```text
Vue 3 SPA
  → typed authenticated HTTP/SSE interfaces
  → book/search/explore/candidate/source-interaction workflows
  → shared source executor, analyzer, JavaScript bridge, and SourceSession
  → HTTP, fingerprint, typed-data, or Patchright WebView transport
  → per-reader SQLite data and files
```

Repository ownership:

- `backend/internal/api/` — authenticated HTTP/SSE adapters.
- `backend/internal/auth/` — accounts, sessions, setup, recovery, and administration.
- `backend/internal/readerstore/` — reader homes, schemas, lifecycle, backup staging, and replacement.
- `backend/internal/booksource/` — lossless BookSource model and persistence.
- `backend/internal/sourceexec/` — shared request construction, source sessions, and transport routing.
- `backend/internal/analyzer/` — Legado-compatible rules and JavaScript bridge.
- `backend/internal/book/` — Search, Explore parsing, Book Info, catalogs, content, shelf, source binding, progress, and bookmarks.
- `backend/internal/candidate/` — bounded metadata-first shelf admission.
- `backend/internal/sourceinteraction/` — reader-owned source settings, credentials, actions, and browser continuations.
- `backend/internal/backup/` — portable Reader Data archive and staged restore workflows.
- `backend/internal/webview/` and `webview-worker/` — versioned Go/Patchright browser seam.
- `frontend/src/features/` — feature-owned Vue workflows and presentation.
- `testdata/booksource/` — minimal synthetic conformance fixtures and sanitized dated evidence; historical tracked source material is not precedent for new fixtures.
- `test-booksources/` — ignored private local corpora, never committed or required by CI.

Current architecture:

- [Authentication, reader storage, and backup](docs/architecture/authentication-and-reader-storage.md)
- [Discovery, shelf, catalogs, and reading](docs/architecture/discovery-and-reading.md)
- [Domain language](docs/reference/domain-language.md)

Accepted future-facing architecture:

- [Reading documents, resources, and modality renderers](docs/decisions/0002-reading-documents-and-resources.md)

## Current State

### Complete product foundations

- Local Reader Accounts, setup, registration policy, recovery, password management, administration, and durable deletion.
- Per-reader storage with isolated `reader.db`, files, source profile state, and encrypted source credentials.
- Portable Reader Data backup/restore with scoped automation tokens and staged atomic replacement.
- Lossless BookSource import/export, Source Collections with independently persisted Search/Explore availability, manual/scheduled collection synchronization, and duplicate source definitions.
- Shared HTTP/fingerprint/typed-data/WebView execution with bounded source sessions and process capacity.
- Batched streaming Search and strict single-source Explore.
- Metadata-first shelf admission: bounded Book Info selects a source; catalog synchronization is separate, single-flight, cached in SQLite, observable, retryable, and atomically published.
- Logical-book identity by normalized title/author with exact `(SourceID, BookURL)` source bindings, unified source recovery, and atomic source switching.
- Reader progress and bookmarks, bounded session chapter reuse, default-on next-chapter prefetch, ordered non-blocking progress saves, Chinese conversion reuse, explicit Refresh, and source-switch invalidation.
- Browser-local typography/image/wake-lock preferences, responsive reader controls, keyboard navigation, TOC filtering/ordering/current positioning, and shelf filtering/restoration.
- Compact source-management summaries with on-demand lossless editing, plus shared full-cover presentation and reader-scoped seven-day cover caching.
- Reader-owned source interaction, settings, login state, controlled browser sessions, and bounded `startBrowserAwait` continuation replay.
- Installable Vue PWA; production GHCR Compose, checkout-built local Compose with bind-mounted data, and separate deterministic Compose E2E.
- Worker-wide Chrome headless/headful selection and paired app/worker release publication with exact-image verification.

### Compatibility position

The major text-reading journeys are operational. Remaining compatibility work is incremental and evidence-driven, concentrated in shared analyzer, JavaScript bridge, session, request, source-interaction, and media-specific semantics. NovelReader does not claim universal parity with every Android/JVM-only Legado behavior.

Use [Legado compatibility roadmap](docs/roadmaps/legado-compatibility.md) for unresolved capability families and [archived compatibility tracker](docs/archive/audits/legado-compatibility-tracker-2026-08.md) for the completed 2026-08 audit queue.

### Completed workstream handoffs

[Parallel release builds](docs/plans/parallel-release-builds.md) — concurrent production builds are
merged and verified. The first release passed in 5m21s with a cached app layer; source-change timing
and comparison limits are recorded in the plan. All verification and publication gates remain.

[GitHub Actions runtime](docs/plans/github-actions-runtime.md) records the initial conservative
optimizations and successful hosted verification (7m42s → 6m35s in the first comparison).

[Reader navigation performance](docs/plans/reader-navigation-performance.md) records the navigation/cache/conversion lifecycle and controlled timing evidence. Live-source timing remains unverified.

The completed [WebView Runtime Efficiency](docs/plans/2026-09-04-webview-runtime-efficiency.md) workstream provides worker-wide Chrome headless/headful selection, leaner packaging, and latest-stable release-image verification while preserving per-request reader isolation and the bounded WebView security seam. Further live compatibility and footprint measurements remain optional, evidence-gated follow-up.

The completed [Source Authentication and Session Foundation](docs/plans/2026-09-03-source-auth-session-foundation.md) work established reader-owned login/session state, scoped runtime-cookie management, secret-safe diagnostics, and bounded authenticated controlled-browser networking. The completed [Source Collection availability](docs/plans/2026-09-02-source-collection-availability.md) work added a collection-level Search/Explore gate while preserving every member source's individual settings and existing shelf reading. The completed [reading document foundation](docs/plans/2026-09-02-reading-document-foundation.md) established the versioned prose-document, opaque-resource, and focused prose-renderer seams around the current BookSource text/image path.

The completed [architecture and code quality improvements](docs/plans/2026-09-05-architecture-code-quality-improvements.md) workstream corrected lifecycle/isolation defects, upload/font/identity contracts and reader-handler ownership, and implemented measured narrow chapter/progress lookups. Reader schema epoch 9 requires matching/fresh development data. Its plan records scoped verification and the approved local-only integration; hosted CI and deployment verification remain unperformed. Frontend decomposition stays evidence-gated.

## Active Work

[BookSource engine compatibility audit](docs/plans/booksource-engine-compatibility-audit.md) — independent shared-engine review anchored in a frozen private 50-source Search/Book Info sample and upstream rule/reference comparisons. Confirmed shared corrections are being implemented in tested steps; current outcomes and remaining work are in the plan. No source-specific patches or real BookSources committed.

## Immediate Priorities

1. Continue the accepted [BookSource engine corrections](docs/plans/booksource-engine-compatibility-audit.md), the bounded browser-UA provider is implemented; prioritize the user's lifecycle hardening request after confirming worker failure escalation. Three ownership/admission gaps are recorded in the active plan; other compatibility investigations are deferred. Do not claim universal compatibility.
2. Select further compatibility slices from current evidence rather than historical unchecked boxes.
3. Introduce provider capability interfaces only when a first non-BookSource provider is accepted; introduce image-sequence documents and structured locations only when that modality becomes active work.
4. Consider still-relevant Reader UX opportunities only after explicit approval; see [Reader UX roadmap](docs/roadmaps/reader-ux.md).
5. Finish consistent display of source-provided `updateTime` metadata if that presentation improvement is prioritized.

## Durable Decisions

- **Compatibility:** match documented and observed Legado behavior at shared seams before considering source-specific behavior.
- **Frontend seam:** Vue consumes typed domain interfaces and never executes BookSource rules or interprets opaque source payloads.
- **Reading seam:** providers open Reading Sections as modality-specific Reading Documents; documents use opaque Content Resources and the Reading Session delegates to modality renderers. See [decision 0002](docs/decisions/0002-reading-documents-and-resources.md).
- **Source identity:** immutable NovelReader Source ID; imported `bookSourceUrl` is source data and may duplicate.
- **Book identity:** normalized title plus author identifies a logical shelf book; exact source bindings live beneath it.
- **Shelf admission:** Book Info metadata is sufficient for admission; catalog availability is a separate observable state.
- **Explore:** one selected BookSource and its native catalog at a time; Search/Explore eligibility combines saved source preferences with independently persisted collection availability, without affecting shelf reading.
- **Storage:** `system.db` plus one self-contained reader home per immutable Reader Account ID.
- **Schema policy:** pre-public disposable data may be recreated; do not add migration machinery without a real compatibility requirement.
- **Source interaction:** reader-owned, source-ID-bound state; removing a source deterministically removes its owned state.
- **Browser runtime:** private bounded Patchright worker behind a versioned backend-owned interface.
- **Errors:** explicit typed failures; never convert parser/transport failures into successful empty results.
- **Source health:** user-triggered diagnostics only; no continuous broad monitoring.
- **Deployment:** production Compose pulls the published app/worker pair; local Compose builds the checkout; E2E Compose remains a separate deterministic verification contract. All use one server owner per writable data root.

Cross-cutting rationale:

- [Local accounts with self-contained reader directories](docs/decisions/0001-user-owned-data-and-local-authentication.md)
- [Reading documents, resources, and modality renderers](docs/decisions/0002-reading-documents-and-resources.md)

## Constraints

Unless separately approved:

- no captcha solving, automatic WAF bypass, or unrestricted browser automation;
- no source-specific production branches merely to make one fixture pass;
- no frontend crawling or source-rule interpretation;
- no distributed writable data root or multi-instance session coordination;
- no continuous source-health polling;
- no speculative shared abstraction or global frontend state for feature-local workflows;
- no media-domain expansion without a scoped model and implementation plan.

## Verification Policy

Match verification to risk and run the smallest authoritative gate first.

For BookSource execution changes:

1. identify the upstream behavior and shared seam;
2. add a deterministic regression through the nearest production interface;
3. implement the smallest shared fix;
4. run focused package tests, then broaden only for real coupling risk;
5. classify live failures as transport, upstream/WAF/DNS, session, analyzer, workflow, storage, or frontend.

For frontend changes, run focused component tests, TypeScript checking, and the production build; use visible browser verification when layout or interaction behavior changes.

For deployment changes, validate the affected Compose contract and report unavailable Docker/runtime verification explicitly.

## Documentation Route

- `README.md` — setup, operation, testing, and deployment.
- `PRODUCT.md` — product purpose and interaction principles.
- `PLAN.md` — current project state, priorities, and routing.
- `docs/architecture/` — current subsystem behavior.
- `docs/decisions/` — durable cross-cutting rationale.
- `docs/plans/` — substantial workstream handoff; completed plans are frozen history.
- `docs/roadmaps/` — unresolved future direction.
- `docs/reference/` — stable terminology and reference material.
- `docs/research/` — durable investigations.
- `docs/runbooks/` — operational procedures.
- `docs/verification/` — dated evidence.
- `docs/archive/` — non-authoritative historical audits, designs, logs, plans, and research.
