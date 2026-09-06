# BookSource engine compatibility audit — initial findings

**Status: audit in progress; not an engine sign-off.** Baseline `bf0c672`, branch `audit/booksource-engine-compatibility`. Scope, reference revisions and private-input policy are in the [active plan](../plans/booksource-engine-compatibility-audit.md).

## What was actually run

[Sanitized per-sample observations](../../testdata/booksource/audits/search-bookinfo/engine-audit-v5-2026-09-06.json) record identity hashes and coarse outcomes without raw source content.

A reproducible sample of 50 of 277 eligible definitions from the user-provided corpus was searched for `凡人修仙传`, followed by Book Info for the first nonblank name/URL result. Production Searcher methods were used, with fresh per-source processes, four workers initially and sequential confirmation. Sources were not edited, replaced or substituted after observing outcomes.

- Initial: **19 search errors, 3 detail errors, 28 search/detail returns**.
- All 22 non-passes reproduced their coarse outcome sequentially.
- Seven apparent successes with unrelated names or collapsed URLs were also replayed; all still returned data. Across all 31 search returns, only 23 had a title containing the query. This is a scrutiny signal, not a relevance-based rejection rule.
- Sample 10 returned 20 results sharing one book URL. A returned value is clearly not enough to certify this source.
- Sample 24 initially lacked a WebView transport. A separate isolated-worker replay returned HTTP 403; the worker was then removed. Do not count the initial infrastructure error as an engine defect.
- No complete source definition, response body, source URL or credential is included in this report. Sample numbers refer to the frozen local manifest, not names/domains.

## Confirmed shared defects with current-source counterfactuals

### E01 — Search URL variables have no workflow owner

**Impact:** two sampled raw definitions (5 and 35, the same contract family) lose request options during Search → Book Info, producing HTTP 401. This is not merely an upstream authentication failure.

`book/search.go::searchSource` builds URL context with only the JS library. `analyzer/context_variables.go::newEvaluationVariables` installs fallback variable storage only when both an analyzer and source state exist. URL evaluation has neither an active analyzer nor book/chapter rule data, so `java.put` returns its argument without retaining it. Later result rules calling `java.get` see an empty value.

Evidence:

- A wholly synthetic local source stores request options in search-URL JS, then retrieves them while constructing the result URL. Search returns a book, but detail lacks the required synthetic header and receives 401.
- A private counterfactual gave the original URL script a variable owner, then passed those values to the existing parser. Both original definitions subsequently fetched Book Info successfully without changing their authored scripts or endpoints.
- Android `AnalyzeUrl.kt:390–411` stores variables on chapter/RuleData. `BookList.kt:72,103,121` passes workflow variables into result parsing.

**Recommended correction:** make request/workflow RuleData explicit from URL evaluation through result parsing and selected-book context. Keep chapter/book/source ownership distinct. Do not turn `java.put` into a global source cache, special-case the `headers` key, or add retry/header patches to the detail handler. The fake-book owner used by the private counterfactual is proof, **not** a proposed implementation.

### E02 — Search never takes the documented detail-page branches

**Impact:** valid search redirects become `no elements matched bookList rule`. Sample 26 supplies a confirmed live case. Captured responses from 26 and 33 satisfied their unchanged Book Info rules, but that alone did not establish fallback eligibility for 33.

`book/search.go::parseSearchResultWithRuleStateContextAtURL` rejects an empty list and does not inspect `bookUrlPattern`. Android `BookList.kt:62–108` and Rust `service/search.rs:832–874` both:

1. parse as detail when the response URL matches the configured pattern;
2. when no pattern is configured and the list is empty, attempt detail parsing.

Evidence: two deterministic local-HTTP cases failed on the baseline production search path (pattern match and empty-list fallback). Detail-only counterfactuals parsed two captured responses. Subsequent unchanged-source replay established that sample 33 has a nonmatching configured pattern, which excludes the reference's empty-list fallback; do not count it as a proven engine recovery or bypass its pattern. Sample 39 also returned a detail-shaped page, but its own detail rules did **not** pass the counterfactual: its validity remains unresolved. Current implementation and replay outcomes are in the active plan.

**Recommended correction:** implement these branches in the shared search response workflow, reusing existing detail parsing and preserving response URL, variables and source identity. Do not invent selectors, retry another endpoint, or return a book merely because the page has a title. Existing “empty result URL uses response URL” tests cover a different behavior and missed this gap.

### E03 — URL JavaScript lacks the source definition

**Impact:** sample 17 evaluates a script stored in `source.bookSourceComment`; the missing property leads to an empty search URL before HTTP.

