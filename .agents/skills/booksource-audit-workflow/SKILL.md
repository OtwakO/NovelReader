---
name: booksource-audit-workflow
description: Design a focused NovelReader BookSource live-audit workflow. Use when starting an audit for a new operation, defining deterministic sampling/evidence tooling, or when an operation-specific audit skill needs a shared audit structure.
---

# BookSource Audit Workflow

Build a **funnel**, not a framework: broad observation, sequential confirmation, smallest-tool diagnosis, evidence, approval gate.

## 1. Bound the operation

Name one production workflow and its stopping point, such as Search → Book Info. Identify the production API/runtime seam, corpus, stable identity, eligibility rule, query/input, and fields that constitute a credible pass.

Keep adjacent stages outside scope unless they are required to execute the chosen operation. Prefer extending existing conformance observability over building an audit-only runtime.

**Done when:** one sentence states the exact start, stop, and pass condition, and every live request belongs to that boundary.

## 2. Freeze a reproducible sample

Use `(rawIndex, bookSourceUrl)` identity. Record corpus path/SHA-256, seed, ranking rule, exclusions, sample size, and any user-requested strata. Never substitute identities after observing results.

Ask for sample size when runtime or diagnosis volume changes materially; recommend the smallest batch likely to expose shared gaps.

**Done when:** another developer can regenerate the same ordered identities without live access.

## 3. Create operation-owned artifacts

Use this layout:

```text
.agents/skills/<operation>-live-audit/SKILL.md
scripts/<operation>-audit/vN/
testdata/booksource/audits/<operation>/
```

Put deterministic regression inputs under `testdata/booksource/conformance/<operation>/`, never in the audit directory. Reuse shared runner code; version only operation-specific evidence scripts. Do not add a registry, abstraction, or dependency for one audit.

**Done when:** scripts, live evidence, and offline fixtures each have one obvious owner and no file is placed directly in `testdata/booksource/`.

## 4. Run production behavior

Use a fresh disposable data root and import the corpus unchanged. Capture every selected identity, including failures. Use bounded concurrency for the initial pass and sequentially replay every non-pass or suspicious result.

Sampling is read-only with respect to source/parser behavior. Preserve unrelated working-tree changes.

**Done when:** every frozen identity has an initial observation and every non-pass has an authoritative replay or explicit infrastructure failure.

## 5. Diagnose with the smallest tool

Treat the imported BookSource as the contract to evaluate, not proof that its site still works. Check in order:

1. captured production request, response, rule, and diagnostic;
2. direct DNS/HTTP, redirects, headers, status, and body;
3. body versus the authored selectors/scripts/options;
4. official rule documentation and `reference/legado` behavior;
5. Playwright when rendered DOM, browser JavaScript, cookie/session behavior, or anti-bot presentation can distinguish source drift from an engine gap;
6. a reduced production-runtime probe to isolate a reusable seam.

Playwright is evidence, not an automatic fallback: record what browser behavior proves that direct HTTP could not.

**Done when:** every non-pass has concrete evidence separating upstream/source failure from NovelReader behavior.

## 6. Require shared-gap proof

Promote an engine gap only when all are true:

- the imported contract is valid under documented/upstream Legado semantics;
- current content or browser behavior should satisfy it;
- NovelReader disagrees at a reusable parser, transport, state, or bridge seam.

Seek a second source, corpus inventory, or reduced fixture when practical. A single source may prove a gap when the upstream contract and live counterfactual are conclusive, but the fix still targets the shared seam. Record unsupported syntax separately when live validity is unproven.

**Done when:** each engine gap explains the shared contract, counterfactual behavior, and affected seam without naming a source in the proposed implementation.

## 7. Preserve evidence and stop

Write JSON plus Markdown under `testdata/booksource/audits/<operation>/`. Include selection metadata, environment, initial/replay observations, classifications, references, shared gaps, limitations, and whether Playwright was used. Update the root testdata map and `PLAN.md` only when current state changes.

Validate JSON, counts, identities, corpus hash, operation path ownership, and `git diff --check`. Report findings and stop before fixes. Fixes require approval, public-seam TDD, and a targeted live rerun stored beside the audit evidence.

**Done when:** the audit is reproducible, classifications are reviewable, no source/parser fix is mixed into sampling, and the user can approve one bounded next action.
