import crypto from 'node:crypto';
import fs from 'node:fs';

const evidencePath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v3-2026-08-12.json';
const evidence = JSON.parse(fs.readFileSync(evidencePath, 'utf8'));
const corpusBytes = fs.readFileSync(evidence.corpus.path);
const corpus = JSON.parse(corpusBytes);
const corpusDigest = crypto.createHash('sha256').update(corpusBytes).digest('hex');
if (corpusDigest !== evidence.corpus.sha256) throw new Error(`corpus SHA mismatch: ${corpusDigest}`);
if (evidence.entries.length !== 50 || evidence.scope.sampleSize !== 50) throw new Error('sample size mismatch');

const priorPaths = [
  'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v1-2026-08-11.json',
  'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v2-2026-08-11.json',
];
const priorKeys = new Set();
for (const path of priorPaths) {
  const prior = JSON.parse(fs.readFileSync(path, 'utf8'));
  for (const entry of prior.entries) priorKeys.add(`${entry.identity.rawIndex}\0${entry.identity.bookSourceUrl}`);
}
if (priorKeys.size !== 50 || evidence.selection.excludedIdentityCount !== 50) throw new Error('prior exclusion mismatch');

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

const seen = new Set();
let replayCount = 0;
const computedSummary = {};
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
  if (entry.classification !== 'credible_search_and_detail') {
    if (!entry.sequentialReplay) throw new Error(`missing replay raw ${entry.identity.rawIndex}`);
    replayCount++;
  }
}
if (replayCount !== 39) throw new Error(`expected 39 sequential replays, got ${replayCount}`);
if (JSON.stringify(Object.fromEntries(Object.entries(computedSummary).sort())) !== JSON.stringify(Object.fromEntries(Object.entries(evidence.summary).sort()))) throw new Error('summary histogram does not match entries');
if (evidence.summary.credible_search_and_detail !== 11 || evidence.summary.shared_engine_gap !== 2) throw new Error('headline totals changed');

if (evidence.sharedGaps.length !== 2) throw new Error('expected two confirmed shared gaps');
const gaps = new Map(evidence.sharedGaps.map((gap) => [gap.id, gap]));
const expectedGapRaws = new Map([
  ['regex-only-one-preserves-surrounding-input', [375]],
  ['get-query-option-charset', [364]],
]);
for (const [id, raws] of expectedGapRaws) {
  const gap = gaps.get(id);
  if (!gap?.evidence || !gap.proposedSharedSeam || JSON.stringify(gap.primaryAffectedRawIndices) !== JSON.stringify(raws)) throw new Error(`incomplete shared gap ${id}`);
  for (const raw of raws) {
    const entry = evidence.entries.find((item) => item.identity.rawIndex === raw);
    if (entry?.classification !== 'shared_engine_gap') throw new Error(`gap raw ${raw} is not classified as shared_engine_gap`);
  }
}
const classifiedGapRaws = evidence.entries.filter((entry) => entry.classification === 'shared_engine_gap').map((entry) => entry.identity.rawIndex).sort((a, b) => a - b);
if (JSON.stringify(classifiedGapRaws) !== JSON.stringify([364, 375])) throw new Error('unexpected shared gap classifications');

for (const raw of [123, 182, 354, 355]) {
  const browser = evidence.browserEvidence.find((item) => item.rawIndex === raw);
  if (!browser || browser.status !== 403 || !/verification|验证/i.test(`${browser.title} ${browser.result}`)) throw new Error(`browser evidence missing raw ${raw}`);
}

const canonicalHash = (value) => crypto.createHash('sha256').update(JSON.stringify(value)).digest('hex');
const counters = new Map(evidence.counterfactuals.map((item) => [item.rawIndex, item]));
for (const raw of [364, 375]) {
  const counter = counters.get(raw);
  if (!counter || counter.sourceSha256 !== canonicalHash(counter.search.rawSource)) throw new Error(`counterfactual source hash mismatch raw ${raw}`);
  if (counter.search.classification !== 'success' || counter.search.extracted.length !== 20 || counter.detail.classification !== 'success') throw new Error(`counterfactual did not recover Search and Book Info raw ${raw}`);
  if (!counter.search.request?.url || !counter.search.response?.finalUrl) throw new Error(`counterfactual request/response missing raw ${raw}`);
  const frozenPosition = evidence.entries.findIndex((entry) => entry.identity.rawIndex === raw);
  const frozen = exactSources[frozenPosition];
  const changed = counter.search.rawSource;
  if (raw === 375) {
    const frozenCopy = structuredClone(frozen);
    frozenCopy.ruleSearch.bookUrl = changed.ruleSearch.bookUrl;
    frozenCopy.ruleSearch.coverUrl = changed.ruleSearch.coverUrl;
    if (JSON.stringify(frozenCopy) !== JSON.stringify(changed)) throw new Error('raw 375 counterfactual changed more than two URL-field rules');
    if (counter.search.extracted[0].bookUrl !== 'https://m.qidian.com/book/107580/') throw new Error('raw 375 transformed URL mismatch');
  } else {
    const frozenCopy = structuredClone(frozen);
    frozenCopy.searchUrl = changed.searchUrl;
    if (JSON.stringify(frozenCopy) !== JSON.stringify(changed)) throw new Error('raw 364 counterfactual changed more than Search query representation');
    if (!/%B7%B2%C8%CB%D0%DE%CF%C9%B4%AB/i.test(changed.searchUrl)) throw new Error('raw 364 GB2312 bytes missing');
  }
}

if (!evidence.observedButDeferred.some((item) => item.id === 'webview-transport' && item.rawIndices.length === 4)) throw new Error('WebView deferral missing');
if (!evidence.observedButDeferred.some((item) => item.id === 'rhino-jvm-interop' && item.rawIndices.includes(47) && item.rawIndices.includes(51))) throw new Error('JVM deferral missing');
if (!evidence.observedButDeferred.some((item) => item.id === 'unsupported-js-url-syntax' && [46, 51, 192].every((raw) => item.rawIndices.includes(raw)))) throw new Error('unsupported <js> observation missing');
if (!evidence.observedButDeferred.some((item) => item.id === 'search-url-jslib-context' && item.rawIndices.includes(47))) throw new Error('jsLib observation missing');

console.log(`verified ${evidence.entries.length} disjoint exact identities; corpus ${corpusDigest}; exact import ${exactDigest}; ${replayCount} sequential replays; ${evidence.sharedGaps.length} confirmed recoverable gaps`);
