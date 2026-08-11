import crypto from 'node:crypto';
import fs from 'node:fs';

const initial = JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v2-initial.json', 'utf8'));
const rerunData = JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v2-rerun.json', 'utf8'));
const reruns = new Map(rerunData.entries.map((entry) => [entry.rawIndex, entry]));
const classifications = new Map(Object.entries({
  96: ['credible_search_and_detail', 'Production Search and Book Info both returned usable data.'],
  402: ['upstream_http', 'Sequential replay and direct HTTP both return HTTP 403 Forbidden from the authored search endpoint.'],
  233: ['credible_search_and_detail', 'Production Search and Book Info both returned usable data.'],
  179: ['search_engine_gap', 'The authored POST redirects to a live detail page. Search list extraction returns zero, while the final response URL and the same source Book Info rules enrich 凡人修仙传 successfully.'],
  49: ['detail_engine_gap', 'Search extracts two matching hrefs into one newline-concatenated bookUrl, so Book Info requests a malformed aggregate and fails. The first extracted href is live and enriches successfully.'],
  174: ['credible_search_and_detail', 'Production Search and Book Info both returned usable data.'],
  148: ['upstream_http', 'The live endpoint repeatedly redirects to the identical URL until the client reaches its redirect limit; direct HTTP reproduces the loop.'],
  413: ['credible_search_and_detail', 'Production Search and Book Info both returned usable data.'],
  16: ['credible_search_and_detail', 'Production Search and Book Info both returned usable data.'],
  378: ['credible_search_and_detail', 'Production Search and Book Info both returned usable data.'],
  155: ['credible_search_and_detail', 'Production Search and Book Info both returned usable data.'],
  396: ['deferred_webview', 'The source receives a JavaScript cookie challenge and explicitly calls java.webView before retrying its JSON API. WebView execution is outside the current regular/JS source target and is not promoted to a non-WebView engine gap.'],
  165: ['upstream_http', 'The live authored search endpoint returns HTTP 400 on replay; direct HTTP also fails.'],
  35: ['blocked_or_auth', 'Direct HTTP returns Cloudflare 403 and Chromium renders an explicit “Sorry, you have been blocked” page.'],
  126: ['legitimate_empty', 'The live HTTP 200 page contains the query text but no source-authored .col-lg-3 or .card-title result entries.'],
  152: ['upstream_timeout', 'The authored POST repeatedly times out before any HTTP response; DNS still resolves.'],
  99: ['blocked_or_auth', 'The live endpoint returns HTTP 503 with an explicit access-limit page.'],
  350: ['stale_source_contract', 'The source script assumes baseUrl contains word=<value>&. Baidu reorders the final URL so word is last; upstream Legado also exposes the final response URL as baseUrl, so the same authored regex fails there.'],
  111: ['blocked_or_auth', 'The live endpoint returns a Cloudflare HTTP 403 challenge page.'],
  224: ['site_drift', 'The current HTTP 200 page retains an old .mcon container but no longer contains the source-authored result field structure needed to form books.'],
  19: ['upstream_http', 'The live authored search endpoint consistently returns HTTP 400.'],
  55: ['credible_search_and_detail', 'Production Search and Book Info both returned usable data.'],
  306: ['legitimate_empty', 'The live JSON response explicitly reports 没有相关作品 with results: [].'],
  198: ['upstream_timeout', 'The authored endpoint repeatedly times out before any HTTP response; DNS still resolves.'],
  69: ['upstream_http', 'Sequential replay and direct HTTP both return HTTP 404 for the authored search route.'],
}).map(([index, value]) => [Number(index), value]));

