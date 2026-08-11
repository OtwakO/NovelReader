import fs from 'node:fs';

const initial = JSON.parse(fs.readFileSync('/tmp/explore-v13-initial.json', 'utf8'));
const rerun = JSON.parse(fs.readFileSync('/tmp/explore-v13-rerun.json', 'utf8'));
const sequential = new Map(rerun.targets.map((entry) => [entry.rawIndex, entry.sequential]));
const diagnoses = {
  795: ['blocked_or_auth', 'The exact first category repeatedly returns Cloudflare HTTP 403. Direct HTTP with desktop/mobile User-Agents and a real Chromium render all show the explicit blocked page with no configured result cards.'],
  252: ['source_incomplete_or_invalid', 'The exact category redirects to a current Tencent catalog with thousands of rendered results, but the imported source has an empty ruleExplore object and therefore declares no executable result contract.'],
  883: ['upstream_dns', 'The exact host m.babahome.net has no usable address. Cloudflare and Google DNS-over-HTTPS both report SERVFAIL/refused authoritative delegation, so no response body exists.'],
  681: ['upstream_http', 'The exact first category repeatedly returns Cloudflare HTTP 521 (web server is down); DNS is healthy and no usable source body reaches parsing.'],
  73: ['source_incomplete_or_invalid', 'The imported category script uses invalid arrow-function destructuring such as map([title,b]=>...), confirmed by the production category_script_failed response and Node syntax checking.'],
  873: ['upstream_dns', 'The exact host www.frxsw.com has no usable address. Cloudflare and Google DNS-over-HTTPS both report SERVFAIL/refused authoritative delegation, so no response body exists.'],
  316: ['upstream_http', 'The exact first category repeatedly returns Cloudflare HTTP 522 (connection timed out); DNS is healthy and no usable source body reaches parsing.'],
  158: ['source_incomplete_or_invalid', 'A real Chromium session satisfies the JavaScript cookie gate and receives a current JSON response with 20 items, but the imported source has an empty ruleExplore object and therefore declares no executable result contract.'],
};

function compact(run) {
  if (!run) return null;
  const catalog = run.catalog;
  const page = run.page;
  return {
    catalog: {
      status: catalog.status,
      durationMs: catalog.durationMs,
      entryCount: catalog.body?.entries?.length ?? 0,
      error: catalog.status === 200 ? null : { code: catalog.body?.code, stage: catalog.body?.stage, severity: catalog.body?.severity, retryable: catalog.body?.retryable, message: catalog.body?.message },
    },
    category: run.category,
    page: page ? {
      status: page.status,
      durationMs: page.durationMs,
      books: page.bookCount,
      distinctBookUrls: page.distinctBookUrls,
      exhausted: page.exhausted,
      diagnostics: page.diagnostics,
      samples: page.sampleBooks,
      error: page.status === 200 ? null : { code: page.body?.code, stage: page.body?.stage, severity: page.body?.severity, retryable: page.body?.retryable, message: page.body?.message },
    } : null,
  };
}

const entries = initial.entries.map((entry) => {
  const authoritative = sequential.get(entry.rawIndex) ?? entry.initial;
  const run = compact(authoritative);
  const pass = run.page?.status === 200 && run.page.books > 0;
  const diagnosis = diagnoses[entry.rawIndex];
  const [classification, evidence] = pass
    ? ['credible_nonempty', `Authoritative production Explore returned ${run.page.books} books with ${run.page.distinctBookUrls} distinct book URLs${run.page.diagnostics?.length ? '; the replay retained books while explicitly reporting the received upstream status' : ''}.`]
    : diagnosis ?? [];
  if (!classification) throw new Error(`missing diagnosis ${entry.rawIndex}`);
  return { rawIndex: entry.rawIndex, bookSourceUrl: entry.sourceUrl, name: entry.name, rank: entry.rank, initial: compact(entry.initial), sequential: sequential.has(entry.rawIndex) ? compact(sequential.get(entry.rawIndex)) : null, classification, evidence };
});
const counts = {};
for (const entry of entries) counts[entry.classification] = (counts[entry.classification] ?? 0) + 1;
const credible = entries.filter((entry) => entry.classification === 'credible_nonempty');
const audit = {
  schemaVersion: 1,
  auditedAt: new Date().toISOString(),
  seed: initial.frozen.seed,
  corpus: initial.frozen.corpus,
  selection: initial.frozen.selection,
  execution: {
    method: 'Fresh disposable reader root; unmodified corpus imported through authenticated production API; catalog then first selectable URL category page 1.',
    initialConcurrency: 4,
    timeoutSeconds: 90,
    sequentialConfirmation: 'Every initial catalog/page failure, empty result, duplicate-only result, timeout, or diagnostic was rerun sequentially in the same isolated environment; sequential result is authoritative.',
    sourceOrParserChanges: false,
    setupNote: 'An initial harness attempt omitted the required Origin header and received uniform HTTP 403 before source execution. That invalid capture was discarded; the unchanged frozen sample was rerun with the authenticated cookie and canonical Origin.',
  },
  summary: {
    sampleSize: entries.length,
    credibleNonempty: credible.length,
    totalDistinctBooks: credible.reduce((total, entry) => total + (entry.sequential?.page?.distinctBookUrls ?? entry.initial.page?.distinctBookUrls ?? 0), 0),
    counts,
    sharedEngineGaps: [],
  },
  browserEvidence: {
    used: true,
    purpose: 'Distinguish browser-visible working content from Cloudflare blocking and a JavaScript cookie gate; browser execution was not used as an automatic fallback.',
    raw795: { url: 'https://www.3004ss.com/meinv/page/1', title: 'Attention Required! | Cloudflare', configuredCardCount: 0, conclusion: 'blocked_or_auth' },
    raw252: { requestedURL: 'https://m.ac.qq.com/category/listAll?type=tm&rank=upt&pageSize=30&page=1', finalURL: 'https://ac.qq.com/Comic/all?pageSize=30&type=tm&rank=upt&page=1', renderedResultClaim: '3811 results', conclusion: 'live content exists but imported ruleExplore is empty' },
    raw158: { url: 'https://www.hdzyk.com/inc/apijson.php?ac=detail&pg=1&t=5', gateCookie: 'ge_js_validator_228 (value redacted)', jsonCode: 1, jsonItems: 20, conclusion: 'live content exists but imported ruleExplore is empty' },
  },
  probes: {
    raw883: { host: 'm.babahome.net', cloudflareDNSStatus: 2, googleDNSStatus: 2, conclusion: 'lame/refused delegation' },
    raw873: { host: 'www.frxsw.com', cloudflareDNSStatus: 2, googleDNSStatus: 2, conclusion: 'lame/refused delegation' },
    raw681: { url: 'http://www.shuqi.cc/full/0_1.html', httpStatus: 521 },
    raw316: { url: 'https://m.woo15.com/top/lastupdate-1.html', httpStatus: 522 },
    raw73: { productionCode: 'category_script_failed', syntaxError: 'Malformed arrow function parameter list at map([title,b]=>...)' },
  },
  entries,
};
fs.writeFileSync('testdata/booksource/audits/explore/explore-live-audit-v13-2026-08-11.json', `${JSON.stringify(audit, null, 2)}\n`);
console.log(audit.summary);
