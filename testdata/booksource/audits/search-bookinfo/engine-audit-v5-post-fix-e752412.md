# Engine audit: post-fix comparison at e752412

Status: observations, **not a compatibility sign-off**. [Sanitized records](engine-audit-v5-post-fix-e752412.json). [Original audit](../../../../docs/verification/booksource-engine-compatibility-audit.md). [Active work](../../../../docs/plans/booksource-engine-compatibility-audit.md).

## Scope and verification

Same frozen 50 definitions and `凡人修仙传` query; frozen SHA-256 verified against the original manifest and evidence. The rebuilt probe calls production Search → Book Info with result variables preserved. Each source has a fresh process, without installed reader state or a WebView worker. It does not test batch scheduling, browser behavior, TOC or content. All 19 non-passes were replayed sequentially; seven suspicious returns were also rechecked sequentially.

Raw observations remain ignored under `test-booksources/engine-audit/post-fix-e752412/`. The v5 runner now accepts a named `--run` and refuses to overwrite a phase directory. No engine code changed in this observation pass.

## Observations

| Outcome | Baseline | Post-fix, after rechecks |
|---|---:|---:|
| Search and detail returned | 28 | 31 |
| Search error | 19 | 10 |
| Empty results, unclassified | 0 | 8 |
| Detail error | 3 | 1 |

These are not pass/fail counts: 24 of the 31 returned cases contain the query in a result title, and only 15 returned detail objects have a TOC URL. Neither heuristic proves usability. An empty list can be a legitimate outcome, stale rules, an auth/error response or another engine gap; the eight empties are **not** classified as successful or upstream failures.

- Former errors 5, 17, 26 and 35 now return search results and Book Info. Prior focused evidence ties these to the shared corrections; 26 still has no returned TOC URL.
- 42 now fails at Book Info with HTTP 502 on both initial and sequential attempts. This occurs before rule parsing; it is not evidence of a parser regression. The remote service's underlying cause was not established.
- Missing surfaces remain: 0 (`java.getWebViewUA`) and 25 (`JavaImporter`). Live recovery remains unproven.
- 14, 15 and 18 still report DNS failures. 24 lacks the required browser worker in this audit environment.
- 8 returns HTTP 502, 9 HTTP 500, and 40 HTTP 403; these statuses alone do not prove permanent source invalidity.
- 33 still has no list elements with a nonmatching configured URL pattern. Do not force the reference's conditional detail fallback.
- Empty observations: 6, 11, 13, 34, 36, 38, 39 and 41. Their HTTP 200 responses do not establish that the authored rules should produce a book.

## Bounded next approaches

**Recommend a feasibility check for the browser-UA contract first.** Legado's `getWebViewUA` returns the browser engine's default UA (`JsExtensions.kt:690–691`), not an arbitrary mobile-UA constant. NovelReader's worker health/protocol currently exposes no UA. A clean implementation would obtain the real browser-owned value through the existing worker boundary and define unavailable-worker behavior explicitly. Do not hardcode an Android identity or silently use an unrelated HTTP default. Establish live benefit before accepting a cross-worker implementation change.

**Keep Java imports separate.** Nine corpus definitions reference `JavaImporter`, with package references spanning Java language/utilities, crypto, IO, security, Android helpers and OkHttp. This inventory is not proof that every imported package is actually used. Sample 25's directly visible Java type references are Base64 and String; one statically decoded text block adds no identified crypto type references. A focused importer/string/byte conversion probe may therefore be smaller than a crypto bridge, but execution requirements and live validity are not yet established. Never implement a no-op importer, pretend arbitrary Java support exists, or rewrite the authored source to use NovelReader-specific helpers.

Alternatively, investigate the eight empty HTTP-200 observations against their authored rules and captured bodies before extending the bridge. This may expose reusable rule defects, but none is newly confirmed by this pass. No browser automation or source modifications were performed.
