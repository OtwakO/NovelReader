---
status: active
updated: 2026-09-05
---

# Architecture and Code Quality Improvements

## Goal

Correct demonstrated architecture/code-quality defects while preserving product behavior, compatibility, reader isolation and storage integrity. Prefer the smallest complete ownership/interface correction over downstream patches, cosmetic splitting or speculative frameworks.

Done means affected contracts work, appropriate tests pass, performance claims have measurements, and documentation describes actual verification rather than intended outcomes.

## Scope

The original workstream covered frontend lint, frontend workflow cohesion, backend boundaries, auth-session deadline duplication, scheduled Source Collection reliability, and measurement-gated performance work.

The user subsequently requested an independent, single-agent audit of the current implementation and all branch commits. That review is complete at `33b9a1f`; its findings and recommendations live in the [independent architecture/code audit](../verification/independent-architecture-code-audit.md). They supersede the earlier audit's claim that the remaining priority was simply to choose a frontend decomposition boundary.

The user accepted careful implementation after a second verification, without subagents. Proceed in small corrective slices: (1) browser ownership and its failing test gate, (2) candidate transitions, (3) frontend account-state isolation, (4) source/runtime cancellation and invalidation, (5) upload/font/identity contracts. Larger request/workflow refactors must earn their scope from the corrected contracts; performance work remains measurement-gated.

Excluded without separate approval:

- new product features, UX redesign, generic providers or a DI/event framework;
- bulk deletion based on static reachability hints;
- parser rewrites without deterministic compatibility evidence;
- mass file splitting or formatting to meet line-count targets;
- speculative caching, concurrency or infrastructure changes without measurements.

## Accepted approach and decisions

- Keep logical changes independently verifiable; maintain this plan at meaningful stopping points.
- Preserve the real backend-only BookSource execution, reader-scoped storage and private WebView boundaries.
- Use synthetic fixtures and focused contract tests; real BookSources remain optional ignored local evidence.
- Auth-session late-creation cleanup is one genuinely shared behavior, not a generic background-task framework.
- Performance hypotheses remain measurement-gated. File length and test count alone do not establish defects.
- Implement confirmed correctness findings at their owning boundary; do not treat the audit as permission for every suggested refactor. Recheck code and direct callers before each slice.

## Current state

Branch: `review/architecture-code-audit`. Audit baseline: merge-base `4456b1bf5f7117288fcedc01025894ae0af6f9ff`; reviewed HEAD: `33b9a1f`.

| Prior phase | Committed outcome | Independent review status |
|---|---|---|
| Initial audit plan | `d773c8e` | Earlier completeness/prioritization claims superseded by the independent audit. |
| F01: frontend lint | `0a390e9` | Lint and CI wiring are useful; Options API conversion left a failing SourceBrowserSession test. Not fully green. |
| F08: auth deadlines | `1c219b4` | Shared helper is justified; no new defect established in the extraction. Existing auth tests pass. |
| F09: scheduler reliability | `33b9a1f` | Committed, not awaiting a commit. Improvements are partial: downstream context-free reads and lifecycle discrepancies remain; no scheduler regression was added. |
| Other frontend/backend/performance proposals | Not implemented | Reprioritize using the independent audit, not the previous F-number order. |

The independent review reproduced account-state leakage, cancellation bypass in candidate shelving, worker double-release, browser UI post-unmount creation, and a context-free collection read bypassing cancellation. It also traced source-lifecycle, upload, font-persistence and identity-contract defects. Evidence, severity, recommendations and limits have one canonical home: the linked audit.

No production fixes were made during the independent audit. Temporary synthetic probes were removed. Second verification has reconfirmed the browser ownership and candidate transition defects. Corrective implementation is in progress. A03/A04 are fixed: the worker transfers capacity ownership at registration; the browser component marks its lifetime closed before teardown, closes late-created sessions and prevents stale polling/close continuations. The missing test i18n injection is corrected. Verification: 3 browser-component tests, TypeScript checking and changed-file ESLint pass; 11 worker interaction/health tests pass (mocked browser, not real Chrome).

