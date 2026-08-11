import crypto from 'node:crypto';
import fs from 'node:fs';

const evidence = JSON.parse(fs.readFileSync('testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v1-2026-08-11.json', 'utf8'));
const corpusBytes = fs.readFileSync(evidence.corpus.path);
const corpus = JSON.parse(corpusBytes);
const digest = crypto.createHash('sha256').update(corpusBytes).digest('hex');
if (digest !== evidence.corpus.sha256) throw new Error(`corpus SHA mismatch: ${digest}`);
if (evidence.entries.length !== evidence.scope.sampleSize) throw new Error('entry count mismatch');
const keys = evidence.entries.map((entry) => `${entry.identity.rawIndex}\0${entry.identity.bookSourceUrl}`);
if (new Set(keys).size !== keys.length) throw new Error('duplicate stable identity');
for (const entry of evidence.entries) {
  const source = corpus[entry.identity.rawIndex];
  if (!source || source.bookSourceUrl !== entry.identity.bookSourceUrl) throw new Error(`identity mismatch at raw ${entry.identity.rawIndex}`);
  const isPass = entry.classification === 'credible_search_and_detail';
  if (!isPass && !entry.sequentialReplay) throw new Error(`missing sequential replay at raw ${entry.identity.rawIndex}`);
}
const count = Object.values(evidence.summary).reduce((sum, value) => sum + value, 0);
if (count !== evidence.entries.length) throw new Error('summary count mismatch');
const expected = [4,40,101,406,133,80,408,173,53,190,267,22,392,323,97,170,184,56,145,415,151,92,137,50,164];
if (JSON.stringify(evidence.selection.rawIndices) !== JSON.stringify(expected)) throw new Error('frozen raw indices changed');
console.log(`verified ${evidence.entries.length} unique identities; corpus ${digest}; ${count} classified`);