const entries = initial.entries.map((entry) => {
  const classified = classifications.get(entry.rawIndex);
  if (!classified) throw new Error(`missing classification for raw ${entry.rawIndex}`);
  return {
    identity: { rawIndex: entry.rawIndex, bookSourceUrl: entry.bookSourceUrl },
    sourceName: entry.bookSourceName,
    classification: classified[0],
    rationale: classified[1],
    selectedResult: entry.selectedResult,
    initial: { search: entry.search, detail: entry.detail },
    sequentialReplay: reruns.get(entry.rawIndex) ?? null,
  };
});
const summary = entries.reduce((counts, entry) => {
  counts[entry.classification] = (counts[entry.classification] ?? 0) + 1;
  return counts;
}, {});
const evidence = {
  schemaVersion: 1,
  audit: 'Search → Book Info live audit v2',
  date: initial.frozen.date,
  scope: {
    query: initial.frozen.query,
    sampleSize: initial.frozen.selection.size,
    initialConcurrency: 4,
    searchPage: 1,
    detailSelection: 'first result with non-blank name and bookUrl',
    stopsAfterBookInfo: true,
    sourceOrParserChangesDuringSampling: false,
  },
  corpus: initial.frozen.corpus,
  exactImport: {
    path: 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v2-frozen-sources-2026-08-11.json',
    sha256: crypto.createHash('sha256').update(fs.readFileSync('testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v2-frozen-sources-2026-08-11.json')).digest('hex'),
    entries: initial.frozen.selection.size,
    identity: '(rawIndex, bookSourceUrl)',
    runtimeKey: 'bookSourceUrl',
  },
  selection: { ...initial.frozen.selection, seed: initial.frozen.seed },
  environment: {
    importedSources: initial.frozen.selection.size,
    productionPath: 'backend/internal/conformance using production analyzer/source executor/fingerprint transport',
    timeoutInitialSeconds: 30,
    timeoutReplaySeconds: 45,
  },
  references: {
    ruleDocumentation: 'https://mgz0227.github.io/The-tutorial-of-Legado/Rule/source.html',
    legadoRepository: 'reference/legado',
    relevantUpstreamContracts: [
      'BookList.getSearchItem extracts bookUrl with AnalyzeRule.getString(..., isUrl=true); Default URL extraction uses JSoup getString0 and retains the first value.',
      'BookList.analyzeBookList falls back to Book Info when Search list extraction is empty and bookUrlPattern is absent.',
      'WebBook.searchBookAwait passes the final response URL to BookList as baseUrl.',
    ],
  },
  summary,
  sharedGaps: [
    {
      id: 'list-url-first-value-semantics',
      stage: 'Search list field extraction → Book Info',
      confirmedWorkingImpact: [49],
      description: 'NovelReader evaluates bookUrl with generic multi-value string semantics and joins matching hrefs with newlines. Legado evaluates list bookUrl as a URL field and keeps the first extracted value.',
      evidence: 'Raw 49 Search produces two hrefs in one bookUrl and Book Info fails. Each individual href returns HTTP 200; using the first href in the production Book Info path succeeds and enriches the book.',
      proposedSharedSeam: 'Give URL-valued list fields first-value URL semantics while leaving ordinary text fields unchanged.',
    },
    {
      id: 'empty-search-list-detail-fallback',
      stage: 'Search response classification',
      confirmedWorkingImpact: [179],
      description: 'NovelReader treats an empty Search list as zero results. Legado attempts Book Info on the Search response when list extraction is empty and bookUrlPattern is absent.',
      evidence: 'Raw 179 POST redirects to a live detail URL. Search returns zero, but the final response URL with the same source Book Info rules succeeds and enriches 凡人修仙传.',
      proposedSharedSeam: 'After empty Search list extraction, attempt the existing Book Info parser against the final response URL/body under the upstream fallback conditions.',
    },
  ],
  observedButDeferred: [
    {
      id: 'java-webview-bridge',
      rawIndices: [396],
      reason: 'The source explicitly requires WebView to solve a JavaScript cookie challenge. WebView sources remain a deferred extension target, so this audit does not promote it as a regular/JS source regression.',
    },
  ],
  browserEvidence: [{ rawIndex: 35, purpose: 'Resolve whether Cloudflare HTTP 403 is bypassed by an ordinary browser.', result: 'Chromium rendered Attention Required and Sorry, you have been blocked.' }],
  entries,
};
fs.writeFileSync('testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v2-2026-08-11.json', `${JSON.stringify(evidence, null, 2)}\n`);