A02 is fixed: candidate acceptance, shelving, cancellation and expiry share one transition lock, while snapshot reads remain independent of slow persistence. Terminal states cannot acquire write eligibility again; automatic commit failures remain retryable and completed commits remain idempotent. Candidate and API package tests pass; targeted terminal/idempotent-commit tests also pass under the race detector.

A01 is fixed: one application-level identity boundary invokes feature-local resets and aborts previous-identity HTTP work. Search/Explore generations remain monotonic across resets; old candidate events and queued progress writes cannot repopulate new-reader state. Owner-tagged tab restoration remains available for the same reader, and browser appearance preferences are preserved. No schema or HTTP payload changes.

A05/A06 are fixed: runtime cleanup is owned, capacity-counted and outside the manager mutex; cancellable waits replace lock-held draining. Collection replacement/deletion return affected IDs from their own transactions rather than a separate read. Success-record reads honor context. Manual and scheduled changes share browser/source-session invalidation; cleanup failure after commit is explicit, and scheduler sweeps reconcile even when no collections remain. API, BookSource, source-profile, reader-store, backup and auth package tests pass; runtime/quiescence/scheduler/partial-cleanup regressions pass under the race detector. OS filesystem calls remain synchronous; cancellation does not abandon owned cleanup.

A07 is fixed: source replacement enforces 50 MiB before JSON parsing; font upload enforces a 21 MiB request cap and a 20 MiB parsed-file cap before reading/persistence. Oversize input returns HTTP 413; multipart temporary files are removed. Unknown-length source overflow, both font boundaries, temporary-file cleanup and normal font persistence are covered by focused tests. The API package passes. README documents the limits. No schema changes; rollback is code-only.

After the inference interruption, the working tree and `97a04af` were checked; API, BookSource and source-profile package tests passed again, as did focused runtime/quiescence/scheduler/partial-cleanup tests under the race detector.

## Next action

Address font persistence and logical-book identity contracts (A08–A09), rechecking each direct boundary first. A01–A07 and the original frontend test regression are complete. Font replacement must preserve existing behavior without losing cleanup references on failure; confirm any storage migration before implementation.

Runtime approach: keep closing runtimes tracked (and counted against capacity) until their owned cleanup finishes. Start cleanup outside the manager lock; same-reader acquisition/quiescence waits on a completion signal with the caller's context, while unrelated readers remain usable. Shutdown drains tracked cleanup; no detached unbounded cleanup pool or new runtime for a still-closing reader. Propagate context to collection reads rather than wrapping context-free calls. No schema changes; rollback is code-only, but runtime lease/drain tests are required. Promote only the relevant diagnostic reproductions into maintained tests; do not resume cosmetic decomposition automatically.

## Verification

Independent review at `33b9a1f`:

- Backend: `go test ./internal/auth ./internal/api ./internal/booksource ./internal/candidate ./internal/sourceinteraction ./internal/readerstore` passed for those six packages (cached); no full backend-suite claim.
- Current frontend verification after A01–A04: **189/189 tests passed across 56 files**, lint and production build (including TypeScript checking) passed. This supersedes the audit's 181/182 result.
- `npm run build` passed separately, including TypeScript checking.
- Synthetic diagnostic probes reproduced the five defects named above; they intentionally failed assertions of the desired behavior and were removed, not committed as tests.
- AFT inspection now completes but reports no authoritative Go diagnostics (`gopls` unavailable); Go tests are authoritative. Targeted candidate transition/idempotence tests passed under the race detector. No Docker/Compose E2E, real browser-worker execution, live-source audit, browser UX verification or performance benchmark was performed.

Earlier clean-test claims are not the current verification state. See the [audit verification and limits](../verification/independent-architecture-code-audit.md#tests-dead-code-overfitting-and-review-limits) for exact scope.

## Compatibility and rollback

Initial slices change no schema or HTTP payload contract and can be reverted independently. Browser cleanup must preserve explicit save/continuation behavior; candidate cancellation/expiry must reject new writes without breaking idempotent completed commits or retryable persistence failures. Account cleanup must preserve browser-owned appearance settings while removing reader-owned state. Source-state/runtime work must preserve credential separation, source identity and lease draining. Confirm any storage migration before implementation. Preserve the branch's working auth extraction.
