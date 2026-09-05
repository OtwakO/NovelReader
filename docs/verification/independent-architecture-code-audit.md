# Independent architecture and code audit

**Reviewed revision:** `33b9a1f` on `review/architecture-code-audit`  
**Branch baseline:** `4456b1bf5f7117288fcedc01025894ae0af6f9ff` (merge-base with `main`)  
**Status:** Review complete; fixes and implementation order require approval.

## Scope and method

Independent, single-agent review against `AGENTS.md`. The preceding audit was a hypothesis source, not evidence. Reviewed branch diffs plus the current request/authentication boundary, reader-home/runtime ownership, discovery/candidate/catalog/reading flows, source execution and interaction, browser worker, source and font persistence, backup/restore, frontend state and tests, and CI wiring. This is a cross-system risk-based review, not a claim that every compatibility rule or test was exhaustively inspected.

Findings below distinguish reproduced defects, source-traced defects, maintainability concerns, and unmeasured performance hypotheses. No production implementation was changed. Temporary synthetic regression probes were removed after running them; no private BookSource data was used.

## Summary

The top-level architecture is reasonable: reader-scoped storage, backend-only BookSource execution, transport adapters, a private browser worker, and typed frontend boundaries earn their complexity. A wholesale rewrite, provider framework, global event bus, or new general-purpose repository layer is not justified.

The main weakness is **lifecycle ownership spread across cooperating objects**: account state, candidate transitions, browser sessions, runtime leases, and collection cleanup each depend on ordering rules that their interfaces do not enforce. Fix these contracts before cosmetic file splitting. Large files are secondary evidence, not the reason by themselves to refactor.

Priority: **P1** = correctness/isolation/resource-ownership issue deserving early correction; **P2** = concrete boundary or maintainability issue; **P3** = measured follow-up/cleanup. P1 here does not imply a demonstrated remote exploit.

## Confirmed correctness and boundary findings

### A01 — P1: Frontend reader-owned state survives account changes

**Evidence:** `frontend/src/stores/session.ts:102-142`; `features/search/search-store.ts:10-30,53-80`; `features/explore/explore-store.ts:5-16`; `features/candidates/candidate-operation.ts:85-116,181-193`; `features/search/candidate-selection.ts:3-11` (frontend paths relative to `frontend/src/`).

Logout changes authentication state but neither disposes discovery stores/streams nor clears their unscoped session-storage keys and module-level remembered candidate commits. The same application/Pinia instance survives login as a different reader. Search/Explore results, query history, candidate selections and remembered shelf-book IDs therefore remain associated with the previous reader. Candidate commit keys use source URL/book URL/title/author, not Reader Account ID.

**Reproduced:** Set a search query, persist it, remember a candidate commit, then call the real session-store logout action with a stubbed successful HTTP logout. The query and serialized session remained; an equivalent candidate with a different installed Source ID still returned `candidateWasCommitted() === true`.

**Impact:** Client-side cross-account information exposure and incorrect shelf/navigation state in a shared browser. Backend ownership checks are not thereby bypassed; this is a distinct frontend isolation failure.

**Recommendation:** Own identity transitions at one application boundary. Dispose reader-owned streams/pending operations and clear or account-scope discovery/candidate state on logout and identity replacement. Keep intentionally browser-owned appearance preferences separate. Add one A→logout→B regression rather than a reset test for every helper.

### A02 — P1: Cancelled/expired candidate state does not prevent shelving

**Evidence:** `backend/internal/candidate/manager.go:259-299`; `operation.go:174-207,211-230`.

`Cancel` marks a verified operation cancelled and releases its runtime, but leaves `op.resolved` present. `commit` checks only for a cached commit or a non-nil resolved value; it does not require an admissible operation state. Expiry also retains the resolved value. Commit, cancellation, and expiry do not share one transition lock.

**Reproduced:** Start → wait for verified and operation completion → cancel → commit. The runtime-release callback ran, yet commit succeeded with `state=committed` and one shelf write.

**Recommendation:** Enforce legal transitions in the operation, using the same serialization for cancel/expiry/commit. Only verified or explicitly retryable commit-failure states may begin a new write; terminal cancellation/expiry must reject it. Preserve idempotent reads of an already completed commit. Tie lease release to that state machine instead of independent flags. Test cancellation and one competing transition, not all state combinations.

**Important non-finding:** Verified candidates already have an `expirePending` timer. The defect is not an absence of pending-operation expiry.

### A03 — P1: Browser-worker failure cleanup releases capacity twice

