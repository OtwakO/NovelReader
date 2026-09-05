---
status: active
updated: 2026-09-05
---

# Architecture and Code Quality Improvements

## Goal

Resolve the verified architecture and code-quality issues from the 2026-09-05 repository audit without changing product behavior, weakening compatibility, or introducing speculative abstractions.

Done means the confirmed violations are fixed, performance hypotheses are measured before redesign, obsolete interfaces are removed only after caller verification, and affected tests and documentation accurately describe the resulting interfaces.

## Scope

Included:

- restore the declared frontend lint standard and enforce it in CI;
- reduce responsibility concentration in the Reader and Source Management frontend modules;
- deepen backend workflow and HTTP modules where current interfaces expose too much unrelated state;
- consolidate duplicated authenticated-session deadline handling;
- make scheduled Source Collection failures observable and bounded;
- measure the SourceSession registry and JavaScript runtime concerns before optimizing them;
- remove individually confirmed obsolete internal interfaces;
- simplify high-complexity functions only where cohesive phases can be extracted without changing behavior.

Excluded unless separately approved:

- new product behavior or UX redesign;
- a generic provider architecture before a second provider exists;
- replacement of the existing BookSource, transport, per-reader runtime, or WebView seams;
- bulk deletion based on static dead-code output;
- bulk formatting or deduplication of frozen audit scripts;
- parser rewrites without deterministic compatibility evidence;
- performance redesign without measurements showing a practical problem.

## Accepted Approach

Work in small, independently verified phases. Correct objective standard violations first, then make localized cohesion improvements, then measure performance hypotheses, and only then consider broader backend module restructuring.

For every phase:

1. preserve existing observable behavior with focused tests;
2. change the smallest interface that removes the verified problem;
3. avoid pass-through layers and one-adapter hypothetical seams;
4. run focused verification before broader tests;
5. update this plan's Current State, Next Action, and Verification at meaningful milestones.

## Decisions

### Separate confirmed defects from design risks and measurement hypotheses

**Decision:** Track each finding with a verification classification rather than treating every static-analysis result as a defect.

**Why:** Complexity, file size, dead-code analysis, and asymptotic cost are useful signals but do not prove incorrect behavior or user-visible performance. The repository standard requires fixing causes rather than optimizing or abstracting speculatively.

**Alternatives:** Treat all audit output as a cleanup queue; ignore static analysis entirely.

**Revisit when:** Better profiling or caller analysis changes a finding's classification.

### Preserve real seams

**Decision:** Keep the current BookSource-only provider model, `sourceexec.Transport` seam, per-reader runtime lifecycle, generation-based frontend stale-work protection, and private WebView worker architecture.

**Why:** Each currently solves a demonstrated problem. Replacing them would expand scope without evidence that their core design is wrong.

**Alternatives:** Introduce a generic provider framework; collapse browser execution into the app process; replace the runtime lifecycle during unrelated cleanup.

**Revisit when:** A second provider, deployment requirement, or measured runtime failure creates a real new case.

### Refactor by cohesive behavior, not size alone

**Decision:** Large modules are candidates for improvement only where they contain independently changing responsibilities. Extract deep modules or focused views, not thin wrappers.

**Why:** Line count and cyclomatic complexity are signals. Arbitrary splitting would make navigation and interfaces worse.

**Alternatives:** Enforce a mechanical line limit; leave all large modules unchanged.

**Revisit when:** A proposed extraction cannot demonstrate a smaller interface, better locality, or clearer ownership.

## Verified Findings

### F01 — Frontend lint contract is broken and absent from CI

**Classification:** Confirmed defect; first priority.

**Evidence:**

- `npm run lint` fails with 7 errors and 3 warnings.
- Violations are in `ProseRenderer.vue`, `ReaderActionsMenu.vue`, `ReaderSettingsSheet.vue`, `ReaderView.vue`, and `SourceBrowserSession.vue`.
- `.github/workflows/publish.yml` runs frontend tests and build but not `npm run lint`.
- `frontend/eslint.config.js` explicitly requires Options API and the reported component ordering/style rules.

