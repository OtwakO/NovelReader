import crypto from 'node:crypto';
import fs from 'node:fs';

const initial = JSON.parse(fs.readFileSync('/tmp/explore-v14-initial.json', 'utf8'));
const rerun = JSON.parse(fs.readFileSync('/tmp/explore-v14-rerun.json', 'utf8'));
const corpus = JSON.parse(fs.readFileSync(initial.frozen.corpus.path, 'utf8'));
const sequential = new Map(rerun.targets.map((entry) => [entry.rawIndex, entry.sequential]));
const exactDefinitions = initial.frozen.selection.identities.map((entry) => corpus[entry.rawIndex]);
const exactImportBytes = `${JSON.stringify(exactDefinitions, null, 2)}\n`;
const exactImportSha256 = crypto.createHash('sha256').update(exactImportBytes).digest('hex');
const exactImportKeys = exactDefinitions.map((source) => source.bookSourceUrl);

const diagnoses = {
  669: ['engine_gap', 'The current body contains 60 configured book-li results that parse when bookUrlPattern filtering is disabled. NovelReader rejects them because the imported pattern requires the obsolete HTTP/underscore path shape; Legado does not apply bookUrlPattern per result.'],
  246: ['site_drift', 'The sampled Tencent category redirects to current desktop markup with 12 ret-search-item cards, while the imported fallback ruleSearch expects legacy comic-link/comic-title classes. A reduced parser probe confirms the captured current body does not satisfy that authored contract.'],
  297: ['source_incomplete_or_invalid', 'The effective Explore rule object has no bookList field. The sampled request also returns HTTP 444, but the empty authored result contract independently prevents Explore parsing.'],
  864: ['upstream_dns', 'The exact sampled host repeatedly resolves as SERVFAIL through local and public DNS-over-HTTPS checks, so no response body reaches parsing.'],
  329: ['upstream_http', 'The exact first selectable category request repeatedly returns HTTP 404; this finding is scoped to that sampled route.'],
  703: ['engine_gap', 'The current redirected body contains 15 c_row results that the imported selectors parse when bookUrlPattern filtering is disabled. NovelReader incorrectly filters each result by the stale qudushu.com pattern; Legado does not use bookUrlPattern as a per-result Search/Explore filter.'],
  80: ['engine_gap', 'The current body contains 30 configured result cards that parse when bookUrlPattern filtering is disabled. NovelReader rejects them because the imported pattern requires obsolete ?for-search text; Legado does not apply bookUrlPattern per result.'],
  609: ['upstream_dns', 'The exact sampled host is NXDOMAIN through local and public DNS checks, so no response body reaches parsing.'],
  488: ['upstream_http', 'The sampled route repeatedly returns HTTP 502. Its rules turn the error page title into one synthetic result named 502 Bad Gateway; that is not a credible book.'],
  707: ['site_drift', 'The exact sampled route redirects to a parked domain-sale page; the imported hot_sale selectors no longer describe the current response.'],
  67: ['site_drift', 'The current response does not contain the imported #ppluck list contract; detail fallback also fails, producing the repeatable result_rule_failed response.'],
  107: ['upstream_http', 'The exact frozen raw-index contract requests the sampled first category on jjjjxsw.com and repeatedly reaches a missing/redirected HTTP 404 route. This finding is scoped to that sampled route.'],
  83: ['source_incomplete_or_invalid', 'The current body contains tjxs entries, but the imported Explore bookList selects the enclosing ul.xbk as one item while field rules search descendant li fields in per-item XPath context, producing no complete results. Its malformed bookUrlPattern is not the sole cause: removing the filter still yields zero.'],
  136: ['source_incomplete_or_invalid', 'The imported Explore category JavaScript uses malformed arrow-function destructuring such as map([title,b]=>...), and production repeatedly returns category_script_failed before a request is made.'],
  775: ['source_incomplete_or_invalid', 'The effective Explore rule object has no bookList field. The current endpoint also presents a JavaScript redirect/challenge, but the empty authored result contract independently prevents parsing.'],
  648: ['source_incomplete_or_invalid', 'The imported category script yields no selectable URL category and the effective Explore rule object has no bookList. The current host is also a parked page.'],
  13: ['upstream_dns', 'The exact sampled host is NXDOMAIN through local and public DNS checks, so no response body reaches parsing.'],
  172: ['upstream_http', 'The exact first selectable category request repeatedly returns HTTP 404; this finding is scoped to that sampled route.'],
  467: ['source_incomplete_or_invalid', 'The imported category JavaScript uses malformed arrow-function destructuring and production repeatedly returns category_script_failed before a category request. The current API also rejects the legacy client metadata.'],
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
  const apparentPass = run.page?.status === 200 && run.page.books > 0;
  const diagnosis = diagnoses[entry.rawIndex];
  const [classification, evidence] = diagnosis ?? (apparentPass
    ? ['credible_nonempty', `Authoritative production Explore returned ${run.page.books} books with ${run.page.distinctBookUrls} distinct book URLs.`]
    : []);
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
    method: 'Fresh disposable reader root; exact 50 frozen raw-index definitions imported unchanged through authenticated production API; catalog then first selectable URL category page 1.',
    exactImport: { sourceCount: exactDefinitions.length, uniqueStorageKeys: new Set(exactImportKeys).size, sha256: exactImportSha256 },
    initialConcurrency: 4,
    timeoutSeconds: 90,
    sequentialConfirmation: 'Every initial catalog/page failure, empty result, duplicate-only result, timeout, or diagnostic was rerun sequentially in the same isolated exact-contract environment; sequential result is authoritative.',
    sourceOrParserChanges: false,
    correctionNote: 'A discarded preliminary run imported the whole compilation. Because bookSourceUrl is the storage key, duplicate URLs replaced frozen raws 107 and 126. The authoritative run imported only the exact frozen definitions; raw 126 then returned 10 books. No preliminary outcomes are used in classification.',
  },
  summary: {
    sampleSize: entries.length,
    credibleNonempty: credible.length,
    totalDistinctBooks: credible.reduce((total, entry) => total + (entry.sequential?.page?.distinctBookUrls ?? entry.initial.page?.distinctBookUrls ?? 0), 0),
    counts,
    sharedEngineGaps: [{
      id: 'book-url-pattern-result-filter',
      seam: 'Search/Explore result parsing',
      affectedRawIndices: [669, 703, 80],
      finding: 'NovelReader discards each parsed result whose resolved book URL does not match bookUrlPattern. Upstream Legado uses bookUrlPattern for Search final-detail detection/manual URL association, not as a per-result Search/Explore filter.',
      counterfactual: { raw669: { filtered: 0, unfiltered: 60 }, raw703: { filtered: 0, unfiltered: 15 }, raw80: { filtered: 0, unfiltered: 30 } },
      implementationDirection: 'Remove bookUrlPattern filtering from the shared parsed-result path and protect Search final-detail detection separately; do not branch on source identity.',
    }],
  },
  browserEvidence: { used: false, purpose: 'Direct DNS/HTTP, captured-body comparison, upstream Legado inspection, and reduced production-runtime probes resolved all ambiguities; no rendered browser state was needed.' },
  probes: {
    raw669: { capturedBody: '/tmp/v14exact-669.html', configuredListCount: 60, filteredResults: 0, unfilteredResults: 60 },
    raw703: { capturedBody: '/tmp/v14exact-703.html', configuredListCount: 15, filteredResults: 0, unfilteredResults: 15 },
    raw80: { capturedBody: '/tmp/v14exact-80.html', configuredListCount: 30, filteredResults: 0, unfilteredResults: 30 },
    raw246: { finalURL: 'https://ac.qq.com/Comic/all?rank=upt&page=1&pageSize=30&type=tm', currentResultClass: 'ret-search-item', currentResultCount: 12, importedResultClass: 'comic-link', importedClassCount: 0 },
    raw83: { configuredListContainerPresent: true, unfilteredResults: 0, conclusion: 'bookUrlPattern is not sole cause' },
    malformedCategoryScripts: [136, 467],
    dnsFailures: [864, 609, 13],
  },
  entries,
};
fs.writeFileSync('testdata/booksource/audits/explore/explore-live-audit-v14-2026-08-11.json', `${JSON.stringify(audit, null, 2)}\n`);
fs.writeFileSync('testdata/booksource/audits/explore/explore-live-audit-v14-frozen-sources-2026-08-11.json', exactImportBytes);
console.log(audit.summary);
