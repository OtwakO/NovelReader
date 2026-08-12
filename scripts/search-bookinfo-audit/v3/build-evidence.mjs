import crypto from 'node:crypto';
import fs from 'node:fs';

const initial = JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v3-initial.json', 'utf8'));
const rerunData = JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v3-rerun.json', 'utf8'));
const reruns = new Map(rerunData.entries.map((entry) => [entry.rawIndex, entry]));
const direct = new Map(JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v3-direct.json', 'utf8')).map((entry) => [entry.rawIndex, entry]));
const browser = JSON.parse(JSON.parse(fs.readFileSync('/tmp/v3-playwright.json', 'utf8')).result);

const classifications = new Map(Object.entries({
  361: ['credible_search_and_detail', 'Production Search returned 20 books and Book Info enriched the selected result.'],
  371: ['upstream_timeout', 'The exact authored request timed out in sequential production replay and direct curl before any HTTP response.'],
  37: ['credible_search_and_detail', 'Production Search returned 10 books and Book Info enriched the selected result.'],
  418: ['upstream_timeout', 'The exact authored request timed out in sequential production replay and direct curl before any HTTP response.'],
  14: ['credible_search_and_detail', 'Production Search returned 15 books and Book Info enriched the selected result.'],
  130: ['deferred_webview', 'The source explicitly requests WebView behavior, outside the current regular/JavaScript transport target.'],
  375: ['shared_engine_gap', 'Standalone OnlyOne regex replacement preserved surrounding outer HTML, corrupting bookUrl and coverUrl. Changing only the two URL-field expressions to equivalent first-match replacement semantics recovered 20 Search results and successful Book Info.'],
  123: ['blocked_or_auth', 'The exact route returns Cloudflare HTTP 403; Chromium remains on the security-verification page.'],
  57: ['upstream_timeout', 'The exact authored request timed out in sequential production replay and direct curl before any HTTP response.'],
  414: ['deferred_webview', 'The source explicitly requests WebView behavior, outside the current regular/JavaScript transport target.'],
  158: ['upstream_timeout', 'The exact authored request repeatedly timed out after redirect before a usable response.'],
  349: ['credible_search_and_detail', 'Production Search returned one result and Book Info completed successfully.'],
  180: ['legitimate_empty', 'The exact GBK POST returns HTTP 200 but no source-authored result-list structure for the query.'],
  118: ['legitimate_empty', 'The live HTTP 200 response explicitly says no related content was found.'],
  102: ['upstream_transport', 'The exact authored HTTPS endpoint fails TLS negotiation in production and direct curl.'],
  340: ['upstream_http', 'The exact authored search route repeatedly returns HTTP 404.'],
  135: ['upstream_http', 'The exact authored search route repeatedly returns HTTP 400 with an empty body.'],
  355: ['blocked_or_auth', 'The exact Tieba route returns HTTP 403 and Chromium remains on 百度安全验证.'],
  160: ['credible_search_and_detail', 'Production Search returned 20 books and Book Info enriched the selected result.'],
  51: ['deferred_rhino_jvm', 'The source requires Rhino/JVM APIs including JavaImporter, Packages.javax.crypto, and Java byte arrays; regular goja execution cannot provide that JVM surface.'],
  27: ['credible_search_and_detail', 'Production Search returned 20 books and Book Info completed successfully.'],
  182: ['blocked_or_auth', 'The exact route returns Cloudflare HTTP 403; Chromium remains on the security-verification page.'],
  419: ['upstream_timeout', 'The exact authored request timed out in sequential production replay and direct curl before any HTTP response.'],
  129: ['credible_search_and_detail', 'Production Search returned 18 books and Book Info enriched the selected result.'],
  193: ['credible_search_and_detail', 'Production Search returned 11 books and Book Info enriched the selected result.'],
  364: ['shared_engine_gap', 'The declared GB2312 charset was applied to response decoding but not GET query encoding. Changing only the query bytes to declared GB2312 recovered 20 Search results and successful Book Info.'],
  192: ['upstream_http', 'The exact current route returns HTTP 403/redirect failure. Its inline <js> syntax is unsupported by NovelReader, but current content does not satisfy the full frozen workflow, so it is recorded separately rather than promoted as a recoverable gap.'],
  352: ['site_or_source_drift', 'The source script assumes final baseUrl contains word=<value>&, but the current final URL places word last; upstream Legado also exposes the final response URL.'],
  427: ['upstream_timeout', 'The exact authored request timed out in sequential production replay and direct curl before any HTTP response.'],
  6: ['deferred_webview', 'The source explicitly requests WebView behavior, outside the current regular/JavaScript transport target.'],
  201: ['site_or_source_drift', 'The multiline template JavaScript executes, but the current homepage no longer contains the expected action attribute, so the source script throws on match()[1].'],
  46: ['site_or_source_drift', 'The current bootstrap response no longer defines variables required by the frozen source program. NovelReader also lacks whole-<js> URL execution, recorded separately as unsupported sampled syntax rather than a recoverable primary gap.'],
  66: ['upstream_timeout', 'The exact authored request timed out in sequential production replay and direct curl before any HTTP response.'],
  78: ['legitimate_empty', 'The live HTTP 200 response is an information page stating that no search result was found.'],
  372: ['upstream_timeout', 'The exact authored request timed out in sequential production replay and direct curl before any HTTP response.'],
  13: ['credible_search_and_detail', 'Production Search returned 15 books and Book Info enriched the selected result.'],
  86: ['upstream_transport', 'The exact authored route repeats HTTP 301 redirects until the redirect limit is reached in production and direct curl.'],
  199: ['upstream_http', 'The exact authored Next.js data route repeatedly returns HTTP 400.'],
  354: ['blocked_or_auth', 'The exact Tieba route returns HTTP 403 and Chromium remains on 百度安全验证.'],
  108: ['upstream_timeout', 'The exact authored API request timed out in sequential production replay and direct curl.'],
  431: ['upstream_timeout', 'The exact authored API request timed out in sequential production replay and direct curl.'],
  395: ['credible_search_and_detail', 'Production Search returned 10 books and Book Info enriched the selected result.'],
  387: ['deferred_webview', 'The source explicitly requests WebView behavior, outside the current regular/JavaScript transport target.'],
  384: ['upstream_dns', 'The authored API hostname does not resolve in production or direct curl.'],
  373: ['upstream_timeout', 'The exact authored request timed out in sequential production replay and direct curl before any HTTP response.'],
  187: ['invalid_source_contract', 'The trailing URL-option text is malformed (`{"body": "id"="search-form"}`) and cannot represent a valid Legado request option object.'],
  405: ['upstream_timeout', 'The exact authored request timed out in sequential production replay and direct curl before any HTTP response.'],
  134: ['credible_search_and_detail', 'Production Search returned 20 books and Book Info enriched the selected result.'],
  47: ['deferred_rhino_jvm', 'The source requires Rhino/JVM JavaImporter after its jsLib is supplied. Search also omits source jsLib from URL evaluation, recorded separately as a compatibility observation without claiming workflow recovery.'],
  347: ['upstream_timeout', 'The exact authored request timed out in sequential production replay and direct curl before any HTTP response.'],
}).map(([raw, value]) => [Number(raw), value]));

const entries = initial.entries.map((entry) => {
  const classified = classifications.get(entry.rawIndex);
  if (!classified) throw new Error(`missing classification for raw ${entry.rawIndex}`);
  return {
    identity: { rawIndex: entry.rawIndex, bookSourceUrl: entry.bookSourceUrl },
    sourceName: entry.bookSourceName,
    classification: classified[0],
    rationale: classified[1],
    initial: { search: entry.search, selectedResult: entry.selectedResult, detail: entry.detail },
    sequentialReplay: reruns.get(entry.rawIndex) ?? null,
    directVerification: direct.get(entry.rawIndex) ?? null,
  };
});
const summary = entries.reduce((counts, entry) => {
  counts[entry.classification] = (counts[entry.classification] ?? 0) + 1;
  return counts;
}, {});
const exactPath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v3-frozen-sources-2026-08-12.json';
const raw375Search = JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v3-raw375-search.json', 'utf8'))[0];
const raw364Search = JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v3-raw364-search.json', 'utf8'))[0];
const counterfactuals = [
  {
    rawIndex: 375,
    changedSeam: 'Only the two standalone OnlyOne URL-field rules were replaced with equivalent first-match capture-and-replacement JavaScript; request, list/name rules, and Book Info rules remained frozen.',
    sourceSha256: crypto.createHash('sha256').update(JSON.stringify(raw375Search.rawSource)).digest('hex'),
    search: raw375Search,
    detail: JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v3-raw375-detail.json', 'utf8')),
  },
  {
    rawIndex: 364,
    changedSeam: 'Only the interpolated query bytes were changed from UTF-8 to the source-declared GB2312 representation; source rules and request options remained frozen.',
    sourceSha256: crypto.createHash('sha256').update(JSON.stringify(raw364Search.rawSource)).digest('hex'),
    search: raw364Search,
    detail: JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v3-raw364-detail.json', 'utf8')),
  },
];
const evidence = {
  schemaVersion: 1,
  audit: 'Search → Book Info live audit v3',
  date: initial.frozen.date,
  scope: {
    query: initial.frozen.query,
    sampleSize: 50,
    initialConcurrency: 4,
    searchPage: 1,
    detailSelection: 'first result with non-blank name and bookUrl',
    stopsAfterBookInfo: true,
    sourceOrParserChangesDuringSampling: false,
  },
  corpus: initial.frozen.corpus,
  exactImport: {
    path: exactPath,
    sha256: crypto.createHash('sha256').update(fs.readFileSync(exactPath)).digest('hex'),
    entries: 50,
    identity: '(rawIndex, bookSourceUrl)',
    runtimeKey: 'bookSourceUrl',
  },
  selection: { ...initial.frozen.selection, seed: initial.frozen.seed },
  environment: {
    importedSources: 50,
    productionPath: 'backend/internal/conformance using production analyzer/source executor/fingerprint transport',
    timeoutInitialSeconds: 30,
    timeoutReplaySeconds: 45,
  },
  references: {
    ruleDocumentation: 'https://mgz0227.github.io/The-tutorial-of-Legado/Rule/source.html',
    legadoRepository: 'reference/legado',
    relevantUpstreamContracts: [
      'AnalyzeUrl.initUrl executes <js> URL programs before URL and option parsing.',
      'Source-level jsLib is available to rule and URL JavaScript evaluation.',
      'OnlyOne regex rules return only the first matched-and-replaced value, not surrounding input.',
      'AnalyzeUrl encodes GET query parameters with the URL option charset when one is declared.',
    ],
  },
  summary,
  sharedGaps: [
    {
      id: 'regex-only-one-preserves-surrounding-input',
      stage: 'Search field regex evaluation',
      primaryAffectedRawIndices: [375],
      description: 'NovelReader OnlyOne regex replacement preserves text before and after the match; Legado returns only matcher.group(0) after replacement and returns empty when no match exists.',
      evidence: 'Raw 375 leaked the entire selected outer HTML into bookUrl and coverUrl. The captured counterfactual changes only those two URL-field expressions to equivalent first-match capture/replacement semantics and returns 20 Search results plus successful Book Info.',
      proposedSharedSeam: 'Correct the shared regex OnlyOne path to return only the first matched-and-replaced substring without changing replace-all rules.',
    },
    {
      id: 'get-query-option-charset',
      stage: 'Search request URL encoding',
      primaryAffectedRawIndices: [364],
      description: 'RequestSpec.Charset controls response decoding and POST bodies, but GET query interpolation is already UTF-8 encoded before the option charset can apply.',
      evidence: 'Raw 364 declares GB2312. NovelReader sends UTF-8 and gets no matches; the same frozen request encoded in GB2312 returns 20 matching books.',
      proposedSharedSeam: 'Apply declared URL-option charset to GET query variable encoding in the shared URL builder before RequestSpec construction.',
    },
  ],
  observedButDeferred: [
    { id: 'webview-transport', rawIndices: [6, 130, 387, 414], reason: 'These sources explicitly request WebView behavior and remain outside the current regular/JavaScript transport target.' },
    { id: 'rhino-jvm-interop', rawIndices: [47, 51], reason: 'These scripts require JavaImporter, Packages/javax.crypto, and/or Java byte-array semantics beyond ordinary goja JavaScript compatibility.' },
    { id: 'unsupported-js-url-syntax', rawIndices: [46, 51, 192], reason: 'NovelReader does not execute whole/inline <js> URL programs, but current sampled workflows also have independent site, HTTP, or Rhino/JVM blockers, so syntax impact is recorded without claiming recovery.' },
    { id: 'search-url-jslib-context', rawIndices: [47], reason: 'Search omits source jsLib from URL evaluation. Supplying it reaches a separate Rhino/JVM JavaImporter dependency, so this audit records the compatibility omission without promoting raw 47 as recoverable.' },
  ],
  browserEvidence: browser.map((item) => ({ rawIndex: item.raw, purpose: 'Determine whether an ordinary browser clears the captured security response.', status: item.status, finalUrl: item.finalUrl, title: item.title, result: item.body })),
  counterfactuals,
  entries,
};
const evidencePath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v3-2026-08-12.json';
fs.writeFileSync(evidencePath, `${JSON.stringify(evidence, null, 2)}\n`);

const rows = entries.map((entry) => `| ${entry.identity.rawIndex} | ${entry.sourceName.replaceAll('|', '\\|')} | \`${entry.classification}\` | ${entry.rationale.replaceAll('|', '\\|')} |`).join('\n');
const md = `# Search → Book Info live audit v3 — 2026-08-12

## Scope

- Corpus: \`${evidence.corpus.path}\`, SHA-256 \`${evidence.corpus.sha256}\`, ${evidence.corpus.entries} entries.
- Deterministic unrestricted sample: 50, disjoint from all 50 v1/v2 identities; ${evidence.selection.eligibleRemainingBeforeSelection} eligible identities remained before selection.
- Seed: \`${evidence.selection.seed}\`; identity: \`(rawIndex, bookSourceUrl)\`.
- Query: \`${evidence.scope.query}\`; page 1 only.
- Workflow: production Search, then Book Info for the first credible result; stop before TOC/content.
- Exactly the 50 frozen raw definitions were executed, preventing duplicate runtime keys from replacing sampled contracts.
- Initial concurrency: four; all 39 non-passes replayed sequentially.
- No source or parser behavior changed during the audit.

## Result

- ${summary.credible_search_and_detail}/50 completed Search and Book Info with usable data.
- ${summary.shared_engine_gap}/50 are primarily affected by two confirmed recoverable shared compatibility gaps.
- ${summary.deferred_webview}/50 explicitly require deferred WebView transport; one additional source is primarily a deferred Rhino/JVM dependency.
- Remaining outcomes are blocked/authentication, legitimate empty pages, upstream transport/DNS/HTTP failures, source/site drift, or an invalid source contract.

| Raw index | Source | Classification | Evidence-backed rationale |
|---:|---|---|---|
${rows}

## Confirmed recoverable shared gaps

### Correct OnlyOne regex replacement

Raw 375's standalone \`##pattern##replacement###\` rule should return only the first matched-and-replaced value. NovelReader preserves surrounding outer HTML and corrupts URL fields. A captured counterfactual changed only the two URL-field expressions to equivalent first-match capture/replacement semantics; Search returned 20 valid results and Book Info completed successfully.

### Honor URL-option charset for GET query encoding

Raw 364 declares GB2312. NovelReader decodes the response using that charset but sends the query in UTF-8. Sending the same query in GB2312 recovers 20 matching books. The fix belongs before RequestSpec construction in the shared URL builder.

## Deferred dependencies

Four sampled sources explicitly require WebView. Raws 47 and 51 require Rhino/JVM interoperability beyond regular goja support. Whole/inline \`<js>\` URL syntax is unsupported for raws 46/51/192, and Search omits \`jsLib\` for raw 47, but their current workflows also have independent site/HTTP/JVM blockers. These observations are preserved without claiming source recovery or counting them as primary shared-gap outcomes.

## Browser evidence

Playwright was used only for raws 123, 182, 354, and 355, where direct HTTP left browser-bypass ambiguity. Chromium remained on Cloudflare or Baidu security-verification pages for all four.

## Recommendation

Do not change production behavior inside this audit. Present the two confirmed recoverable shared gaps for approval, then implement them as focused shared-seam slices with deterministic public regressions and exact frozen-source post-fix reruns. Keep unsupported \`<js>\`, \`jsLib\`, WebView, and Rhino/JVM observations separate until a current satisfiable workflow proves recovery.
`;
fs.writeFileSync('testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v3-2026-08-12.md', md);
console.log(JSON.stringify(summary));
