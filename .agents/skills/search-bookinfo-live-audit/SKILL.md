---
name: search-bookinfo-live-audit
description: Audit NovelReader Search and Book Info compatibility with a deterministic live-source sample. Use when asked to sample, validate, or find shared gaps in searching and fetching book details.
---

# Search → Book Info Live Audit

Run a bounded funnel: deterministic search sampling, detail-only follow-up, sequential replay, focused diagnosis, then stop before fixes.

## 1. Freeze scope

Use `test_booksource3.json`. Stable identity is `(rawIndex, bookSourceUrl)`, never source name. Eligible entries must be enabled text sources with non-blank `searchUrl` and `ruleSearch`. Rank identities by:

```text
SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)
```

Take the user-approved count without substitutions. Record seed, corpus SHA-256, query, eligibility rule, and selected identities.

## 2. Reset and import

Stop existing NovelReader processes. Remove the disposable development data root, start the current production server with an isolated bootstrap token, complete Administrator setup, and import the unmodified full corpus. Never migrate or preserve audit user data.

Preserve unrelated working-tree changes, especially generated frontend files.

## 3. Execute only the target workflow

For every selected identity:

1. Run page-1 search with the frozen query through production transport/rules.
2. Record exact expanded request, redacted headers, response status/final URL/body sample, parser output, duration, and diagnostics.
3. If search returns results, choose the first result with non-blank name and book URL.
4. Fetch Book Info only with that complete search-result context.
5. Record enriched fields or the exact detail-stage failure.

Do not fetch TOC or chapter content. Use bounded concurrency only for the initial pass. Rerun every non-pass sequentially.

## 4. Classify conservatively

The imported BookSource contract is the source of truth, but a source need not still be valid or online. Consult both:

- `https://mgz0227.github.io/The-tutorial-of-Legado/Rule/source.html`
- `reference/legado`

Use these classifications:

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

An engine gap requires all three: a valid BookSource contract under documented/upstream Legado semantics, live compatible data, and a reusable NovelReader transport/parser/Java-bridge mismatch. Never patch or generalize from one broken source.

## 5. Preserve evidence and stop

Write:

- `testdata/booksource/search-bookinfo-live-audit-vN-YYYY-MM-DD.json`
- `testdata/booksource/search-bookinfo-live-audit-vN-YYYY-MM-DD.md`

Keep scripts under `scripts/search-bookinfo-audit/vN/`. Validate counts, identities, corpus hash, JSON, and `git diff --check`. Report shared gaps and non-engine outcomes, explicitly state that no source/parser behavior changed, and stop for approval before fixes.
