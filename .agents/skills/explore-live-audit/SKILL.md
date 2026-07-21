---
name: explore-live-audit
description: Audit NovelReader Explore compatibility with a deterministic disjoint live-source sample. Use when asked to audit, randomly sample, validate, or find shared gaps in the Explore system.
---

# Explore Live Audit

Run a **funnel**: broad deterministic sampling, sequential confirmation, focused live diagnosis, then an approval gate. Audit and report only; fixes require a new explicit request.

## 1. Gate the batch

Read `PLAN.md` and `testdata/booksource/README.md`, inspect every prior `explore-live-audit-*.json`, and check git status without touching unrelated changes.

Ask the user for batch size every run and recommend one based on remaining unsampled enabled Explore identities and the latest shared-gap yield. Offer concrete sizes, normally 25 and 50. Record the chosen size, rationale, seed, corpus SHA-256, and prior manifests excluded.

**Done when:** the user chose a size and every previously sampled stable identity is in the exclusion set.

## 2. Freeze the sample

Use `test_booksource4.json`. Identity is `(rawIndex, bookSourceUrl)`, never name alone. Keep only enabled Explore sources with a usable Explore rule. Rank eligible unsampled identities by:

```text
SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)
```

Take the first N. Default to an unrestricted random sample; add strata only when the user requests targeting, and record them exactly. Never substitute identities after seeing results.

Before live execution, update `PLAN.md` with the active audit and commit the plan/skill artifact separately from audit evidence.

**Done when:** the selected identities are reproducible from corpus, seed, exclusions, and ranking rule, with no overlap.

## 3. Run production behavior

Use a fresh isolated database and the production Explore API. Import the unmodified corpus. For each source:

1. Open its catalog.
2. Choose the first selectable category.
3. Fetch page 1 with a 90-second timeout.
4. Record status, duration, category, book count, distinct book URLs, exhaustion, diagnostics, and up to two sample books.

Use bounded concurrency of four clients for the initial pass. Keep source/parser code unchanged throughout the audit.

**Done when:** every sampled identity has a machine-readable initial result; no item is silently skipped.

## 4. Confirm every non-pass

Rerun every catalog failure, page failure, empty result, duplicate-only result, timeout, or suspicious diagnostic sequentially against the same fresh environment. Treat the sequential result as authoritative for classification while retaining both observations.

**Done when:** every non-pass has a sequential replay or a loud audit-infrastructure failure.

## 5. Diagnose through the funnel

For each confirmed non-pass, inspect the exact imported source contract and captured production diagnostic first. Then use the smallest next check that separates causes:

- direct DNS/HTTP and final URL/status;
- response body versus imported selectors, URL patterns, scripts, and category contract;
- Playwright only when a credible engine gap remains or rendered DOM is required;
- reduced production-runtime probes to compare equivalent explicit rules or isolate a missing bridge/helper.

Classify with concrete evidence, not intuition:

- `credible_nonempty`
- `engine_gap`
- `upstream_http` / `upstream_dns`
- `blocked_or_auth`
- `stale_source_contract`
- `site_drift`
- `source_incomplete_or_invalid`
- `legitimate_empty`
- `audit_infrastructure`

A shared engine gap requires a valid current source contract, live content that should match, and a production behavior mismatch attributable to a reusable parser, transport, catalog, or Java-bridge seam. Source-specific breakage stays source-specific.

**Done when:** every sample has one classification and every `engine_gap` includes reproducible rule/body/runtime evidence.

## 6. Preserve evidence

Write both:

- `testdata/booksource/explore-live-audit-vN-YYYY-MM-DD.json`
- `testdata/booksource/explore-live-audit-vN-YYYY-MM-DD.md`

The JSON must include schema version, date, seed, corpus path/SHA, sample size and raw indices, exclusions, ranking method, execution method, summary counts, and one full entry per identity. The Markdown must concisely state scope, result table, shared gaps, non-engine outcomes, and recommendation. Update `testdata/booksource/README.md` and `PLAN.md`, including an append-only Issues & Fixes entry for non-trivial findings.

Validate counts, uniqueness, disjointness, corpus hash, JSON parsing, and `git diff --check`. Commit evidence with `test: audit Explore live sample` or a similarly specific message.

**Done when:** another developer can reproduce selection and understand every classification from committed evidence alone.

## 7. Report and stop

Report sample composition, credible pass/book totals, every shared gap, other classifications, evidence paths, verification performed, and a recommendation for fixes or another batch. Explicitly state that no source/parser behavior changed during sampling.

Stop at the approval gate. Do not implement or patch findings in the same run.

**Done when:** the user has an evidence-backed audit report and can approve the next action independently.
