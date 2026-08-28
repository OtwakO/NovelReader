import crypto from 'node:crypto';
import fs from 'node:fs';

const date = '2026-08-12';
const version = 4;
const seed = `NovelReader-search-bookinfo-random-v${version}-${date}`;
const query = '凡人修仙传';
const size = 50;
const corpusPath = 'test-booksources/test_booksource3.json';
const auditDir = 'testdata/booksource/audits/search-bookinfo';
const frozenPath = `/tmp/search-bookinfo-v${version}-frozen.json`;
const exactPath = `${auditDir}/search-bookinfo-live-audit-v${version}-frozen-sources-${date}.json`;

const corpusBytes = fs.readFileSync(corpusPath);
const corpus = JSON.parse(corpusBytes);
const priorNames = fs.readdirSync(auditDir)
  .filter((name) => /^search-bookinfo-live-audit-v\d+-\d{4}-\d{2}-\d{2}\.json$/.test(name))
  .sort((left, right) => left.localeCompare(right, undefined, { numeric: true }));
const excluded = new Set();
for (const name of priorNames) {
  const audit = JSON.parse(fs.readFileSync(`${auditDir}/${name}`, 'utf8'));
  for (const entry of audit.entries ?? []) {
    excluded.add(`${entry.identity.rawIndex}\0${entry.identity.bookSourceUrl}`);
  }
}
const eligible = corpus.flatMap((source, rawIndex) => {
  const sourceUrl = String(source.bookSourceUrl ?? '');
  const identity = `${rawIndex}\0${sourceUrl}`;
  const rules = source.ruleSearch;
  if (
    source.enabled === false ||
    Number(source.bookSourceType ?? 0) !== 0 ||
    !String(source.searchUrl ?? '').trim() ||
    rules == null ||
    (typeof rules === 'string' && !rules.trim()) ||
    (typeof rules === 'object' && Object.keys(rules).length === 0) ||
    excluded.has(identity)
  ) return [];
  const rank = crypto.createHash('sha256').update(`${seed}\0${rawIndex}\0${sourceUrl}`).digest('hex');
  return [{ rawIndex, bookSourceUrl: sourceUrl, bookSourceName: source.bookSourceName, rank }];
}).sort((left, right) => left.rank.localeCompare(right.rank));

if (excluded.size !== 100) throw new Error(`expected 100 excluded identities, got ${excluded.size}`);
if (eligible.length < size) throw new Error(`only ${eligible.length} eligible identities for sample size ${size}`);
const selected = eligible.slice(0, size);
const exactSources = selected.map(({ rawIndex, bookSourceUrl }) => {
  const source = corpus[rawIndex];
  if (!source || source.bookSourceUrl !== bookSourceUrl) throw new Error(`identity mismatch at raw ${rawIndex}`);
  return source;
});
if (new Set(exactSources.map((source) => source.bookSourceUrl)).size !== exactSources.length) {
  throw new Error('frozen sample contains duplicate bookSourceUrl runtime storage keys');
}
fs.writeFileSync(exactPath, `${JSON.stringify(exactSources, null, 2)}\n`);
const exactBytes = fs.readFileSync(exactPath);
const frozen = {
  schemaVersion: 1,
  date,
  version,
  seed,
  query,
  corpus: {
    path: corpusPath,
    sha256: crypto.createHash('sha256').update(corpusBytes).digest('hex'),
    entries: corpus.length,
  },
  exactImport: {
    path: exactPath,
    sha256: crypto.createHash('sha256').update(exactBytes).digest('hex'),
    entries: size,
  },
  selection: {
    size,
    rationale: 'User requested a 50-source Search → Book Info random-sampling round.',
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
fs.writeFileSync(frozenPath, `${JSON.stringify(frozen, null, 2)}\n`);
console.log(JSON.stringify(frozen.selection, null, 2));
