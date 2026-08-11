import crypto from 'node:crypto';
import fs from 'node:fs';

const auditDir = 'testdata/booksource/audits/explore';
const audit = JSON.parse(fs.readFileSync(`${auditDir}/explore-live-audit-v14-2026-08-11.json`, 'utf8'));
const frozenBytes = fs.readFileSync(`${auditDir}/explore-live-audit-v14-frozen-sources-2026-08-11.json`);
const frozenSources = JSON.parse(frozenBytes);
const corpusBytes = fs.readFileSync(audit.corpus.path);
const corpus = JSON.parse(corpusBytes);
if (crypto.createHash('sha256').update(corpusBytes).digest('hex') !== audit.corpus.sha256) throw new Error('corpus hash mismatch');
if (audit.entries.length !== 50 || audit.selection.rawIndices.length !== 50 || frozenSources.length !== 50) throw new Error('sample size mismatch');
const keys = audit.entries.map((entry) => `${entry.rawIndex}\0${entry.bookSourceUrl}`);
if (new Set(keys).size !== 50) throw new Error('duplicate identities');
if (new Set(frozenSources.map((source) => source.bookSourceUrl)).size !== 50) throw new Error('duplicate runtime storage keys');
if (crypto.createHash('sha256').update(frozenBytes).digest('hex') !== audit.execution.exactImport.sha256) throw new Error('exact import hash mismatch');
if (audit.execution.exactImport.sourceCount !== 50 || audit.execution.exactImport.uniqueStorageKeys !== 50) throw new Error('exact import metadata mismatch');
for (let i = 0; i < audit.entries.length; i++) {
  const entry = audit.entries[i];
  const source = corpus[entry.rawIndex];
  if (entry.bookSourceUrl !== source.bookSourceUrl || frozenSources[i].bookSourceUrl !== entry.bookSourceUrl || JSON.stringify(frozenSources[i]) !== JSON.stringify(source)) throw new Error(`frozen definition mismatch ${entry.rawIndex}`);
}
if (audit.entries.some((entry) => !entry.classification || !entry.evidence)) throw new Error('missing classification/evidence');
const needsSequential = (entry) => entry.initial.catalog.status !== 200 || !entry.initial.page || entry.initial.page.status !== 200 || entry.initial.page.books === 0 || entry.initial.page.distinctBookUrls === 0 || entry.initial.page.diagnostics?.length;
if (audit.entries.filter(needsSequential).some((entry) => !entry.sequential)) throw new Error('missing sequential confirmation');
const counts = {};
for (const entry of audit.entries) counts[entry.classification] = (counts[entry.classification] ?? 0) + 1;
if (JSON.stringify(counts) !== JSON.stringify(audit.summary.counts)) throw new Error('summary counts mismatch');
const books = audit.entries.filter((entry) => entry.classification === 'credible_nonempty').reduce((total, entry) => total + (entry.sequential?.page?.distinctBookUrls ?? entry.initial.page?.distinctBookUrls ?? 0), 0);
if (books !== audit.summary.totalDistinctBooks) throw new Error(`book total mismatch ${books}`);
const prior = new Set();
for (const name of audit.selection.priorManifests) {
  const manifest = JSON.parse(fs.readFileSync(`${auditDir}/${name}`, 'utf8'));
  for (const entry of manifest.entries ?? []) prior.add(`${entry.rawIndex}\0${entry.bookSourceUrl ?? entry.sourceUrl}`);
}
if (prior.size !== 600) throw new Error(`prior identity count ${prior.size}`);
if (keys.some((key) => prior.has(key))) throw new Error('sample overlaps prior audit');
const eligible = corpus.flatMap((source, rawIndex) => {
  const sourceUrl = source.bookSourceUrl;
  const key = `${rawIndex}\0${sourceUrl}`;
  if (source.enabled === false || !String(source.exploreUrl ?? '').trim() || prior.has(key)) return [];
  return [{ rawIndex, sourceUrl, name: source.bookSourceName, rank: crypto.createHash('sha256').update(`${audit.seed}\0${rawIndex}\0${sourceUrl}`).digest('hex') }];
}).sort((a, b) => a.rank.localeCompare(b.rank));
if (eligible.length !== 121 || eligible.length !== audit.selection.eligibleRemainingBeforeSelection) throw new Error(`eligible count mismatch ${eligible.length}`);
const expected = eligible.slice(0, 50);
if (JSON.stringify(expected) !== JSON.stringify(audit.selection.identities)) throw new Error('ranked identities mismatch');
if (JSON.stringify(expected.map((entry) => entry.rawIndex)) !== JSON.stringify(audit.selection.rawIndices)) throw new Error('ranked raw indices mismatch');
if (JSON.stringify(expected.map((entry) => `${entry.rawIndex}\0${entry.sourceUrl}`)) !== JSON.stringify(keys)) throw new Error('entry order mismatch');
const expectedCounts = { site_drift: 3, credible_nonempty: 31, engine_gap: 3, source_incomplete_or_invalid: 6, upstream_dns: 3, upstream_http: 4 };
if (JSON.stringify(audit.summary.counts) !== JSON.stringify(expectedCounts)) throw new Error(`unexpected classifications ${JSON.stringify(audit.summary.counts)}`);
const gap = audit.summary.sharedEngineGaps.find((item) => item.id === 'book-url-pattern-result-filter');
if (!gap || JSON.stringify(gap.affectedRawIndices) !== JSON.stringify([669, 703, 80])) throw new Error('shared gap evidence missing');
for (const raw of [669, 703, 80]) if (audit.entries.find((entry) => entry.rawIndex === raw)?.classification !== 'engine_gap') throw new Error(`gap classification ${raw}`);
if (audit.entries.find((entry) => entry.rawIndex === 488)?.classification !== 'upstream_http') throw new Error('synthetic 502 book counted as credible');
if (audit.entries.find((entry) => entry.rawIndex === 126)?.classification !== 'credible_nonempty') throw new Error('exact raw 126 did not supersede overwritten run');
if (audit.browserEvidence?.used) throw new Error('unexpected browser evidence');
console.log(`v14 evidence valid: 50 unique, 600 excluded, ${books} distinct credible books, 1 shared engine gap across 3 identities`);
