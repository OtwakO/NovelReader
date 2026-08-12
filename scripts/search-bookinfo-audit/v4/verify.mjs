import crypto from 'node:crypto';
import fs from 'node:fs';

const evidencePath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v4-2026-08-12.json';
const evidence = JSON.parse(fs.readFileSync(evidencePath, 'utf8'));
const corpusBytes = fs.readFileSync(evidence.corpus.path);
const corpus = JSON.parse(corpusBytes);
const corpusDigest = crypto.createHash('sha256').update(corpusBytes).digest('hex');
if (corpusDigest !== evidence.corpus.sha256) throw new Error(`corpus SHA mismatch: ${corpusDigest}`);
if (evidence.entries.length !== 50 || evidence.scope.sampleSize !== 50) throw new Error('sample size mismatch');

const priorPaths = [
  'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v1-2026-08-11.json',
  'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v2-2026-08-11.json',
  'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v3-2026-08-12.json',
];
const priorKeys = new Set();
for (const path of priorPaths) {
  const prior = JSON.parse(fs.readFileSync(path, 'utf8'));
  for (const entry of prior.entries) priorKeys.add(`${entry.identity.rawIndex}\0${entry.identity.bookSourceUrl}`);
}
if (priorKeys.size !== 100 || evidence.selection.excludedIdentityCount !== 100) throw new Error('prior exclusion mismatch');

const eligible = corpus.flatMap((source, rawIndex) => {
  const sourceUrl = String(source.bookSourceUrl ?? '');
  const identity = `${rawIndex}\0${sourceUrl}`;
  const rules = source.ruleSearch;
  if (source.enabled === false || Number(source.bookSourceType ?? 0) !== 0 || !String(source.searchUrl ?? '').trim() || priorKeys.has(identity)) return [];
  if (rules == null || (typeof rules === 'string' && !rules.trim()) || (typeof rules === 'object' && Object.keys(rules).length === 0)) return [];
  const rank = crypto.createHash('sha256').update(`${evidence.selection.seed}\0${rawIndex}\0${sourceUrl}`).digest('hex');
  return [{ rawIndex, bookSourceUrl: sourceUrl, rank }];
}).sort((left, right) => left.rank.localeCompare(right.rank));
if (eligible.length !== evidence.selection.eligibleRemainingBeforeSelection) throw new Error(`eligible count mismatch: ${eligible.length}`);
const expected = eligible.slice(0, 50);
if (JSON.stringify(expected.map((entry) => entry.rawIndex)) !== JSON.stringify(evidence.selection.rawIndices)) throw new Error('deterministic ranking changed');

const exactBytes = fs.readFileSync(evidence.exactImport.path);
const exactDigest = crypto.createHash('sha256').update(exactBytes).digest('hex');
if (exactDigest !== evidence.exactImport.sha256) throw new Error('exact import SHA mismatch');
const exactSources = JSON.parse(exactBytes);
if (exactSources.length !== 50) throw new Error('exact import count mismatch');
if (new Set(exactSources.map((source) => source.bookSourceUrl)).size !== 50) throw new Error('runtime storage keys are not unique');

