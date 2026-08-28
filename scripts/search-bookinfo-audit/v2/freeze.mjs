import crypto from 'node:crypto';
import fs from 'node:fs';

const date = '2026-08-11';
const seed = `NovelReader-search-bookinfo-random-v2-${date}`;
const query = '凡人修仙传';
const size = 25;
const corpusPath = 'test-booksources/test_booksource3.json';
const auditDir = 'testdata/booksource/audits/search-bookinfo';
const outputPath = '/tmp/search-bookinfo-v2-frozen.json';
const corpusBytes = fs.readFileSync(corpusPath);
const corpus = JSON.parse(corpusBytes);
const priorNames = fs.readdirSync(auditDir)
  .filter((name) => /^search-bookinfo-live-audit-v\d+-.*\.json$/.test(name))
  .sort((left, right) => left.localeCompare(right, undefined, { numeric: true }));
const excluded = new Set();
for (const name of priorNames) {
  const audit = JSON.parse(fs.readFileSync(`${auditDir}/${name}`, 'utf8'));
  for (const entry of audit.entries ?? []) excluded.add(`${entry.identity.rawIndex}\0${entry.identity.bookSourceUrl}`);
}
const eligible = corpus.flatMap((source, rawIndex) => {
  const sourceUrl = String(source.bookSourceUrl ?? '');
  const identity = `${rawIndex}\0${sourceUrl}`;
  if (source.enabled === false || Number(source.bookSourceType ?? 0) !== 0 || !String(source.searchUrl ?? '').trim() || excluded.has(identity)) return [];
  const rules = source.ruleSearch;
  if (rules == null || (typeof rules === 'string' && !rules.trim()) || (typeof rules === 'object' && Object.keys(rules).length === 0)) return [];
  const rank = crypto.createHash('sha256').update(`${seed}\0${rawIndex}\0${sourceUrl}`).digest('hex');
  return [{ rawIndex, bookSourceUrl: sourceUrl, bookSourceName: source.bookSourceName, rank }];
}).sort((left, right) => left.rank.localeCompare(right.rank));
if (excluded.size !== 25) throw new Error(`expected 25 excluded identities, got ${excluded.size}`);
if (eligible.length < size) throw new Error(`only ${eligible.length} eligible identities for sample size ${size}`);
const selected = eligible.slice(0, size);
const frozen = {
  schemaVersion: 1,
  date,
  seed,
  query,
  corpus: { path: corpusPath, sha256: crypto.createHash('sha256').update(corpusBytes).digest('hex'), entries: corpus.length },
  selection: {
    size,
    rationale: 'User requested another Search → Book Info random-sampling round; retain v1’s focused 25-identity diagnosis load.',
    eligibleRemainingBeforeSelection: eligible.length,
    excludedIdentityCount: excluded.size,
    priorManifests: priorNames,
    eligibility: 'enabled text source (bookSourceType=0) with non-blank searchUrl and ruleSearch',
    ranking: 'SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)',
    unrestricted: true,
    substitutions: false,
    rawIndices: selected.map((entry) => entry.rawIndex),
    identities: selected,
  },
};
fs.writeFileSync(outputPath, `${JSON.stringify(frozen, null, 2)}\n`);
console.log(JSON.stringify(frozen.selection, null, 2));