**Evidence:** `webview-worker/interactive.py:51-75,126-150`; `webview-worker/browser.py:168-179`.

`InteractiveSessions.create` registers ownership and calls `frame`. If the initial screenshot fails, `create`'s inner exception handler calls `close`, which removes the session, closes the context and releases browser capacity. Its outer exception handler then observes that the session is no longer registered and closes/releases again. `BrowserWorker._release_interactive` releases the semaphore and decrements `active`; it is not idempotent.

**Reproduced:** A synthetic page whose first screenshot raises produced **two releases for one acquisition**.

**Impact:** Corrupted capacity accounting, negative active counts and admission beyond the configured browser-context limit; browser recycling decisions also depend on that count.

**Recommendation:** Explicitly transfer cleanup ownership once the session is registered. Exactly one owner must release each acquired browser slot. Test the initial-frame failure through session creation and assert one release; do not mask the bug by clamping counters.

### A04 — P2: Browser UI can create an orphan after unmount

**Evidence:** `frontend/src/features/sources/SourceBrowserSession.vue:29-40,56-88,138-157`.

`beforeUnmount` clears an existing timer/session only. An outstanding `startSourceBrowser` can resolve afterwards, store the returned frame/session, and schedule polling on the unmounted component. An in-flight frame request can similarly schedule another timer after teardown. The start API call has no abort signal in this flow, and the continuation has no disposed/generation guard.

**Reproduced:** Mount with a deferred start response → unmount → resolve the session. `closeSourceBrowser` was never called for the new session.

**Recommendation:** Give the interaction a single disposable lifetime. Reject stale continuations; if creation completes after disposal, explicitly close the returned session. Do not reschedule polling after disposal. Worker expiry remains a safety net, not normal ownership cleanup.

### A05 — P1: The scheduler's new deadlines are not end-to-end bounds

**Evidence:** `backend/internal/api/source_collection_scheduler.go:64-102,106-128`; `source_lifecycle.go:26-37`; `reader_runtime.go:68-86,148-169,228-238`; `backend/internal/booksource/store.go:58-74`; `backend/internal/booksource/collection.go:429-438`.

The new contexts reach reader listing, due-collection listing, HTTP loading and transaction work, but replacement first calls context-free `ListByCollection`, and success recording calls context-free `GetCollection`. Runtime acquisition also takes a plain mutex and can evict/close runtimes while holding it. Runtime close performs browser I/O and waits for catalog work. A timeout on the caller cannot interrupt these waits.

**Reproduced:** Hold the reader DB's sole connection, invoke collection replacement with an already cancelled context, and observe it remain blocked until the connection is released. The failure occurs before the context-aware replacement transaction.

**Impact:** A slow reader can still stall the single scheduler sweep and shutdown. Runtime cleanup under the global runtime-manager mutex can delay unrelated readers, not just scheduled work.

**Recommendation:** Propagate context through the actual storage calls. Remove external I/O and draining from global manager critical sections using explicit detach/closing ownership. Keep one scheduler loop; a pool of workers would hide, not fix, this cause. Add a cancellation-under-blocked-storage test and a cross-reader cleanup test. Describe each bound accurately.

### A06 — P2: Source mutation commits, profile cleanup and runtime invalidation disagree

**Evidence:** `backend/internal/api/source_lifecycle.go:11-65`; `source_collections.go:207-220`; `source_collection_scheduler.go:127`; `backend/internal/sourceprofile/store.go:143-200`; `backend/internal/booksource/collection.go:248-264`.

Manual mutation passes `closeSourceRuntime`, which invalidates source sessions and the interactive browser; scheduled mutation passes only `DeleteSourceSession`. Updating/removing a source through scheduled sync can leave the old interactive session active.

Separately, collection replacement commits definitions and success metadata before profile reconciliation. A reconciliation failure returns early, skipping runtime invalidation and reporting a failed operation after the persistent definition change already succeeded. Deletion has the same ordering problem. Credentials are deliberately in a separate database; this is not safely repaired by claiming a single ordinary transaction covers everything.

**Recommendation:** Put source-lifecycle ownership at the reader-runtime boundary and use it from manual and scheduled operations. Return affected Source IDs from committed mutations. Invalidate committed changes even if post-commit cleanup fails, and explicitly record/retry incomplete cross-database cleanup using the existing reconciliation mechanism. Distinguish committed mutation with cleanup failure from a rolled-back mutation. Avoid a generic lifecycle framework.

### A07 — P2: Some upload/update boundaries have no real body-size limit

**Evidence:** `backend/internal/api/server.go:501-514,973-999`.

