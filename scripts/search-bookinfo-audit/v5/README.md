# Production-path engine audit (v5)

Accepted scope and current findings: [audit plan](../../../docs/plans/booksource-engine-compatibility-audit.md).

Unlike v1–v4, this run calls production `book.Searcher.SearchInstalledSource` and `GetBookInfoForBookContext`, including dynamic headers and normal request construction. It does not use the older conformance runner's reconstructed search path. Each source runs in a separate process with fresh state; this is **not** an authenticated batch/concurrency test.

From the repository root:

```sh
python3 scripts/search-bookinfo-audit/v5/freeze.py
(cd backend && go build -o ../test-booksources/engine-audit/engine-audit ./cmd/engine-audit)
python3 scripts/search-bookinfo-audit/v5/run.py initial
python3 scripts/search-bookinfo-audit/v5/run.py replay
python3 scripts/search-bookinfo-audit/v5/run.py scrutiny
```

The input is the user-selected `test-booksources/new_test_booksource.json`. Selection is fixed at 50 enabled text sources and query `凡人修仙传`; the private manifest records hashes and ranking. Do not regenerate the sample after inspecting failures. `replay` runs initial non-passes sequentially; `scrutiny` sequentially rechecks apparent successes lacking the query in names or collapsing multiple results to one book URL. Neither heuristic is a final classification.

**All raw evidence stays in ignored `test-booksources/engine-audit/`.** The command's JSON and stderr can contain complete requests, credentials, source expressions and website content. Never redirect them into tracked directories or commit them. Inspect and sanitize any derived report; prefer sample indices and hashes over real URLs.

Optional `--webview <endpoint>` supplies a disposable worker. A missing worker is an infrastructure limitation, not an engine gap. The v5 initial/replay run had no worker; sample 24 was separately replayed with an isolated local worker and private `browser-24.json` evidence. Do not reuse an installed account's login/browser state.

Limitations: no account/device identity hydration or all-source shared runtime; main HTTP/WebView exchanges are captured, JS-internal AJAX is not; response previews cap at 1 MiB. Returned detail metadata is not automatically a credible pass because some fields may be inherited from search. Initial evidence predates forwarding `wordCount`/`updateTime` into the detail candidate; the identified counterfactuals do not depend on those fields. Private temporary counterfactual probes are documented in the plan, not part of default tests. Re-running a phase replaces that phase's private observations; preserve earlier evidence before a post-fix run.