const expectedSummary = {
  credible_search_and_detail: 9,
  upstream_timeout: 20,
  upstream_http: 4,
  legitimate_empty: 3,
  blocked_or_auth: 3,
  deferred_webview: 3,
  site_or_source_drift: 3,
  invalid_source_contract: 2,
  upstream_dns: 1,
  upstream_transport: 1,
  deferred_rhino_jvm: 1,
};
const seen = new Set();
const computedSummary = {};
let replayCount = 0;
const outputFor = (entry) => entry.sequentialReplay?.search?.output?.[0] ?? entry.initial?.search?.output?.[0] ?? null;
const includesWebViewCall = (source) => JSON.stringify(source ?? {}).includes('java.webView');
for (const [position, entry] of evidence.entries.entries()) {
  const key = `${entry.identity.rawIndex}\0${entry.identity.bookSourceUrl}`;
  if (seen.has(key) || priorKeys.has(key)) throw new Error(`duplicate/prior identity ${key}`);
  seen.add(key);
  const selected = expected[position];
  if (selected.rawIndex !== entry.identity.rawIndex || selected.bookSourceUrl !== entry.identity.bookSourceUrl) throw new Error(`selection mismatch position ${position}`);
  const source = corpus[entry.identity.rawIndex];
  if (!source || source.bookSourceUrl !== entry.identity.bookSourceUrl) throw new Error(`corpus identity mismatch raw ${entry.identity.rawIndex}`);
  if (JSON.stringify(exactSources[position]) !== JSON.stringify(source)) throw new Error(`exact definition mismatch raw ${entry.identity.rawIndex}`);
  computedSummary[entry.classification] = (computedSummary[entry.classification] ?? 0) + 1;
  const output = outputFor(entry);
  if (!output) throw new Error(`missing production output raw ${entry.identity.rawIndex}`);
  if (entry.classification === 'credible_search_and_detail') {
    if (entry.initial?.search?.output?.[0]?.classification !== 'success') throw new Error(`credible Search not successful raw ${entry.identity.rawIndex}`);
    if (!entry.initial?.selectedResult?.name || !entry.initial?.selectedResult?.bookUrl) throw new Error(`credible selected result incomplete raw ${entry.identity.rawIndex}`);
    if (entry.initial?.detail?.output?.classification !== 'success' || !entry.initial?.detail?.output?.detail?.name) throw new Error(`credible Book Info not successful raw ${entry.identity.rawIndex}`);
  } else {
    const replay = entry.sequentialReplay;
    if (!replay) throw new Error(`missing sequential replay raw ${entry.identity.rawIndex}`);
    if (replay.rawIndex !== entry.identity.rawIndex || replay.bookSourceUrl !== entry.identity.bookSourceUrl || replay.frozenIndex !== position) throw new Error(`sequential replay identity mismatch raw ${entry.identity.rawIndex}`);
    if (output.identity?.index !== position || output.identity?.bookSourceUrl !== entry.identity.bookSourceUrl) throw new Error(`replay output identity mismatch raw ${entry.identity.rawIndex}`);
    replayCount++;
  }
  switch (entry.classification) {
    case 'upstream_timeout':
      if (output.classification !== 'transport_timeout') throw new Error(`timeout outcome changed raw ${entry.identity.rawIndex}`);
      break;
    case 'upstream_http':
      if (output.classification !== 'http_or_waf_failure' || Number(output.response?.statusCode ?? 0) < 400) throw new Error(`HTTP outcome changed raw ${entry.identity.rawIndex}`);
      break;
    case 'upstream_dns':
      if (output.classification !== 'transport_dns_failure') throw new Error(`DNS outcome changed raw ${entry.identity.rawIndex}`);
      break;
    case 'upstream_transport':
      if (output.classification !== 'transport_failure') throw new Error(`transport outcome changed raw ${entry.identity.rawIndex}`);
      break;
    case 'deferred_webview':
      if (output.classification !== 'unsupported_webview' && !includesWebViewCall(output.rawSource) && !/webView/.test(String(output.error ?? ''))) throw new Error(`WebView contract missing raw ${entry.identity.rawIndex}`);
      break;
    case 'deferred_rhino_jvm':
      if (!/xGorgon is not defined/.test(String(output.error ?? '')) || !/JavaImporter/.test(String(output.rawSource?.jsLib ?? ''))) throw new Error(`Rhino/JVM evidence changed raw ${entry.identity.rawIndex}`);
      break;
    case 'invalid_source_contract':
      if (!['js_or_request_build_failure', 'transport_failure'].includes(output.classification)) throw new Error(`invalid-contract outcome changed raw ${entry.identity.rawIndex}`);
      break;
  }
}
const canonicalObject = (value) => JSON.stringify(Object.fromEntries(Object.entries(value).sort()));
if (replayCount !== 41) throw new Error(`expected 41 sequential replays, got ${replayCount}`);
if (canonicalObject(computedSummary) !== canonicalObject(evidence.summary)) throw new Error('summary does not match entries');
if (canonicalObject(computedSummary) !== canonicalObject(expectedSummary)) throw new Error('headline histogram changed');
if (!Array.isArray(evidence.sharedEngineGaps) || evidence.sharedEngineGaps.length !== 0) throw new Error('v4 must not claim a recoverable shared gap');
if (evidence.entries.some((entry) => entry.classification === 'shared_engine_gap')) throw new Error('unexpected shared_engine_gap classification');

const timeoutRaws = evidence.entries.filter((entry) => entry.classification === 'upstream_timeout').map((entry) => entry.identity.rawIndex).sort((a, b) => a - b);
if (timeoutRaws.length !== 20) throw new Error('timeout set changed');
for (const raw of timeoutRaws) {
  const entry = evidence.entries.find((item) => item.identity.rawIndex === raw);
  const direct = entry.directVerification;
  const output = outputFor(entry);
  if (!direct || direct.rawIndex !== raw || direct.code !== 28) throw new Error(`missing direct curl timeout raw ${raw}`);
  if (String(direct.method).toUpperCase() !== String(output.request?.method ?? 'GET').toUpperCase()) throw new Error(`direct timeout method mismatch raw ${raw}`);
  const requested = new URL(output.request.url);
  const tested = new URL(direct.url);
  if (requested.host !== tested.host && !String(direct.summary ?? '').includes(requested.host)) throw new Error(`direct timeout URL mismatch raw ${raw}`);
  if (!/timed out|timeout/i.test(`${direct.stderr ?? ''} ${direct.summary ?? ''}`)) throw new Error(`direct timeout marker missing raw ${raw}`);
}