Source replacement uses `io.ReadAll(r.Body)` without `MaxBytesReader`. Font upload uses `ParseMultipartForm(20 << 20)` followed by `io.ReadAll(file)`. The multipart argument is a memory threshold, **not an upload-size cap**; large files spill to temporary disk and are then read entirely into memory. The source-import endpoint already demonstrates bounded reading, so boundary policy is inconsistent.

**Recommendation:** Apply explicit request/file limits before parsing or reading, report oversized input consistently, and test one over-limit request at each distinct boundary. This needs no application-wide rate-limiting framework. Exposure is authenticated, but malformed local files can trigger the same resource problem.

### A08 — P2: Font replacement leaves durable orphan files

**Evidence:** `backend/internal/fontstore/store.go:55-75,116-127`; `backend/internal/readerstore/backup.go:43-50`.

Fonts have unique names. Add writes a new ID-named file, then `INSERT OR REPLACE` can remove the previous same-name database row without deleting its file. Repeated same-name uploads accumulate files that cannot be listed/deleted through the UI and are copied by backups. Delete also ignores a file-removal failure before deleting the row, losing the cleanup reference.

**Recommendation:** Make replacement explicit: keep the previous file reference until the new metadata commits, then retire it with visible/recoverable cleanup handling. Do not add a periodic whole-storage garbage collector as the first fix. One same-name replacement test plus a removal-failure case covers the real contract.

### A09 — P2: Logical-book normalization has drifted across the API seam

**Evidence:** `frontend/src/features/books/book-identity.ts:3-18`; `backend/internal/book/store.go:242-281`.

Both implement logical title/author identity, but frontend author-prefix removal allows whitespace before a colon while backend removal requires exact prefixes. For example, `Author : Someone` normalizes to `someone` in the frontend and `authorsomeone` in the backend. The frontend also uses locale-dependent lowercasing. Recovery matching and persisted shelf identity can consequently disagree.

**Recommendation:** Define the actual normalization contract, then run a small common set of synthetic contract vectors in both languages. Prefer authoritative backend identity where already available. Do not introduce code generation or a shared runtime just to avoid two small language-specific implementations. Discovery's intentionally looser grouping is a different contract and should not be conflated with shelf identity.

## Maintainability and performance findings

### A10 — P2: Request handling reconstructs the dependency graph and router

**Evidence:** `backend/internal/api/server.go:34-63,234-296`; `reader_runtime.go:92-109`.

Each authenticated API request shallow-copies the entire root Server, rewrites numerous dependencies, creates a new ServeMux and re-registers every feature route. Root-level and reader-specific fields coexist in one partially populated struct. The root Server and readerRuntime duplicate the knowledge of which feature dependencies belong to a reader.

**Cost:** Correctness depends on remembering which fields to replace whenever a feature is added; missing one can retain root or wrong-scope state. Route reconstruction is definite repeated work, but no material latency regression was measured.

**Recommendation:** Make reader-bound handler ownership explicit and bind stable routes once per appropriate lifetime. Keep the reader-scoping construction in one place rather than copying its field list into each request. Preserve the existing simple constructors for genuinely different test/production uses; do not add a DI container or generic service registry. Benchmark route/allocation cost before claiming performance wins.

### A11 — P2: Frontend decomposition has not captured workflow ownership

**Evidence:** `frontend/src/features/reader/ReaderView.vue:23-81,101-171`; `features/books/CandidateBookDetailView.vue:29-105`; `features/candidates/CandidateShelfAction.vue`; `features/sources/SourceManagementView.vue:65-113,201-410` (frontend paths relative to `frontend/src/`).

ReaderView coordinates navigation, loading, conversion, progress, source switching, restoration, polling and timers through many mutable flags/counters. Candidate view/action callers each own stream/reconnect/commit/recollection orchestration around a mostly transport/storage helper. SourceManagement has separate collection, import, editor and deletion workflows sharing broad busy/error state. Extracting presentation sheets has not made these lifetimes local.

**Recommendation:** Fix the demonstrated ownership bugs first, then extract complete feature-local workflows with explicit inputs, outputs and disposal. For example, a reading session should own navigation/load/progress generations together; a candidate operation client should own one subscription's lifetime. Do not split each boolean/timer into a helper or introduce a global store for every dialog. A 250-line target is not an acceptance criterion.

### A12 — P3: Full-catalog reads sit on narrow chapter/progress paths

**Evidence:** `backend/internal/api/server.go:700-724,817-833`; `backend/internal/book/store.go:694-701,810-820`.

