import crypto from 'node:crypto';
import fs from 'node:fs';

const initial = JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v4-initial.json', 'utf8'));
const rerunData = JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v4-rerun.json', 'utf8'));
const reruns = new Map(rerunData.entries.map((entry) => [entry.rawIndex, entry]));
const direct = new Map(JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v4-direct.json', 'utf8')).map((entry) => [entry.rawIndex, entry]));
const browserGet = JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v4-browser-get.json', 'utf8'));
const browserPost = JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v4-browser-post.json', 'utf8'));
const browser = [...browserGet, browserPost];
const raw72Counter = JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v4-raw72-counter-result.json', 'utf8'))[0];

const groups = {
  credible_search_and_detail: [166, 425, 374, 38, 186, 168, 217, 106, 120],
  upstream_timeout: [138, 388, 420, 409, 428, 181, 103, 113, 424, 407, 433, 132, 154, 345, 394, 18, 163, 140, 435, 403],
  upstream_http: [74, 157, 125, 76],
  legitimate_empty: [221, 178, 72],
  blocked_or_auth: [146, 62, 360],
  deferred_webview: [362, 127, 177],
  site_or_source_drift: [79, 351, 195],
  invalid_source_contract: [327, 329],
  upstream_dns: [324],
  upstream_transport: [88],
  deferred_rhino_jvm: [1],
};

const rationale = {
  credible_search_and_detail: 'Production Search returned usable results and Book Info enriched the selected result.',
  upstream_timeout: 'The exact authored request timed out in sequential production replay and in a bounded direct-request cross-check.',
  upstream_http: 'The exact authored route persistently returned an upstream HTTP failure in production and direct replay.',
  legitimate_empty: 'The exact request reached a valid query response but produced no source-authored results for this query.',
  blocked_or_auth: 'The exact request reached a security or login boundary rather than a usable search result page.',
  deferred_webview: 'The exact source explicitly requires WebView/browser behavior outside the regular HTTP/JavaScript transport path.',
  site_or_source_drift: 'The current live page or response schema no longer satisfies assumptions encoded by the frozen source rules.',
  invalid_source_contract: 'The frozen source script or URL construction is internally invalid under standard JavaScript/URL semantics.',
  upstream_dns: 'The exact authored hostname does not resolve.',
  upstream_transport: 'The exact authored endpoint fails at the transport/protocol boundary.',
  deferred_rhino_jvm: 'The source library requires Rhino/JVM facilities such as JavaImporter before its URL function can be defined.',
};

const classificationByRaw = new Map();
for (const [classification, raws] of Object.entries(groups)) {
  for (const raw of raws) {
    if (classificationByRaw.has(raw)) throw new Error(`duplicate classification raw ${raw}`);
    classificationByRaw.set(raw, classification);
  }
}

const entries = initial.entries.map((entry) => {
  const classification = classificationByRaw.get(entry.rawIndex);
  if (!classification) throw new Error(`missing classification raw ${entry.rawIndex}`);
  return {
    identity: { rawIndex: entry.rawIndex, bookSourceUrl: entry.bookSourceUrl },
    sourceName: entry.bookSourceName,
    classification,
    rationale: entry.rawIndex === 72
      ? 'The production bridge ignored the frozen gb2312 argument and sent UTF-8; changing only that bridge result fixed the query text, but the live page still explicitly returned zero records.'
      : rationale[classification],
    initial: { search: entry.search, selectedResult: entry.selectedResult, detail: entry.detail },
    sequentialReplay: reruns.get(entry.rawIndex) ?? null,
    directVerification: direct.get(entry.rawIndex) ?? null,
  };
});

const summary = entries.reduce((counts, entry) => {
  counts[entry.classification] = (counts[entry.classification] ?? 0) + 1;
  return counts;
}, {});

const exactPath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v4-frozen-sources-2026-08-12.json';
const counterBody = String(raw72Counter.response?.bodySample ?? '');
const compatibilityObservations = [
  {
    id: 'java-encode-uri-charset-argument',
    stage: 'Search URL JavaScript bridge',
    affectedRawIndices: [72],
    description: 'Legado exposes java.encodeURI(str, enc), but NovelReader currently exposes UTF-8-only java.encodeURI(str); the frozen source requests gb2312 explicitly.',
    counterfactual: {
      changedSeam: 'Only the value returned by java.encodeURI(key, "gb2312") was replaced with the GB2312 percent-encoded bytes Legado documents; all other frozen source fields remained unchanged.',
      requestUrl: raw72Counter.request?.url,
      finalUrl: raw72Counter.response?.finalUrl,
      statusCode: raw72Counter.response?.statusCode,
      classification: raw72Counter.classification,
      extractedCount: raw72Counter.extracted?.length ?? 0,
      decodedQueryPresent: counterBody.includes('凡人修仙传'),
      explicitZeroResult: /共有\s*<B>0<\/B>项|未查询到符合条件的记录/.test(counterBody),
    },
    conclusion: 'The bridge omission is genuine, but correcting only the charset argument still yields an explicit zero-result page, so this audit does not count raw 72 as a recoverable shared-gap outcome.',
    proposedSharedSeam: 'Add the optional charset argument to the existing java.encodeURI bridge using the shared charset encoder; do not couple it to WebView or Rhino/JVM support.',
  },
];

const evidence = {
  schemaVersion: 1,
  audit: 'Search → Book Info live audit v4',
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
    directTimeoutCrossCheckSeconds: 15,
  },
  references: {
    ruleDocumentation: 'https://mgz0227.github.io/The-tutorial-of-Legado/Rule/source.html',
    legadoRepository: 'reference/legado',
    relevantUpstreamContract: 'java.encodeURI(str, enc) accepts an optional charset and URL-encodes with that charset.',
  },
  summary,
  sharedEngineGaps: [],
  compatibilityObservations,
  browserEvidence: browser,
  sourceContractEvidence: [
    {
      rawIndex: 362,
      requirement: 'java.webView',
      sourceRuleContainsRequirement: String(initial.entries.find((entry) => entry.rawIndex === 362)?.search?.output?.[0]?.rawSource?.ruleSearch?.bookList ?? '').includes('java.webView'),
      productionError: initial.entries.find((entry) => entry.rawIndex === 362)?.search?.output?.[0]?.error,
      productionStatusCode: initial.entries.find((entry) => entry.rawIndex === 362)?.search?.output?.[0]?.response?.statusCode,
    },
  ],
  entries,
};

