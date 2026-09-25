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
- `backend/internal/library/` — shared publication metadata, reading state, bookmarks, and revision contracts.
- `backend/internal/reading/` — common catalogs/prose documents and revision-qualified reading operations over BookSource/TXT/EPUB.
- `backend/internal/txtstore/` — managed TXT originals/indexes and acquisition/removal/recovery.
- `backend/internal/fileimport/` — shared bounded TXT/EPUB admission, preparation scheduling and recovery lifecycle.
- `backend/internal/epubstore/` — EPUB receipts, generation-scoped prepared streams/resources and portable validation.
- `backend/internal/book/` — BookSource Search, Explore, Book Info, native catalogs/content, bindings, and cache.
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

[Native ARM64 container releases](docs/plans/2026-09-26-arm64-containers.md) — implemented and verified.
Native AMD64/ARM64 builds, both browser modes, Compose E2E and multi-platform registry checks passed
in verification-only run 36176447907. Production run [36180574302](https://github.com/OtwakO/NovelReader/actions/runs/36180574302)
succeeded after retrying an interrupted Chrome download; both published `latest` image indexes were
verified to include `linux/amd64` and `linux/arm64`.

The multi-provider branch is integrated into local `main` with a history-preserving merge after [clean-checkout integration checks](docs/notes/2026-09-21-import-user-testing-and-branch-review.md#local-branch-integration). That integration has not been pushed or deployed. The cache/prefetch workstream is implemented, locally verified, and merged into local `main`; it has not been pushed or deployed.

Reader schema is **16**, adding EPUB inbox claims with portable cleanup-authority stripping. Existing epoch-15 and older homes/backups remain preserved, not migrated. [Independent last-read tracking](docs/plans/2026-09-17-last-read-tracking.md), introduced at epoch 13, continues to separate reading from additions/metadata updates.

### Complete product foundations

- Local Reader Accounts, setup, registration policy, recovery, password management, administration, and durable deletion.
- Per-reader storage with isolated `reader.db`, files, source profile state, and encrypted source credentials.
- Portable Reader Data backup/restore with scoped automation tokens and staged atomic replacement.
- Lossless BookSource import/export, Source Collections with independently persisted Search/Explore availability, manual/scheduled collection synchronization, and duplicate source definitions.
- Shared HTTP/fingerprint/typed-data/WebView execution with bounded source sessions and process capacity.
- Batched streaming Search and strict single-source Explore.
- Metadata-first shelf admission: bounded Book Info selects a source; catalog synchronization is separate, single-flight, cached in SQLite, observable, retryable, and atomically published.
- Logical-book identity by normalized title/author with exact `(SourceID, BookURL)` source bindings, unified source recovery, and atomic source switching.
- TXT import, custom-pattern review and explicit published-book reparse with conservative location preservation.
- Reader progress and bookmarks, revision-coherent navigation, bounded session chapter reuse, default-on next-chapter prefetch, ordered non-blocking progress saves, Chinese conversion reuse, explicit Refresh, and source-switch invalidation.
- Browser-local typography/image/wake-lock preferences, responsive reader controls, keyboard navigation, TOC filtering/ordering/current positioning, and shelf filtering/restoration.
- Compact source-management summaries with on-demand lossless editing, plus shared full-cover presentation and reader-scoped seven-day cover caching.
- Reader-owned source interaction, settings, login state, controlled browser sessions, and bounded `startBrowserAwait` continuation replay.
- Installable Vue PWA; production GHCR Compose, checkout-built local Compose with bind-mounted data, and separate deterministic Compose E2E.
- Worker-wide Chrome headless/headful selection and paired app/worker release publication with exact-image verification.

The completed [reader cache and prefetch](docs/plans/2026-09-22-reader-cache-and-prefetch.md) workstream adds backend cache-first/Refresh, immutable image resources, persistent client caching, transactional invalidation, two-target conversion preparation/renewal and destination-specific navigation feedback. Focused tests and synthetic browser checks pass. [Live reader re-entry measurements](docs/notes/2026-09-26-reader-reentry-latency.md) confirm memory/IndexedDB chapter hits but identify blocking catalog/metadata requests and repeated conversion; the note proposes improvements without weakening invalidation, TTL or retention. No follow-up implementation is accepted yet; upstream crawl latency remains unmeasured.

### Compatibility position

The major text-reading journeys are operational. Remaining compatibility work is incremental and evidence-driven, concentrated in shared analyzer, JavaScript bridge, session, request, source-interaction, and media-specific semantics. NovelReader does not claim universal parity with every Android/JVM-only Legado behavior.

Use [Legado compatibility roadmap](docs/roadmaps/legado-compatibility.md) for unresolved capability families and [archived compatibility tracker](docs/archive/audits/legado-compatibility-tracker-2026-08.md) for the completed 2026-08 audit queue.

### Completed workstream handoffs

[Verified project-review corrections](docs/plans/2026-09-25-project-review-corrections.md) — five reproduced defects corrected, test-only search merger removed, and approved ReaderView formatting completed and fast-forwarded into local `main`. Full Go tests/vet, frontend regression, focused race tests and builds pass. Not pushed or deployed; the plan records evidence and verification limits.

[EPUB support](docs/plans/2026-09-17-epub-support.md) — completed bounded reflowable novel-reading milestone at epoch 16. Browser/inbox intake, review, shared reading and image resources, progress/bookmarks, portable lifecycle and removal are integrated without a second reader or scheduler. The final isolated Chromium journey covered inbox import → reading/bookmark → export → removal → restore → restart → reading/resources → removal. Existing data was untouched; deployment, broad real-book compatibility and stress/power-loss testing are not claimed.

[Parallel release builds](docs/plans/parallel-release-builds.md) — concurrent production builds are
merged and verified. The first release passed in 5m21s with a cached app layer; source-change timing
and comparison limits are recorded in the plan. All verification and publication gates remain.

[GitHub Actions runtime](docs/plans/github-actions-runtime.md) records the initial conservative
optimizations and successful hosted verification (7m42s → 6m35s in the first comparison).

[Reader navigation performance](docs/plans/reader-navigation-performance.md) records the navigation/cache/conversion lifecycle and controlled timing evidence. Live-source timing remains unverified.

The completed [WebView Runtime Efficiency](docs/plans/2026-09-04-webview-runtime-efficiency.md) workstream provides worker-wide Chrome headless/headful selection, leaner packaging, and latest-stable release-image verification while preserving per-request reader isolation and the bounded WebView security seam. Further live compatibility and footprint measurements remain optional, evidence-gated follow-up.

The completed [Source Authentication and Session Foundation](docs/plans/2026-09-03-source-auth-session-foundation.md) work established reader-owned login/session state, scoped runtime-cookie management, secret-safe diagnostics, and bounded authenticated controlled-browser networking. The completed [Source Collection availability](docs/plans/2026-09-02-source-collection-availability.md) work added a collection-level Search/Explore gate while preserving every member source's individual settings and existing shelf reading. The completed [reading document foundation](docs/plans/2026-09-02-reading-document-foundation.md) established the versioned prose-document, opaque-resource, and focused prose-renderer seams around the current BookSource text/image path.

The completed [architecture and code quality improvements](docs/plans/2026-09-05-architecture-code-quality-improvements.md) workstream corrected lifecycle/isolation defects, upload/font/identity contracts and reader-handler ownership, and implemented measured narrow chapter/progress lookups. That checkpoint introduced reader schema epoch 9; subsequent shared-library, TXT interpretation, last-read tracking and EPUB work advance the current epoch to 16. Its plan records scoped verification and the approved local-only integration; hosted CI and deployment verification remain unperformed. Frontend decomposition stays evidence-gated.

The completed [multi-provider library and imported books](docs/plans/2026-09-10-multi-provider-library.md)
workstream delivers TXT browser/inbox intake, bounded review and explicit admission, shared reading,
portable lifecycle/cleanup, custom patterns and safe published reparse. The epoch-12 model retains one
original and active/candidate indexes, with no migration layer. Backend normal/race, scoped frontend
and fresh real-server checks cover the recorded workflows, including stale-reader protection. Hosted
CI/deployment and high-load throughput remain unverified.

Post-review [restore outcome recovery and TXT failure guidance](docs/plans/2026-09-15-restore-outcome-and-txt-errors.md)
are also implemented and verified. The initiating tab retires old work before restoration and recovers
uncertain outcomes without replay; analysis errors now provide safe, specific guidance.

The [automatic TXT import workflow](docs/plans/2026-09-15-simple-txt-import.md) now lives on a dedicated
**Local import / 本地匯入** page, reached by one shelf action. The completed
[frontend presentation consistency pass](docs/plans/2026-09-15-frontend-presentation-consistency.md)
unifies typography, disclosures and actions across the app while preserving other workflows.
Shared UI ownership is documented in [frontend/src/ui/README.md](frontend/src/ui/README.md).
The completed [import layout and selector refinement](docs/plans/2026-09-15-import-layout-and-selectors.md)
adds distinct review/inbox task panes and fixes shared selector widths, truncation and viewport placement.
The completed [action-affordance pass](docs/plans/2026-09-16-action-affordances.md) unifies button presentation,
aligns book-detail/reparse controls, fills the desktop prose preview, and updates locale/brand presentation.
The [reader-first presentation pass](docs/plans/2026-09-16-reader-first-presentation.md) is complete: clearer typography/navigation, reader-first Settings and task guidance, retaining current fonts and parchment colors.
[Import history and review](docs/plans/2026-09-16-import-history-and-review.md) fixes hidden/stale persisted imports and adds default-on review before shelf admission, shared with device-local Settings. Retention remains explicit discard/removal; selected-chapter TXT previews are delivered by the unified preview work below.

[EPUB import preview and image preference](docs/plans/2026-09-24-epub-import-preview.md) — completed selected-section preview with authored contents, prose/images and explicit import authorization; both intake controls share one saved device-local image preference. Scoped backend normal/race tests, 59 frontend tests, typecheck/build and isolated desktop/mobile Chromium verification pass. No schema change, second reader or deployment.

[Mixed-format import history and status filters](docs/plans/2026-09-24-import-history-filters.md) — completed U3/U4: default All, newest-first pagination and shared lifecycle filters, including Needs review for both formats. Verified with affected backend packages, import UI tests, typecheck/build and an isolated synthetic browser journey. No schema change; existing provider endpoints remain compatible.

[Unified import preview](docs/plans/2026-09-25-unified-import-preview.md) — implemented B: visible 250px desktop contents, persistent title/author and arrow navigation. TXT import/re-analysis show whole selected chapters; EPUB retains its loaders and all prose images are centered in preview/reader. Focused backend/frontend and isolated desktop/mobile checks passed; no schema or deployment change.

Continue Reading's narrow-screen reflow (`947a2b8`) and the bookshelf cover-gallery layout (`aed9220`) are implemented in `ShelfView.vue`. Their disposable prototypes, along with the completed import-preview prototypes, have been removed; Git history retains the experiments.

[Import user-testing issues and branch review](docs/notes/2026-09-21-import-user-testing-and-branch-review.md) — U1–U4 and R1–R3 are resolved: preview/history improvements, retained inbox refresh notifications, behavior-preserving reader readability, and corrected test fixtures. Final reader/import verification passed 126 tests without warnings, typecheck and production build. The note retains original findings and scoped verification limits.

## Active Work

[BookSource engine compatibility audit](docs/plans/booksource-engine-compatibility-audit.md) — independent shared-engine review anchored in a frozen private 50-source Search/Book Info sample and upstream rule/reference comparisons. Confirmed E01–E05 corrections, the browser-owned UA provider and lifecycle hardening were locally integration-tested, merged and pushed to `main` at `759391e`. No implementation remains unfinished in that checkpoint; unresolved compatibility investigations and release-verification limits remain in the plan. No source-specific patches or real BookSources committed.

## Immediate Priorities

1. Gather manual usability and real-book compatibility feedback for the completed [EPUB milestone](docs/plans/2026-09-17-epub-support.md). Scope any resulting defects separately; preserve epoch-15 and older data rather than migrating or resetting it implicitly.
2. Keep future work proportional to its risk: reuse the established storage, reading and lifecycle owners, use focused verification, and avoid speculative frameworks. The completed TXT plan is historical evidence, not an active backlog.
3. The accepted [BookSource engine corrections](docs/plans/booksource-engine-compatibility-audit.md), bounded browser-UA provider and [browser lifecycle hardening](docs/plans/browser-worker-lifecycle.md) are implemented. Confirm any next compatibility slice with the user before implementation; retain the recorded verification limits. Do not claim universal compatibility.
4. Select further compatibility slices from current evidence rather than historical unchecked boxes; introduce image-sequence documents and structured locations only when that modality becomes active work.
5. Consider still-relevant Reader UX opportunities only after explicit approval; see [Reader UX roadmap](docs/roadmaps/reader-ux.md). Finish consistent display of source-provided `updateTime` metadata only if that presentation improvement is prioritized.

## Durable Decisions

- **Compatibility:** match documented and observed Legado behavior at shared seams before considering source-specific behavior.
- **Frontend seam:** Vue consumes typed domain interfaces and never executes BookSource rules or interprets opaque source payloads.
- **Reading seam:** providers open Reading Sections as modality-specific Reading Documents; documents use opaque Content Resources and the Reading Session delegates to modality renderers. See [decision 0002](docs/decisions/0002-reading-documents-and-resources.md).
- **Source identity:** immutable NovelReader Source ID; imported `bookSourceUrl` is source data and may duplicate.
- **BookSource book identity:** normalized title plus author identifies a logical BookSource shelf book; exact source bindings live beneath it. Imported publications remain independently identified shelf items by default; cross-provider edition linking requires a separate decision.
- **Shelf admission:** Book Info metadata is sufficient for admission; catalog availability is a separate observable state.
- **Explore:** one selected BookSource and its native catalog at a time; Search/Explore eligibility combines saved source preferences with independently persisted collection availability, without affecting shelf reading.
- **Storage:** `system.db` plus one self-contained reader home per immutable Reader Account ID. File-backed publications use rooted relative paths under that home; portable backup and deletion coordinate database and durable-file generations without provider-specific backup systems or silent orphaned bytes.
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