const rows = entries.map((entry) => `| ${entry.identity.rawIndex} | ${entry.sourceName.replaceAll('|', '\\|')} | \`${entry.classification}\` | ${entry.rationale.replaceAll('|', '\\|')} |`).join('\n');
const md = `# Search → Book Info live audit v2 — 2026-08-11

## Scope

- Corpus: \`${evidence.corpus.path}\`, SHA-256 \`${evidence.corpus.sha256}\`, ${evidence.corpus.entries} entries.
- Deterministic unrestricted sample: ${evidence.selection.size}, disjoint from the 25 v1 identities; ${evidence.selection.eligibleRemainingBeforeSelection} eligible identities remained before selection.
- Seed: \`${initial.frozen.seed}\`; identity: \`(rawIndex, bookSourceUrl)\`.
- Query: \`${evidence.scope.query}\`; page 1 only.
- Workflow: production Search, then Book Info for the first credible result; stop before TOC/content.
- Exactly the 25 frozen raw definitions were imported, preventing duplicate runtime keys from replacing sampled contracts.
- Initial concurrency: four; every non-pass replayed sequentially.
- No source or parser behavior changed during the audit.

## Result

- ${summary.credible_search_and_detail}/25 completed Search and Book Info with usable data.
- Two distinct shared compatibility gaps were confirmed against currently working live responses.
- One source explicitly depends on deferred WebView behavior.
- All other outcomes were upstream HTTP/timeouts, blocking, source/site drift, or legitimate empty results.

| Raw index | Source | Classification | Evidence-backed rationale |
|---:|---|---|---|
${rows}

## Confirmed shared gaps

### URL-valued Search fields must keep the first extracted value

Raw 49's broad \`a@href\` rule matches both the detail link and a chapter link. NovelReader joins them with a newline and passes the malformed aggregate to Book Info. Upstream Legado extracts Search \`bookUrl\` with URL semantics, whose Default/JSoup path keeps the first value.

Both individual hrefs are live HTTP 200 pages. Replaying Book Info with the first extracted href succeeds and enriches \`凡人修仙之仙界篇\`. The fix belongs at URL-valued list-field extraction—not in this source.

### Empty Search list must support detail-page fallback

Raw 179's authored POST redirects to \`https://www.23hh.la/book/3/3713/\`. Its list selector is empty because the response is already a detail page. NovelReader reports zero results. The same final URL and body succeed through the source's Book Info rules and enrich \`凡人修仙传\`.

Upstream Legado attempts Book Info after empty Search list extraction when \`bookUrlPattern\` is absent. NovelReader needs the equivalent shared fallback while preserving the existing final response URL/body and source session.

## Deferred rather than promoted

Raw 396 receives a JavaScript cookie challenge and explicitly calls \`java.webView\` before retrying its JSON API. This is genuine WebView-dependent behavior, but WebView execution remains a deferred extension target. Do not patch it as a regular JavaScript bridge method without the WebView architecture.

## Browser evidence

Playwright was used only for raw 35, where direct HTTP left a browser-bypass ambiguity. Chromium rendered Cloudflare's explicit blocked page, so the result remains \`blocked_or_auth\`.

## Recommendation

Approve two separate focused fix phases: first-value semantics for URL-valued Search/Explore fields, and empty-list Search-to-Book-Info fallback. Each should use a reduced public-boundary regression plus the corresponding frozen live source for post-fix evidence. Do not implement WebView support or patch sampled sources as part of either fix.
`;
fs.writeFileSync('testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v2-2026-08-11.md', md);
console.log(JSON.stringify(summary));
