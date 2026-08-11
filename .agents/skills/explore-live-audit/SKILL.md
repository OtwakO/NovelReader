---
name: explore-live-audit
description: Audit NovelReader Explore compatibility with a deterministic disjoint live-source sample. Use when sampling Explore, validating catalog/page behavior, or finding shared Explore gaps.
---

# Explore Live Audit

Follow `.agents/skills/booksource-audit-workflow/SKILL.md`. This skill supplies only the Explore branch.

## 1. Gate and freeze

Read `PLAN.md`, `testdata/booksource/README.md`, and every JSON manifest in `testdata/booksource/audits/explore/`. Ask for batch size and recommend 25 or 50 based on remaining unsampled identities and recent shared-gap yield.

Use `test_booksource4.json`. Eligible identities are enabled sources with a usable Explore rule. Rank unsampled `(rawIndex, bookSourceUrl)` identities by:

```text
SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)
```

Take the first N without substitution. Record exclusions, corpus SHA-256, seed, rationale, and any requested strata. Update and commit the active plan before live execution.

**Done when:** the ordered sample is reproducible and disjoint from every prior Explore audit.

## 2. Execute Explore only

Use a fresh disposable database and the production Explore API. Build an import containing the exact frozen raw-index definitions and import it unchanged. Do not import the whole compilation: `bookSourceUrl` is the storage key, so a later duplicate URL can replace the sampled raw-index contract. Assert that the frozen import has N unique URLs. For each identity:

1. open its catalog;
2. choose the first selectable category;
3. fetch page 1 with a 90-second audit timeout;
4. record duration, category, book count, distinct URLs, exhaustion, diagnostics, and up to two books.

Use four clients for the initial pass. Sequentially replay catalog failures, page failures, empty/duplicate-only results, timeouts, and suspicious diagnostics. Keep both observations; use replay for classification.

**Done when:** every frozen identity has an observation and every non-pass has a replay or explicit infrastructure failure.

## 3. Diagnose and classify

Use the shared smallest-tool ladder. Use Playwright when rendered categories/results, browser JavaScript, cookie state, or anti-bot presentation can resolve ambiguity; state exactly what it proves.

Classify as:

- `credible_nonempty`
- `engine_gap`
- `upstream_http` / `upstream_dns`
- `blocked_or_auth`
- `stale_source_contract`
- `site_drift`
- `source_incomplete_or_invalid`
- `legitimate_empty`
- `audit_infrastructure`

An Explore engine gap must satisfy the shared-gap proof gate and identify a reusable catalog, parser, transport, state, or Java-bridge seam. Keep source-specific breakage source-specific.

**Done when:** every identity has one evidence-backed classification and every engine gap has a reproducible counterfactual.

## 4. Preserve and stop

Keep scripts under `scripts/explore-audit/vN/`. Write:

- `testdata/booksource/audits/explore/explore-live-audit-vN-YYYY-MM-DD.json`
- `testdata/booksource/audits/explore/explore-live-audit-vN-YYYY-MM-DD.md`

Targeted post-fix reruns stay in the same directory. Validate counts, uniqueness, disjointness, corpus hash, JSON, path ownership, and `git diff --check`. Update `testdata/booksource/README.md` only if placement policy changes; update `PLAN.md` for current findings.

Report sample composition, credible books, shared gaps, other outcomes, Playwright use, and verification. State that sampling changed no source/parser behavior, then stop for approval.

**Done when:** committed evidence independently explains selection and every classification, with no fix mixed into sampling.
