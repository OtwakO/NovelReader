import crypto from 'node:crypto';
import fs from 'node:fs';

const seed = 'NovelReader-explore-random-v11-2026-07-23';
const size = 50;
const corpusPath = 'test_booksource4.json';
const auditPattern = /^explore-live-audit(?:-v\d+)?-.*\.json$/;
const corpusBytes = fs.readFileSync(corpusPath);
const corpus = JSON.parse(corpusBytes);
const priorManifests = fs.readdirSync('testdata/booksource')
  .filter((name) => auditPattern.test(name) && !name.includes('priority-fix-rerun'))
  .sort((a, b) => a.localeCompare(b, undefined, { numeric: true }));
const excluded = new Set();
for (const name of priorManifests) {
  const audit = JSON.parse(fs.readFileSync(`testdata/booksource/audits/explore/${name}`, 'utf8'));
  for (const entry of audit.entries ?? []) {
    excluded.add(`${entry.rawIndex}\0${entry.bookSourceUrl ?? entry.sourceUrl}`);
  }
}
const eligible = corpus.flatMap((source, rawIndex) => {
  const sourceUrl = source.bookSourceUrl;
  const identity = `${rawIndex}\0${sourceUrl}`;
  if (source.enabled === false || !String(source.exploreUrl ?? '').trim() || excluded.has(identity)) return [];
  const rank = crypto.createHash('sha256').update(`${seed}\0${rawIndex}\0${sourceUrl}`).digest('hex');
  return [{ rawIndex, sourceUrl, name: source.bookSourceName, rank }];
}).sort((a, b) => a.rank.localeCompare(b.rank));
if (excluded.size !== 450) throw new Error(`expected 450 excluded identities, got ${excluded.size}`);
if (eligible.length !== 271) throw new Error(`expected 271 eligible identities, got ${eligible.length}`);
const selected = eligible.slice(0, size);
const frozen = {
  schemaVersion: 1,
  seed,
  corpus: { path: corpusPath, sha256: crypto.createHash('sha256').update(corpusBytes).digest('hex'), entries: corpus.length },
  selection: {
    size,
    rationale: 'Same unrestricted 50-identity breadth as v10 because that batch still exposed one shared compatibility seam.',
    eligibleRemainingBeforeSelection: eligible.length,
    excludedIdentityCount: excluded.size,
    priorManifests,
    ranking: 'SHA-256(seed + NUL + rawIndex + NUL + bookSourceUrl)',
    unrestricted: true,
    rawIndices: selected.map((entry) => entry.rawIndex),
    identities: selected,
  },
};
fs.writeFileSync('/tmp/explore-v11-frozen.json', `${JSON.stringify(frozen, null, 2)}\n`);
console.log(JSON.stringify(frozen.selection, null, 2));
