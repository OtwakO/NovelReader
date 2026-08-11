import crypto from 'node:crypto';
import fs from 'node:fs';

const evidencePath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v2-2026-08-11.json';
const evidence = JSON.parse(fs.readFileSync(evidencePath, 'utf8'));
const corpusBytes = fs.readFileSync(evidence.corpus.path);
const corpus = JSON.parse(corpusBytes);
const digest = crypto.createHash('sha256').update(corpusBytes).digest('hex');
if (digest !== evidence.corpus.sha256) throw new Error(`corpus SHA mismatch: ${digest}`);
if (evidence.entries.length !== 25 || evidence.scope.sampleSize !== 25) throw new Error('sample size mismatch');

const prior = JSON.parse(fs.readFileSync('testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v1-2026-08-11.json', 'utf8'));
const priorKeys = new Set(prior.entries.map((entry) => `${entry.identity.rawIndex}\0${entry.identity.bookSourceUrl}`));
if (priorKeys.size !== evidence.selection.excludedIdentityCount) throw new Error('excluded identity count mismatch');
const eligible = corpus.flatMap((source, rawIndex) => {
  const sourceUrl = String(source.bookSourceUrl ?? '');
  const identity = `${rawIndex}\0${sourceUrl}`;
  if (source.enabled === false || Number(source.bookSourceType ?? 0) !== 0 || !String(source.searchUrl ?? '').trim() || priorKeys.has(identity)) return [];
  const rules = source.ruleSearch;
  if (rules == null || (typeof rules === 'string' && !rules.trim()) || (typeof rules === 'object' && Object.keys(rules).length === 0)) return [];
  const rank = crypto.createHash('sha256').update(`${evidence.selection.seed}\0${rawIndex}\0${sourceUrl}`).digest('hex');
  return [{ rawIndex, bookSourceUrl: sourceUrl, rank }];
}).sort((left, right) => left.rank.localeCompare(right.rank));
if (eligible.length !== evidence.selection.eligibleRemainingBeforeSelection) throw new Error('eligible count mismatch');
const expected = eligible.slice(0, evidence.scope.sampleSize);
if (JSON.stringify(expected.map((entry) => entry.rawIndex)) !== JSON.stringify(evidence.selection.rawIndices)) throw new Error('deterministic ranking changed');

const exactBytes = fs.readFileSync(evidence.exactImport.path);
const exactDigest = crypto.createHash('sha256').update(exactBytes).digest('hex');
if (exactDigest !== evidence.exactImport.sha256) throw new Error('exact import SHA mismatch');
const exactSources = JSON.parse(exactBytes);
if (exactSources.length !== 25) throw new Error('exact import count mismatch');
if (new Set(exactSources.map((source) => source.bookSourceUrl)).size !== exactSources.length) throw new Error('runtime storage keys are not unique');

const seen = new Set();
for (const [position, entry] of evidence.entries.entries()) {
  const key = `${entry.identity.rawIndex}\0${entry.identity.bookSourceUrl}`;
  if (seen.has(key)) throw new Error(`duplicate identity ${key}`);
  if (priorKeys.has(key)) throw new Error(`v1 overlap ${key}`);
  seen.add(key);
  const selected = expected[position];
  if (selected.rawIndex !== entry.identity.rawIndex || selected.bookSourceUrl !== entry.identity.bookSourceUrl) throw new Error(`selection mismatch position ${position}`);
  const source = corpus[entry.identity.rawIndex];
  if (!source || source.bookSourceUrl !== entry.identity.bookSourceUrl) throw new Error(`corpus identity mismatch raw ${entry.identity.rawIndex}`);
  if (JSON.stringify(exactSources[position]) !== JSON.stringify(source)) throw new Error(`exact definition mismatch raw ${entry.identity.rawIndex}`);
  if (entry.classification !== 'credible_search_and_detail' && !entry.sequentialReplay) throw new Error(`missing replay raw ${entry.identity.rawIndex}`);
}

const summaryCount = Object.values(evidence.summary).reduce((sum, count) => sum + count, 0);
if (summaryCount !== 25) throw new Error('summary count mismatch');
if (evidence.sharedGaps.length !== 2) throw new Error('expected two shared gaps');
for (const gap of evidence.sharedGaps) {
  if (!gap.confirmedWorkingImpact.length || !gap.evidence || !gap.proposedSharedSeam) throw new Error(`incomplete shared gap ${gap.id}`);
}
const urlGap = evidence.sharedGaps.find((gap) => gap.id === 'default-jsoup-book-url-first-value-semantics');
if (!urlGap?.confirmedWorkingImpact.includes(49)) throw new Error('raw 49 gap evidence missing');
if (!urlGap.description.includes('Default/JSoup') || !urlGap.proposedSharedSeam.includes('XPath/JSONPath')) throw new Error('raw 49 gap is not mode-scoped');
const responseURLGap = evidence.sharedGaps.find((gap) => gap.id === 'empty-search-book-url-response-url-fallback');
if (!responseURLGap?.confirmedWorkingImpact.includes(179)) throw new Error('raw 179 gap evidence missing');
if (!responseURLGap.description.includes('final response baseUrl') || !responseURLGap.proposedSharedSeam.includes('final response URL')) throw new Error('raw 179 gap is not response-URL scoped');
if (!evidence.observedButDeferred.some((item) => item.id === 'java-webview-bridge' && item.rawIndices.includes(396))) throw new Error('WebView deferral missing');
if (!evidence.browserEvidence.some((item) => item.rawIndex === 35)) throw new Error('browser evidence missing');
console.log(`verified ${evidence.entries.length} disjoint exact identities; corpus ${digest}; exact import ${exactDigest}; ${summaryCount} classified; ${evidence.sharedGaps.length} shared gaps`);
