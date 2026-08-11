---
name: search-bookinfo-live-audit
description: Audit NovelReader Search and Book Info compatibility with a deterministic live-source sample. Use when sampling search/detail behavior or finding shared gaps in those stages.
---

# Search → Book Info Live Audit

Follow `.agents/skills/booksource-audit-workflow/SKILL.md`. This skill supplies only the Search → Book Info branch.

## 1. Freeze scope

Use `test_booksource3.json`. Eligible identities are enabled text sources with non-blank `searchUrl` and `ruleSearch`. Rank `(rawIndex, bookSourceUrl)` by:

```text
SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)
```

Take the approved count without substitutions. Record query, seed, corpus SHA-256, eligibility, and identities.

**Done when:** the exact ordered sample and query are reproducible without live access.

## 2. Execute only both stages

Use a fresh disposable data root and execute the exact frozen raw definitions unchanged. Do not import the whole compilation when duplicate `bookSourceUrl` values could replace a sampled raw-index contract. If the conformance runner reads a frozen array directly, map each stable `(rawIndex, bookSourceUrl)` identity to its frozen-array position and verify the preserved definition byte-for-byte. For each identity:

1. run page-1 production search with the frozen query;
2. capture expanded request, redacted headers, status/final URL/body sample, results, duration, and diagnostics;
3. choose the first result with non-blank name and book URL;
4. fetch Book Info with the complete search-result context;
5. stop before TOC/content.

Use bounded concurrency for the initial pass. Sequentially replay every non-pass.

**Done when:** every identity has search evidence and each credible result has detail evidence or an exact detail failure.

## 3. Diagnose and classify

The imported source is the contract to evaluate, not proof that its site remains valid. Consult the official rule documentation and `reference/legado`, following the shared smallest-tool ladder. Use Playwright when rendered DOM, browser JavaScript, session state, or anti-bot behavior can distinguish source drift from an engine gap.

Classify as:

- `credible_search_and_detail`
- `search_engine_gap`
- `detail_engine_gap`
- `upstream_http` / `upstream_dns`
- `blocked_or_auth`
- `stale_source_contract`
- `site_drift`
- `source_incomplete_or_invalid`
- `legitimate_empty`
- `audit_infrastructure`

Apply the shared-gap proof gate. Record unsupported syntax separately when the sampled source cannot prove current working impact.

**Done when:** every non-pass has evidence separating source/upstream failure from a reusable NovelReader mismatch.

## 4. Preserve and stop

Keep scripts under `scripts/search-bookinfo-audit/vN/`. Write:

- `testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-vN-YYYY-MM-DD.json`
- `testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-vN-YYYY-MM-DD.md`

Store targeted post-fix reruns in the same operation directory. Validate counts, identities, corpus hash, JSON, path ownership, and `git diff --check`. Report shared gaps, non-engine outcomes, Playwright use, and verification; state that sampling changed no source/parser behavior and stop for approval.

**Done when:** evidence is reproducible and reviewable, and one bounded next action can be approved independently.
