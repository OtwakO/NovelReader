import fs from 'node:fs';

const initial = JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v1-initial.json', 'utf8'));
const rerunData = JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v1-rerun.json', 'utf8'));
const reruns = new Map(rerunData.entries.map((entry) => [entry.rawIndex, entry]));
const classifications = new Map(Object.entries({
  4: ['site_drift', 'HTTP 200 search page no longer contains the source-authored .novel_cell contract; direct body inspection finds current chapter links but zero .novel_cell nodes.'],
  40: ['credible_search_and_detail', 'Production search and Book Info both returned usable data.'],
  101: ['stale_source_contract', 'The imported source itself says search is invalid; its generated API request now receives HTTP 403.'],
  406: ['stale_source_contract', 'The authored SearchResults.aspx endpoint redirects to HTTPS and returns the site’s HTTP 404 resource-not-found page.'],
  133: ['credible_search_and_detail', 'Production search and Book Info both returned usable data.'],
  80: ['stale_source_contract', 'The imported source explicitly marks search invalid; the current redirected search page has none of its authored result selectors.'],
  408: ['credible_search_and_detail', 'Production search and Book Info both returned usable data.'],
  173: ['credible_search_and_detail', 'Production search and Book Info both returned usable data.'],
  53: ['upstream_http', 'The authored POST reaches the live endpoint, which consistently returns HTTP 500 with an empty body.'],
  190: ['site_drift', 'The request reaches a redirected replacement domain, whose HTTP 200 response asks for a search term instead of returning the authored .lis dl result shape.'],
  267: ['detail_engine_gap', 'Search returns live results. The source-authored bookUrl suffix ,{Cookie:"xmanhua_lang=2"} is valid under Legado lenient URL-option parsing, but NovelReader percent-encodes it into the path and gets 404; fetching the URL portion with that cookie returns HTTP 200 detail HTML.'],
  22: ['credible_search_and_detail', 'Production search and Book Info both returned usable data.'],
  392: ['legitimate_empty', 'The live API returns HTTP 200 with code 1055 and “没有更多小说了” for the frozen query; $.items is absent because the source reports no results.'],
  323: ['blocked_or_auth', 'The live search endpoint consistently returns HTTP 403 with a Baidu security-verification page.'],
  97: ['blocked_or_auth', 'The live endpoint consistently returns HTTP 503 with an explicit access-limit page.'],
  170: ['credible_search_and_detail', 'Production search and Book Info both returned usable data.'],
  184: ['upstream_dns', 'The source hostname consistently fails DNS resolution in the audit environment.'],
  56: ['credible_search_and_detail', 'Production search and Book Info both returned usable data.'],
  145: ['credible_search_and_detail', 'Production search and Book Info both returned usable data.'],
  415: ['credible_search_and_detail', 'Production search and Book Info both returned usable data.'],
  151: ['upstream_http', 'The source uses a valid lenient URL-option POST contract that NovelReader does not parse, but a direct correctly formed POST to the live endpoint returns HTTP 500; this sample cannot establish a working-source failure.'],
  92: ['stale_source_contract', 'The source domain now returns an HTTP 404 domain-parking page.'],
  137: ['credible_search_and_detail', 'Production search and Book Info both returned usable data.'],
  50: ['blocked_or_auth', 'The source declares a login UI and its computed live API returns HTTP 401 “游客没有权限执行此操作”. NovelReader also does not execute whole-URL <js> rules, but anonymous live behavior cannot establish a working-source failure.'],
  164: ['upstream_http', 'The authored POST consistently times out before any HTTP response.'],
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
  audit: 'Search → Book Info live audit v1',
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
  selection: initial.frozen.selection,
  environment: {
    dataRoot: 'backend/data (fresh disposable root, removed after evidence capture)',
    importedSources: 458,
    administratorAccounts: 1,
    productionPath: 'backend/internal/conformance using production analyzer/source executor/fingerprint transport',
    timeoutInitialSeconds: 30,
    timeoutReplaySeconds: 45,
  },
  references: {
    ruleDocumentation: 'https://mgz0227.github.io/The-tutorial-of-Legado/Rule/source.html',
    legadoRepository: 'https://github.com/Luoyacheng/legado',
    legadoRevision: '44e07fea541287804cc58d0168940a756cd11cfd',
    relevantUpstreamContract: 'AnalyzeUrl executes @js and <js> before URL parsing, then parses strict JSON URL options with a lenient Gson fallback.',
  },
  summary,
  sharedGaps: [{
    id: 'lenient-url-option-object-keys',
    stage: 'Book Info URL construction',
    confirmedWorkingImpact: [267],
    additionalAffectedButNotWorkingProof: [151],
    description: 'NovelReader recognizes strict JSON and single-quoted URL options but not Legado’s lenient unquoted object keys. The option suffix remains URL text instead of request metadata.',
    evidence: 'Raw 267 live search returns detail links ending ,{Cookie:"xmanhua_lang=2"}; NovelReader requests an encoded-suffix path and gets 404, while the URL portion plus cookie returns HTTP 200 detail HTML.',
  }],
  observedButUnconfirmedSyntaxGaps: [{
    id: 'whole-url-js-block',
    rawIndices: [50],
    description: 'NovelReader percent-encodes a whole searchUrl <js>…</js> block instead of executing it as AnalyzeUrl does.',
    reasonNotEngineGap: 'The source requires login and its anonymous computed API returns HTTP 401, so this batch does not prove a currently working source is broken by the syntax gap.',
  }],
  entries,
};
const jsonPath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v1-2026-08-11.json';
fs.writeFileSync(jsonPath, `${JSON.stringify(evidence, null, 2)}\n`);

const rows = entries.map((entry) => `| ${entry.identity.rawIndex} | ${entry.sourceName.replaceAll('|', '\\|')} | \`${entry.classification}\` | ${entry.rationale.replaceAll('|', '\\|')} |`).join('\n');
const md = `# Search → Book Info live audit v1 — 2026-08-11

## Scope

- Corpus: \`${evidence.corpus.path}\`, SHA-256 \`${evidence.corpus.sha256}\`, ${evidence.corpus.entries} entries.
- Eligible identities: ${evidence.selection.eligibleBeforeSelection}; deterministic unrestricted sample: ${evidence.selection.size}.
- Seed: \`${evidence.selection.ranking}\` with \`${initial.frozen.seed}\`.
- Query: \`${evidence.scope.query}\`; page 1 only.
- Workflow: production search, then Book Info only for the first result with non-blank name and book URL.
- Initial concurrency: four; all 15 non-passes replayed sequentially.
- Fresh disposable data root; all 458 unmodified corpus entries imported.
- No source/parser behavior changed during sampling.

## Result

- 10/25 completed both Search and Book Info with usable data.
- 1 confirmed shared engine gap affected a currently working source at Book Info.
- 14 outcomes were upstream, blocked/authenticated, stale/drifted contracts, or a legitimate empty result.

| Raw index | Source | Classification | Evidence-backed rationale |
|---:|---|---|---|
${rows}

## Confirmed shared gap

### Lenient URL-option object keys

Legado’s documented URL contract permits a trailing request option object, and upstream \`AnalyzeUrl\` parses strict JSON first then falls back to lenient Gson. Raw 267’s live search returns valid detail links ending:

\`,{Cookie:"xmanhua_lang=2"}\`

NovelReader leaves that object in the path and percent-encodes it, producing a detail 404. A direct request to the URL portion with the declared cookie returns HTTP 200 and current detail HTML. This is a reusable URL-parser seam, not an xmanhua-specific patch opportunity.

Raw 151 uses the same unsupported lenient form for a POST search URL, but the correctly formed live POST currently returns HTTP 500. It supports the shared syntax diagnosis but is not counted as proof of a working-source failure.

## Observed but not promoted to an engine gap

Raw 50 uses a whole-URL \`<js>…</js>\` search rule. Upstream \`AnalyzeUrl\` executes it before URL parsing; NovelReader currently percent-encodes it. The source declares login requirements and its computed anonymous API returns HTTP 401, so this batch cannot establish that the syntax gap broke a currently working source. Preserve the observation for a future valid live fixture; do not fix from this source alone.

## Recommendation

Approve one focused TDD fix for lenient trailing URL-option object keys at the shared URL builder, using reduced fixtures plus raw 267 as targeted live verification. Do not bundle whole-URL \`<js>\` support until a valid working source or deterministic upstream-contract priority justifies it. Do not patch any sampled source contract.
`;
fs.writeFileSync('testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v1-2026-08-11.md', md);
console.log(JSON.stringify(summary));