Chapter content lookup and progress validation call `GetChapters`, materialize the entire TOC, then inspect one chapter. The work grows with catalog length for a single-record operation. The invariant only needs chapter existence/readability and, where relevant, a neighboring chapter.

**Recommendation:** Add a focused store query when tackling this path; retain full TOC loading for navigation/catalog display. Measure representative long catalogs before prioritizing further indexing, caching or virtualization. This is a source-proven unnecessary scan, not a measured user-visible performance regression.

Other hypotheses still need measurements: linear session-registry eviction, repeated search-result merge/sort/serialization, source-management filtering over the complete list. None justifies a speculative cache or worker system now.

## Branch commit review

- **`0a390e9` — frontend lint / Options API conversion:** Keep the lint command and CI gate. A concrete regression remains: the converted `SourceBrowserSession` template now relies on injected `$t`, but its existing test mounts without injection. The entire frontend suite fails. This is a test-contract regression, not proof the production app lacks i18n. Options API-only enforcement predated this commit; reversing that convention is a separate decision, not a fix demanded by this audit.
- **`1c219b4` — centralized auth-session deadlines:** Direction is sound. Three callers genuinely share late-created-session revocation semantics, so the helper removes real duplicated security-sensitive behavior without a generic framework. Reviewed caller/hook behavior and existing auth tests; no new defect established in this extraction.
- **`33b9a1f` — collection scheduler reliability:** Useful improvements: context-aware reader/due-source queries, visible persistence errors, a narrowly scoped due-collection query and bounded status recording. However, A05/A06 remain, and this commit adds no scheduler regression test. Do not call scheduled sync end-to-end bounded or the workstream complete yet.
- **`d773c8e` — audit/implementation plan:** Earlier prioritization overemphasized lint and large-file decomposition while missing lifecycle defects. Its blanket verified/completed claims and instruction to commit an already committed F09 are stale. The active plan now routes to this review rather than repeating those conclusions.

No reason was found to discard the branch wholesale. Correct the frontend test regression, retain the auth extraction, and complete the scheduler's real cancellation/lifecycle boundary instead of stacking more timeout wrappers.

## Tests, dead code, overfitting and review limits

- Existing backend command: `go test ./internal/auth ./internal/api ./internal/booksource ./internal/candidate ./internal/sourceinteraction ./internal/readerstore` — all six packages passed (cached results). This is **not** a full backend-suite run.
- `npm run lint` — passed. Focused frontend run: 21/22 passed, failing SourceBrowserSession. Expanded `npm test`: **181/182 tests passed across 54 files**, with that same failure. `npm run build` separately passed, including TypeScript checking.
- Temporary Go probes reproduced A02 and A05. Temporary frontend probes reproduced A01 and A04. An inline Python fake-page probe reproduced A03. These deliberately failed assertions of the desired contract; they were diagnostic probes, not new committed regressions.
- The existing tests have meaningful API/storage boundaries and synthetic fixtures; a large count is not evidence of over-testing. The more important problem is **coverage allocation**: happy-path/unit checks passed while ownership transitions failed, and no scheduler regression accompanied the timeout change. Prefer the small boundary tests recommended above rather than duplicating every private compatibility case.
- No independently substantiated production source-specific overfit was found in the inspected paths. Compatibility exceptions should still be justified against upstream behavior; this review did not audit every analyzer rule.
- No broad dead-code deletion is recommended. Automated AFT health inspection failed, and search/structural results were insufficient to prove repository-wide reachability. Do not turn dead-code hints or missing static callers into deletions. Small wrappers such as `rootSearcherFetcher` are shallow, but deleting one is low-value compared with correcting A10's ownership boundary.
- No live-site audits, real Chrome execution, Docker/Compose E2E, browser UX run, race-detector run, full backend suite, or performance benchmarks were performed. Backup/restore's bounded archive validation, ownership checks and staging/rollback structure were inspected; no new restore-integrity defect is claimed from that inspection. Runtime-manager contention in A05 can delay quiescence, although quiesce itself detaches the runtime before closing it.

## Recommended order — not yet an accepted implementation plan

1. Restore a green frontend test gate; correct A01–A03 with focused regressions.
2. Correct browser teardown and source/runtime cancellation/invalidation ownership (A04–A06).
3. Fix concrete persistence/input/identity contracts (A07–A09).
4. Deepen the actual reader/request/workflow boundaries (A10–A11) only to the extent the fixes demonstrate a need.
5. Measure and prioritize A12 and other performance hypotheses. Do not begin with a rewrite, mass dead-code cleanup, or bulk test expansion.
