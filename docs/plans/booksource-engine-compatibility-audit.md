---
status: active
---

# BookSource engine compatibility audit

## Goal

Independently identify reusable engine mismatches against documented/observed Legado semantics, anchored in the reported high search failure rate. Done means evidence-backed findings and bounded corrective proposals, not a claim of universal source compatibility or a rewrite.

## Scope

Review shared rule, JavaScript, request, transport and session semantics. First live slice is page-1 Search → first credible Book Info; no live catalog/content crawl. Inspect adjacent workflows when a shared contract requires it. The user has approved re-verification followed by fundamental fixes for confirmed findings; unsupported features and unresolved observations are not automatically implementation scope. No automatic captcha/WAF bypass or source-specific patches.

## Accepted approach

- Branch: `audit/booksource-engine-compatibility`; baseline `bf0c672`.
- User-provided ignored corpus: `test-booksources/new_test_booksource.json`; SHA-256 `122b3a3a49bcd3f5e9464584691a726e1b901829aeedceb375012fb8405ffe40`.
- Corpus: 306 definitions, 277 eligible enabled text searches. Freeze 50 by SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl), seed `NovelReader-engine-audit-v1`, query `凡人修仙传`; no substitutions. Duplicate source URLs remain independent raw-index identities.
- Use disposable isolated runtime state, never the developer's installed account/data. Preserve definitions unchanged. Run production search with bounded concurrency, then sequentially replay non-passes. A diagnostic tool's classifications are observations, not final root causes.
- Compare the [rule guide](https://mgz0227.github.io/The-tutorial-of-Legado/Rule/source.html) with `reference/legado-E` (`8b87c5aba4df91c39a3a0939a68a1180b9f2ee1c`) and `reference/web-legado-rust` (`37eff8639f6a50c5ff16820c80af087bb4cc1aa6`). Android implementation is the primary behavioral reference; Rust is a secondary comparison, not proof by itself.
- Keep complete source objects, scripts, HTTP bodies/headers and raw diagnostics under ignored `test-booksources/engine-audit/`. Commit only sanitized findings and minimal invented regression fixtures. Do not refresh historical raw audit artifacts.
- For a suspected shared gap, isolate a failing production-seam example and verify expected semantics. Separate proven live impact, deterministic semantic mismatch with unknown live impact, upstream failure, blocking/authentication, stale source and unresolved evidence.

## Corrective implementation approach

Re-verify each reported contract before changing production code. Implement E04 at the shared source bridge so source identity and source variables do not vary with the current response URL; retain standalone evaluator behavior when no source definition exists. Then make source metadata and workflow RuleData explicit at the URL/page evaluation seam (E01/E03), preserving independent book/chapter/source lifetimes across Search, Explore, detail, TOC and content. Implement E02 in the shared response parser rather than a search-handler workaround. E05 is an independent URL phase-order correction, verified in an isolated worktree and reviewed before integration. No new cache, parser framework, arbitrary JVM emulation or source-specific exceptions.

Use small synthetic regressions that fail first, then package/boundary tests and only affected live identities. Keep the frozen initial evidence intact. Source-variable key correction changes previously incorrect page-keyed state; no automatic state migration, data reset, or schema change is planned. Rollback is reverting the relevant logical commit; values written under the corrected source key will not be visible to the old page-keyed implementation. Core context changes must preserve per-installed-source and per-reader isolation.

## Current state

Corrective progress: E04 re-verified red→green and fixed in the shared source bridge. Prepared source metadata supplies the stable key for helpers and source-variable storage; independently supplied source states remain isolated even for identical source URLs. Standalone evaluations without source metadata retain their existing caller-provided identity. Analyzer, book and sourceexec package tests pass. E01/E03 context propagation is still required before every URL caller supplies this metadata. E05 was independently reviewed and integrated (`f9e6cbc`): URL JS precedes whole-rule interpolation, then page selection/options parsing; interpolation errors stop request construction. Parent verification also reproduced and corrected the adjacent nested-object brace bug in the shared rule/URL scanner by counting individual expression braces. A public URL regression fails before that correction and passes afterward; analyzer, book and sourceexec packages pass on the combined changes. No broader interpolation or engine certification is claimed.

Frozen sample and manifest are under ignored `test-booksources/engine-audit/`. Added a small explicit production-Searcher audit command and v5 freeze/run scripts; old conformance search is not used because it reconstructs execution and omits production dynamic-header evaluation. Initial 50-source run: 19 search errors, 3 detail errors, 28 search/detail returns (not yet all validated as credible). All 22 non-passes replayed sequentially with the same coarse outcomes. One missing-browser observation was rerun with an isolated worker and returned HTTP 403; that worker was removed without touching the running app.

Two shared defects have deterministic synthetic reproductions: missing redirected-detail search fallback, and lost search-URL `java.put` variables. Captured response counterfactuals prove the authored detail rules work for sample indices 26 and 33; index 39 does not pass that counterfactual and remains unresolved. Restoring variable ownership only in a private probe restores live Book Info for indices 5 and 35 (two raw definitions of the same contract family). Production engine code remains unchanged. A further private counterfactual confirms missing source metadata in URL JS (index 17): binding the original definition restores live search. Synthetic checks also reproduce response-URL-dependent source identity/variables and URL template phase-order defects. Missing `java.getWebViewUA` (index 0) and `JavaImporter` (index 25) are confirmed unsupported surfaces, with live recovery unproven. Seven suspicious apparent successes were replayed sequentially and remain suspect; only 23 of 31 search returns contain the exact query in a title.

The canonical findings and recommended boundaries are in [the initial audit report](../verification/booksource-engine-compatibility-audit.md), with [sanitized per-sample observations](../../testdata/booksource/audits/search-bookinfo/engine-audit-v5-2026-09-06.json). This is a checkpoint, not a completed audit or engine sign-off.

## Next action

Re-verify and implement the accepted corrections, starting with shared source identity and context ownership. Unsupported UA/JVM helpers still require separate contract verification and a bounded implementation decision. Preserve the original sample and observations before post-fix reruns. No full compatibility claim is justified. Any accepted structural correction needs an explicit ownership approach and public-seam regressions before production edits.

## Verification

Verified: corpus/selection hashes and private exclusion; audit command compiles via `go test ./cmd/engine-audit` and `go build`; initial and sequential live runs completed. Private synthetic scripts `probe_direct_detail.py` (two cases) and `probe_variable_handoff.py` fail with the expected defects on the unchanged engine. Temporary package counterfactual probe was removed after execution: 4 of 5 subcases succeeded; the negative outcome at index 39 is preserved in private evidence rather than hidden. The source-metadata live counterfactual subsequently passed; source-identity and URL phase-order probes failed as expected. Existing analyzer, book and sourceexec package tests pass, and the final audit command compiles (`go test ./cmd/engine-audit ./internal/analyzer ./internal/book ./internal/sourceexec`). These passing tests do not cover away the reproduced defects. AFT subsequently completed but had no authoritative Go diagnostics because gopls is unavailable.

Limits: native isolated per-source processes call production Searcher/Book Info methods, not authenticated HTTP batch search; no persisted login state, full Android runtime, cross-source contention or device-identity hydration is exercised. Transport evidence captures main HTTP/WebView requests, not all JS-internal AJAX calls; response previews cap at 1 MiB. No real BookSource is tracked. Existing application data and Compose configuration are unchanged.