**Recommendation:** Decide the canonical Vue interface from the existing explicit configuration, fix all current violations, and add lint to the frontend CI gate. Current evidence supports retaining Options API because it is already the declared project-wide convention.

### F02 — ReaderView owns too many independent responsibilities

**Classification:** Confirmed maintainability issue; behavior currently passes tests.

**Evidence:** `frontend/src/features/reader/ReaderView.vue` coordinates route state, chapter load/prefetch, source switching, progress, bookmarks, Chinese conversion, font loading, wake lock, keyboard/tap navigation, scroll restoration, error recovery, sheets, and document title. Its apparent 201 lines contain unusually dense multi-operation lines and a large mutable state surface.

**Recommendation:** Extract cohesive plain-TypeScript reader workflow modules, beginning with chapter navigation/loading state. Keep DOM and presentation state in the Options API view. Do not introduce generic composables or a speculative global reader store.

### F03 — SourceManagementView contains multiple real subfeatures

**Classification:** Confirmed maintainability issue; behavior currently passes tests.

**Evidence:** `frontend/src/features/sources/SourceManagementView.vue` is 1,081 lines and owns Source Collections, standalone import, installed-source editing/deletion, interaction UI, filtering, pagination, statistics, and many unrelated busy/error/modal states.

**Recommendation:** Split by existing user operation into collection, import, and installed-source panels with a small coordinating parent. Extract only genuinely shared filtering/statistics logic; do not add generic CRUD infrastructure.

### F04 — `book.Searcher` has outgrown its search-named interface

**Classification:** Confirmed cohesion and naming issue; structural refactor deferred until smaller phases are complete.

**Evidence:** `backend/internal/book/search.go` is 1,279 lines. `Searcher` owns Search, Book Info, TOC, content, pagination, source sessions, Explore state, transport creation, source hydration, admission limits, and rate limiting. `GetChapterContentForBookContext` has measured cyclomatic complexity 41.

**Recommendation:** Design focused Search, Book Info, TOC, and Content workflow modules around one shared internal execution context. Avoid pass-through services and retain a temporary facade only if migration requires it.

### F05 — API `Server` is a broad dependency container

**Classification:** Confirmed cohesion/interface issue; structural refactor deferred.

**Evidence:** `backend/internal/api/server.go` is 1,071 lines. `Server` holds roughly 27 dependencies and registers nearly the whole product HTTP surface. `NewAuthenticatedServer` takes 11 substantial parameters. Handler files are already feature-separated, but every handler method depends on the broad `Server` receiver.

**Recommendation:** Move feature route registration and focused dependencies into route modules while retaining one top-level composition and lifecycle owner. Do not create an interface for every concrete store.

### F06 — SourceSession registry performs full scans on routine access

**Classification:** Confirmed algorithmic risk; practical performance impact unmeasured.

**Evidence:** `backend/internal/sourceexec/session_registry.go` calls `evictLocked` on routine reads and writes. Expiry checks scan all sessions; capacity eviction scans for the oldest session; removing a session scans all book and chapter aliases. Limits can reach thousands of sessions.

**Recommendation:** Benchmark representative registry sizes before redesign. If material, first try periodic or operation-count-based sweeps plus reverse alias ownership. Add an LRU/heap only if simpler changes are insufficient.

### F07 — JavaScript “pool” recreates a Goja runtime after every evaluation

**Classification:** Confirmed implementation characteristic; performance impact unmeasured and isolation is intentional.

**Evidence:** `backend/internal/analyzer/js.go` borrows a runtime in `EvalContext` but returns `vm.newRuntime()` rather than the used runtime. Each evaluation rebuilds compatibility bindings. `EvalContext` has measured cyclomatic complexity 42. Comments explicitly justify fresh-runtime isolation.

**Recommendation:** Benchmark evaluation allocation and duration. Preserve state isolation. Prefer caching immutable bootstrap material or clarifying concurrency-admission ownership before attempting runtime reuse.