`analyzer.URLContext`/`urlBindings` carry crawl objects and the JS library but no source metadata. Page analyzers do supply metadata via `book/context.go::setAnalyzerContextData`. The bridge can expose metadata, but URL callers never provide it. Android `AnalyzeUrl.kt:365–378` binds the actual source.

Evidence: baseline URL construction fails; evaluating the unchanged authored script with the source metadata bound produces a URL, and executing that evaluated URL through production search restores live results.

**Recommended correction:** propagate one explicit source-definition context to URL and page evaluation. Do not add just `bookSourceComment` or rewrite sources that store scripts there. This can share the context-boundary work of E01, but source metadata is not mutable workflow RuleData.

## Confirmed semantic mismatches; sampled live impact not yet established

### E04 — Source identity changes with the response URL

`analyzer/js.go::makeSourceObj` uses the current `baseURL` for `source.key`/`getKey`, and `jsSource.GetVariable`/`PutVariable` key source state by that URL. In a synthetic same-source session, writing a source variable on `/search` and reading it on `/book/1` loses the value; `getKey()` also returns the search URL instead of the source key.

Android `BookSource.kt:109–111` returns `bookSourceUrl`; the source object is distinct from the current response URL. Correct the source-identity binding and state key together. Do not merge separate installed sources merely because their source URLs match.

### E05 — URL template phases run in the wrong order

`analyzer/urlbuilder.go` parses URL options before interpolation, interpolates only URL/body, and runs page-selection matching before general `{{...}}` evaluation. Two synthetic checks fail:

- an option header containing `{{key}}` is sent unexpanded;
- a valid JS expression containing `<`, a comma and `>` is corrupted by the page-selector regex before JS evaluation.

Android `AnalyzeUrl.kt:149–156,190–240` explicitly evaluates embedded JS before page selection and URL option parsing, including a comment explaining the comparison-operator hazard. Fix the expansion phases, not isolated headers or one ternary expression. Also review the current warning-and-continue behavior when expansion fails; it can turn a script error into a misleading network/empty-result failure.

## Explicit unsupported surface, not proof of stale sources

- **`java.getWebViewUA`:** sample 0 fails in source-header evaluation before HTTP. Android `JsExtensions.kt:689–692` supplies it. Three corpus entries reference it. Its missing bridge method is confirmed; current upstream success after providing an appropriate UA is not yet proven. Use the actual configured browser identity if implementing it, not an arbitrary permanent UA string.
- **`JavaImporter`:** sample 25 fails before the main request; nine corpus entries reference it. A Goja runtime plus a partial `Packages` object is not a full Rhino/JVM environment. Inventory the concrete imported classes before proposing a bounded bridge; do not promise arbitrary Java support or silently emulate it with empty stubs.

## Remaining evidence and limitations

Timeouts, HTTP 404/403 responses, empty API responses and failed selectors remain **observations**, not conclusions that the source is obsolete. The remaining failures and suspicious returns still require bounded source-contract/body checks. No percentage of “working sources” is claimed.

The probe is not an authenticated 50-source API request: it does not cover shared-reader contention, persisted login state, device-identity hydration or every JS-internal HTTP call. Detail names can be inherited from search, so a successful detail call alone is not validation. Main response previews are capped at 1 MiB. Android/Rust source inspection is not execution of a full Android reference app.

Broader CSS/Default indexing, JSONPath/XPath/regex dialects, JS bridge coverage, nested request semantics, catalog/content pagination and concurrent source isolation are **not certified** by this first slice. No production fix, source edit or data reset has been made. Existing analyzer, book and sourceexec package tests pass; the audit command compiles. The deliberately failing private synthetic probes are not installed in the default test suite.

## Recommended order

1. Complete the remaining bounded classification and preserve synthetic regressions for E01–E05 at the correct public seams.
2. Present a focused implementation proposal for the context boundary (E01/E03/E04) and search detail fallback (E02), rather than patching symptoms in handlers.
3. Correct URL phase ordering (E05), then address measured bridge needs separately.
4. Re-run only affected frozen identities after fixes, plus the original scoped regressions. Continue other engine families with evidence, not a new parser framework or a full JVM rewrite.

The audit remains active. These findings already establish that the engine is responsible for some of the reported failure pattern; they do not establish that all remaining failures are upstream or that all shared gaps have been found.

## Post-fix comparison

The same frozen sample was rerun after E01–E05 at e752412. See the [sanitized post-fix report](../../testdata/booksource/audits/search-bookinfo/engine-audit-v5-post-fix-e752412.md) for current observations, unresolved classifications and bounded next approaches. The initial figures above are historical, not current pass rates.