const browserByRaw = new Map(evidence.browserEvidence.map((item) => [item.rawIndex, item]));
for (const raw of [62, 79, 146, 360]) {
  const browser = browserByRaw.get(raw);
  const entry = evidence.entries.find((item) => item.identity.rawIndex === raw);
  const output = outputFor(entry);
  if (!browser || browser.method !== output.request.method) throw new Error(`missing structured browser evidence raw ${raw}`);
  if (new URL(browser.url).host !== new URL(output.request.url).host) throw new Error(`browser tested wrong host raw ${raw}`);
  if (raw === 62 && (browser.statusCode !== 403 || !/security verification|Cloudflare/i.test(browser.bodyMarker ?? ''))) throw new Error('raw 62 browser boundary changed');
  if (raw === 79 && (browser.statusCode !== 200 || !/app\/DefaultGwd|guwendao\.net/.test(browser.finalUrl ?? '') || !/APP/.test(browser.bodyMarker ?? ''))) throw new Error('raw 79 browser redirect changed');
  if (raw === 146 && (browser.statusCode !== 200 || !/需要登录/.test(`${browser.title ?? ''} ${browser.bodyMarker ?? ''}`))) throw new Error('raw 146 browser auth boundary changed');
  if (raw === 360 && browser.statusCode !== 202) throw new Error('raw 360 browser status changed');
}
const webViewContract = evidence.sourceContractEvidence?.find((item) => item.rawIndex === 362);
if (!webViewContract?.sourceRuleContainsRequirement || webViewContract.requirement !== 'java.webView' || !/webView/.test(webViewContract.productionError ?? '') || webViewContract.productionStatusCode !== 202) throw new Error('raw 362 WebView contract evidence changed');

if (!Array.isArray(evidence.compatibilityObservations) || evidence.compatibilityObservations.length !== 1) throw new Error('expected one compatibility observation');
const observation = evidence.compatibilityObservations[0];
if (observation.id !== 'java-encode-uri-charset-argument' || JSON.stringify(observation.affectedRawIndices) !== JSON.stringify([72])) throw new Error('raw 72 compatibility observation changed');
const raw72 = evidence.entries.find((entry) => entry.identity.rawIndex === 72);
if (raw72?.classification !== 'legitimate_empty') throw new Error('raw 72 must remain a legitimate empty, not a shared gap');
const raw72Output = outputFor(raw72);
const frozen72 = corpus[72];
if (!String(frozen72.searchUrl ?? '').includes("java.encodeURI(key,'gb2312')")) throw new Error('raw 72 frozen charset-aware bridge call missing');
if (!/%E5%87%A1%E4%BA%BA%E4%BF%AE%E4%BB%99%E4%BC%A0/i.test(raw72Output.request?.url ?? '')) throw new Error('raw 72 production UTF-8 bytes missing');
const counter = observation.counterfactual;
if (counter.statusCode !== 200 || counter.classification !== 'rule_mismatch' || counter.extractedCount !== 0 || !counter.decodedQueryPresent || !counter.explicitZeroResult) throw new Error('raw 72 counterfactual no longer proves a corrected but empty response');
if (!/%B7%B2%C8%CB%D0%DE%CF%C9%B4%AB/i.test(counter.requestUrl ?? '')) throw new Error('raw 72 GB2312 bytes missing');
if (counter.requestUrl === raw72Output.request?.url || new URL(counter.requestUrl).origin !== new URL(raw72Output.request.url).origin) throw new Error('raw 72 counterfactual changed more than query bytes');
if (!/java\.encodeURI\(str, enc\)/.test(observation.description ?? '') || !/UTF-8-only/.test(observation.description ?? '')) throw new Error('raw 72 bridge-signature evidence weakened');
if (!/still yields an explicit zero-result/i.test(observation.conclusion ?? '') || evidence.sharedEngineGaps.length !== 0) throw new Error('raw 72 non-recovering boundary weakened');

console.log(`verified ${evidence.entries.length} disjoint exact identities; corpus ${corpusDigest}; exact import ${exactDigest}; ${replayCount} sequential replays; zero recoverable gaps; one non-recovering bridge observation`);
