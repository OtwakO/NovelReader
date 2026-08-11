import crypto from 'node:crypto';
import fs from 'node:fs';

const auditDir = 'testdata/booksource/audits/explore';
const path = `${auditDir}/explore-live-audit-v13-2026-08-11.json`;
const audit = JSON.parse(fs.readFileSync(path, 'utf8'));
const corpusBytes = fs.readFileSync(audit.corpus.path);
if (crypto.createHash('sha256').update(corpusBytes).digest('hex') !== audit.corpus.sha256) throw new Error('corpus hash mismatch');
if (audit.entries.length !== 50 || audit.selection.rawIndices.length !== 50) throw new Error('sample size mismatch');
const keys = audit.entries.map((entry) => `${entry.rawIndex}\0${entry.bookSourceUrl}`);
if (new Set(keys).size !== 50) throw new Error('duplicate identities');
if (audit.entries.some((entry) => !entry.classification || !entry.evidence)) throw new Error('missing classification/evidence');
const needsSequential = (entry) => entry.initial.catalog.status !== 200 || !entry.initial.page || entry.initial.page.status !== 200 || entry.initial.page.books === 0 || entry.initial.page.distinctBookUrls === 0 || entry.initial.page.diagnostics?.length;
if (audit.entries.filter(needsSequential).some((entry) => !entry.sequential)) throw new Error('missing sequential confirmation');
const counts = {};
for (const entry of audit.entries) counts[entry.classification] = (counts[entry.classification] ?? 0) + 1;
if (JSON.stringify(counts) !== JSON.stringify(audit.summary.counts)) throw new Error('summary counts mismatch');
const books = audit.entries.filter((entry) => entry.classification === 'credible_nonempty').reduce((total, entry) => total + (entry.sequential?.page?.distinctBookUrls ?? entry.initial.page?.distinctBookUrls ?? 0), 0);
if (books !== audit.summary.totalDistinctBooks || books !== 1211) throw new Error(`book total mismatch ${books}`);
const prior = new Set();
for (const name of audit.selection.priorManifests) {
  const manifest = JSON.parse(fs.readFileSync(`${auditDir}/${name}`, 'utf8'));
  for (const entry of manifest.entries ?? []) prior.add(`${entry.rawIndex}\0${entry.bookSourceUrl ?? entry.sourceUrl}`);
}
if (prior.size !== 550) throw new Error(`prior identity count ${prior.size}`);
if (keys.some((key) => prior.has(key))) throw new Error('sample overlaps prior audit');
const corpus = JSON.parse(corpusBytes);
for (const entry of audit.entries) if (entry.bookSourceUrl !== corpus[entry.rawIndex].bookSourceUrl) throw new Error(`identity mismatch ${entry.rawIndex}`);
const eligible = corpus.flatMap((source, rawIndex) => {
  const sourceUrl = source.bookSourceUrl;
  const key = `${rawIndex}\0${sourceUrl}`;
  if (source.enabled === false || !String(source.exploreUrl ?? '').trim() || prior.has(key)) return [];
  return [{ rawIndex, sourceUrl, name: source.bookSourceName, rank: crypto.createHash('sha256').update(`${audit.seed}\0${rawIndex}\0${sourceUrl}`).digest('hex') }];
}).sort((a, b) => a.rank.localeCompare(b.rank));
if (eligible.length !== 171 || eligible.length !== audit.selection.eligibleRemainingBeforeSelection) throw new Error(`eligible count mismatch ${eligible.length}`);
const expected = eligible.slice(0, 50);
if (JSON.stringify(expected) !== JSON.stringify(audit.selection.identities)) throw new Error('ranked identities mismatch');
if (JSON.stringify(expected.map((entry) => entry.rawIndex)) !== JSON.stringify(audit.selection.rawIndices)) throw new Error('ranked raw indices mismatch');
if (JSON.stringify(expected.map((entry) => `${entry.rawIndex}\0${entry.sourceUrl}`)) !== JSON.stringify(keys)) throw new Error('entry order mismatch');
const expectedCounts = { credible_nonempty: 42, blocked_or_auth: 1, source_incomplete_or_invalid: 3, upstream_dns: 2, upstream_http: 2 };
if (JSON.stringify(audit.summary.counts) !== JSON.stringify(expectedCounts)) throw new Error('unexpected classifications');
if (audit.summary.sharedEngineGaps.length !== 0) throw new Error('unexpected engine gap');
for (const raw of [252, 73, 158]) if (audit.entries.find((entry) => entry.rawIndex === raw)?.classification !== 'source_incomplete_or_invalid') throw new Error(`invalid source classification ${raw}`);
for (const raw of [883, 873]) if (audit.entries.find((entry) => entry.rawIndex === raw)?.classification !== 'upstream_dns') throw new Error(`DNS classification ${raw}`);
if (!audit.browserEvidence?.used || audit.browserEvidence.raw158.jsonItems !== 20 || audit.browserEvidence.raw795.configuredCardCount !== 0) throw new Error('browser evidence incomplete');
console.log(`v13 evidence valid: 50 unique, 550 excluded, ${books} distinct books, no shared engine gap`);