const evidencePath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v4-2026-08-12.json';
fs.writeFileSync(evidencePath, `${JSON.stringify(evidence, null, 2)}\n`);

const rows = entries.map((entry) => `| ${entry.identity.rawIndex} | ${entry.sourceName.replaceAll('|', '\\|')} | \`${entry.classification}\` | ${entry.rationale.replaceAll('|', '\\|')} |`).join('\n');
const md = `# Search → Book Info live audit v4 — 2026-08-12

## Scope

- Corpus: \`${evidence.corpus.path}\`, SHA-256 \`${evidence.corpus.sha256}\`, ${evidence.corpus.entries} entries.
- Deterministic unrestricted sample: 50, disjoint from all 100 v1–v3 identities; ${evidence.selection.eligibleRemainingBeforeSelection} eligible identities remained before selection.
- Seed: \`${evidence.selection.seed}\`; identity: \`(rawIndex, bookSourceUrl)\`.
- Query: \`${evidence.scope.query}\`; page 1 only.
- Workflow: production Search, then Book Info for the first credible result; stop before TOC/content.
- Exactly the 50 frozen raw definitions were executed, preventing duplicate runtime keys from replacing sampled contracts.
- Initial concurrency: four; all 41 non-passes replayed sequentially.
- No source or parser behavior changed during the audit.

## Result

- ${summary.credible_search_and_detail}/50 completed Search and Book Info with usable data.
- 0/50 is an audit-proven recoverable shared-engine gap.
- ${summary.upstream_timeout}/50 persistently timed out; ${summary.upstream_http}/50 returned persistent HTTP failures.
- ${summary.deferred_webview}/50 explicitly require WebView and ${summary.deferred_rhino_jvm}/50 requires Rhino/JVM interoperability.
- Remaining outcomes are legitimate empties, blocked/authentication boundaries, site/source drift, invalid source contracts, DNS, or transport failures.

| Raw index | Source | Classification | Evidence-backed rationale |
|---:|---|---|---|
${rows}

## Non-recovering compatibility observation

Raw 72 calls \`java.encodeURI(key, 'gb2312')\`. The rulebook and Legado implementation support the charset argument, while NovelReader currently ignores it and emits UTF-8. Replacing only that bridge result with the correct GB2312 bytes fixes the server-side mojibake, but the exact page still reports zero matching records. This proves a genuine bridge omission without proving recovery for the sampled query.

Recommendation: document and separately approve a small shared bridge fix adding the optional charset argument to the existing \`java.encodeURI\` binding. Do not count raw 72 as a recovered source and do not mix this with WebView or Rhino/JVM work.

## Browser evidence

Playwright was used only for four exact requests where direct HTTP left browser behavior ambiguous: raw 62 remained Cloudflare-blocked; raw 79 redirected to the app-download page; raw 146 remained login-gated; and raw 360 stayed at Qidian's security boundary. Raw 362 was not counted as a browser run: its frozen source contract explicitly invokes \`java.webView\`, and production reports the missing WebView bridge against the same Qidian probe response.

## Recommendation

Do not change production behavior inside this audit. There is no recoverable shared-gap outcome to fix from v4. Present the charset-aware \`java.encodeURI\` compatibility omission separately and obtain approval before implementing it.
`;
fs.writeFileSync('testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v4-2026-08-12.md', md);
console.log(JSON.stringify(summary));