### F08 — Authenticated-session deadline handling is duplicated

**Classification:** Confirmed production duplication and consistency risk.

**Evidence:** Equivalent late-session revocation logic appears in `backend/internal/auth/http.go`, `recovery_http.go`, and `setup_http.go`. AFT reports a 70-line duplication group across the three paths.

**Recommendation:** Extract one focused internal operation for session creation before a deadline, including the post-create callback and revocation of late results. Replace the three implementations and test the shared contract once plus focused handler integration.

### F09 — Scheduled Source Collection failures are partly swallowed

**Classification:** Confirmed observability and error-handling defect; concurrency improvement not yet justified.

**Evidence:** `backend/internal/api/source_collection_scheduler.go` silently skips runtime acquisition failures, ignores `ListDueCollections` failures, and discards errors from failure/success recording. Work runs from `context.Background()` without a scheduler-owned per-sync deadline.

**Recommendation:** Log actionable failures without secrets, apply explicit operation deadlines, and preserve sequential processing until timing evidence demonstrates a need for bounded concurrency.

### F10 — Some internal convenience interfaces are obsolete, but the original broad dead-code claim was overstated

**Classification:** Partially confirmed; requires per-symbol cleanup.

**Evidence:** Structural caller analysis finds no callers for `Searcher.Search` or `Executor.Build`. In contrast, legacy content methods still have test and conformance callers, and `NewServer` has extensive test use. AFT's reported 873 dead-code items include false positives from dynamic JavaScript exposure, method dispatch, build variants, tests, and Vue templates.

**Recommendation:** Remove only individually verified caller-free methods. Migrate tests from legacy content interfaces before removal where the current interface better represents the contract. Never use the static dead-code count as a bulk deletion list.

### F11 — Several complex functions mix separable phases

**Classification:** Confirmed maintainability signal; not every high-complexity parser is defective.

**Evidence:** AFT reports 315 functions with cyclomatic complexity at least 10. Highest values include `EvalContext` (42), `GetChapterContentForBookContext` (41), `BuildURLWithContextData` (39), `completeClaim` (33), `handleAddBookmark` (32), and `ParseChapterList` (32). Manual review confirms several combine orchestration, validation, I/O, and transformation phases.

**Recommendation:** Address complexity while working on the owning verified issue. Extract cohesive phases with observable contracts; do not create a standalone complexity-reduction sweep.

### F12 — URL page-selector support records a partial grammar unclearly

**Classification:** Confirmed documentation/compatibility limitation; no current regression established.

**Evidence:** `backend/internal/analyzer/urlbuilder.go` states that page-selector expansion handles only a common case and defers complex JavaScript, using the unclear label `ponytail`.

**Recommendation:** Replace the comment with a precise supported/unsupported grammar when this file is next changed. Extend behavior only from deterministic compatibility evidence.

### F13 — Production duplication is limited; versioned audit duplication should remain historical

**Classification:** Confirmed, with narrowed action.

**Evidence:** AFT reports 754 duplicated lines, 1.2% of analyzed lines. Most are in versioned audit scripts whose immutability supports reproducibility. Actionable production duplication includes authenticated-session handling and quoted-string parsing in `lenientmap.go` and `rulevars.go`.

**Recommendation:** Consolidate duplicated production contracts only when a shared implementation improves locality. Do not rewrite frozen audit versions solely to reduce a metric.

### F14 — Dense one-line Vue source harms review and diagnostics

**Classification:** Confirmed code-quality issue; avoid a repository-wide formatting change.

**Evidence:** `ReaderView.vue` is about 31 KB in only 201 physical lines, with large methods, template sections, and style declarations compressed horizontally. Similar density contributes directly to current lint failures and weak line-level diagnostics.

**Recommendation:** Reformat only modules touched during focused refactors, in the same logical change when practical. Avoid unrelated bulk formatting that obscures history.

## Findings Not Accepted as Current Defects

