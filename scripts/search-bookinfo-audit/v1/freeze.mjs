import crypto from 'node:crypto';
import fs from 'node:fs';

const date = '2026-08-11';
const seed = `NovelReader-search-bookinfo-random-v1-${date}`;
const query = '凡人修仙传';
const size = 25;
const corpusPath = 'test_booksource3.json';
const outputPath = '/tmp/search-bookinfo-v1-frozen.json';
const corpusBytes = fs.readFileSync(corpusPath);
const corpus = JSON.parse(corpusBytes);

const eligible = corpus.flatMap((source, rawIndex) => {
  const sourceUrl = String(source.bookSourceUrl ?? '');
  if (source.enabled === false || Number(source.bookSourceType ?? 0) !== 0 || !String(source.searchUrl ?? '').trim()) return [];
  const rules = source.ruleSearch;
  if (rules == null || (typeof rules === 'string' && !rules.trim()) || (typeof rules === 'object' && Object.keys(rules).length === 0)) return [];
  const rank = crypto.createHash('sha256').update(`${seed}\0${rawIndex}\0${sourceUrl}`).digest('hex');
  return [{ rawIndex, bookSourceUrl: sourceUrl, bookSourceName: source.bookSourceName, rank }];
}).sort((left, right) => left.rank.localeCompare(right.rank));

if (eligible.length < size) throw new Error(`only ${eligible.length} eligible identities for sample size ${size}`);
const selected = eligible.slice(0, size);
const frozen = {
  schemaVersion: 1,
  date,
  seed,
  query,
  corpus: {
    path: corpusPath,
    sha256: crypto.createHash('sha256').update(corpusBytes).digest('hex'),
    entries: corpus.length,
  },
  selection: {
    size,
    rationale: 'User selected a focused first round of 25 Search → Book Info identities.',
    eligibleBeforeSelection: eligible.length,
    eligibility: 'enabled text source (bookSourceType=0) with non-blank searchUrl and ruleSearch',
    ranking: 'SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)',
    unrestricted: true,
    substitutions: false,
    rawIndices: selected.map((entry) => entry.rawIndex),
    identities: selected,
  },
  references: [
    'https://mgz0227.github.io/The-tutorial-of-Legado/Rule/source.html',
    'reference/legado',
  ],
};
fs.writeFileSync(outputPath, `${JSON.stringify(frozen, null, 2)}\n`);
console.log(JSON.stringify(frozen, null, 2));