- No import cycles were found.
- No evidence currently justifies a generic provider abstraction.
- `sourceexec.Transport` has multiple real adapters and remains a justified seam.
- Per-reader runtime caching and quiescence have real lifecycle responsibilities and tests.
- Generation-based stale-work prevention in the reader is useful even though its ownership should become more cohesive.
- The separate bounded WebView worker remains justified.
- The compatibility test volume is substantial but corresponds to distinct parser, transport, session, persistence, security, and workflow behavior. No systemic over-testing or fixture overfitting was established.
- Versioned audit-script duplication is not accepted as cleanup work by default.

## Progress

- [x] Complete initial repository-wide architecture and implementation audit.
- [x] Reverify findings against code, CI configuration, structural callers, lint, diagnostics, and architecture documents.
- [x] Classify findings as defects, maintainability issues, measurement hypotheses, partial findings, or rejected static-analysis noise.
- [x] Record the accepted improvement approach and active workstream.
- [x] Restore and enforce the frontend lint contract.
- [ ] Improve Reader frontend cohesion.
- [ ] Improve Source Management frontend cohesion.
- [x] Consolidate authenticated-session deadline handling.
- [ ] Improve scheduled Source Collection error handling and deadlines.
- [ ] Remove individually verified obsolete interfaces.
- [ ] Add and run SourceSession registry benchmarks; optimize only if justified.
- [ ] Add and run JavaScript evaluation benchmarks; optimize only if justified.
- [ ] Design and implement the accepted backend workflow/route deepening in separately reviewable phases.

## Current State

The audit is verified and documented. The branch is `review/architecture-code-audit`.

F01 is implemented: the five violating Vue components now follow the declared Options API and ordering/style rules, and the publish workflow runs lint before frontend tests and build. The conversions preserved local component ownership and did not introduce shared abstractions or alter product behavior.

F08 is implemented: login, registration, setup, and recovery now call one package-local authenticated-session deadline operation. Late-session revocation, retry bounds, callback timing, and flow-specific diagnostics have one owner in `backend/internal/auth/session_deadline.go`; handler-specific pass-through methods were removed.

Broader frontend and backend refactors remain tracked and evidence-gated.

## Next Action

Review and commit the completed F08 phase. Then address F09 as a localized reliability improvement: define scheduler operation deadlines and make acquisition, listing, sync-recording, and shutdown failures observable without adding concurrency. F02 and F03 are larger design changes and should be discussed before choosing component boundaries.

## Verification

Verified on 2026-09-05:

- `go test ./...` passed for all backend packages.
- `npm test` passed: 54 files and 182 tests.
- `npm run build` passed, including TypeScript checking.
- WebView worker targeted unit tests passed: 36 tests.
- `npm run lint` reproducibly fails with 7 errors and 3 warnings.
- `.github/workflows/publish.yml` omits lint from frontend verification.
- AFT reports no import cycles and 1.2% duplicated analyzed lines.
- AFT diagnostics report no TypeScript, Vue, Python, or YAML diagnostics; Go LSP was unavailable, so `go test ./...` is the authoritative Go compile gate.
- Structural caller checks distinguish caller-free `Searcher.Search` and `Executor.Build` from legacy/test-used interfaces.
- Git working tree was clean before documentation changes.

F01 verification completed on 2026-09-05:

- `npm run lint` passed with zero warnings or errors.
- Affected Reader and Source Browser component tests passed.
- Full frontend Vitest run passed: 53 test files.
- `npm run build` passed, including `vue-tsc --noEmit` and the Vite production build.
- `git diff --check` passed.

F08 verification completed on 2026-09-05:

- `go test ./internal/auth` passed.
- `go test -race ./internal/auth` passed.
- The existing login, recovery, and setup late-session tests exercise the shared operation, including deadline return and retry after transient session-guard contention.
- AFT no longer reports the three-handler authenticated-session duplication group.
- `git diff --check` passed.

Still needed:

- focused verification for each remaining implementation phase;
- benchmarks before accepting F06 or F07 as performance defects;
- visible browser checks when frontend layout or interaction changes;
- final broad regression verification before completing the workstream.
