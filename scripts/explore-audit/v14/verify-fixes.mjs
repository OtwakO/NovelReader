import fs from 'node:fs';

const evidence = JSON.parse(fs.readFileSync(
  'testdata/booksource/audits/explore/explore-live-v14-fixes-rerun-2026-08-11.json',
  'utf8',
));
const audit = JSON.parse(fs.readFileSync(
  'testdata/booksource/audits/explore/explore-live-audit-v14-2026-08-11.json',
  'utf8',
));
const expected = new Map([[669, 60], [703, 15], [80, 30]]);
if (evidence.results.length !== expected.size) throw new Error('unexpected rerun result count');
for (const result of evidence.results) {
  const want = expected.get(result.rawIndex);
  if (!want) throw new Error(`unexpected raw ${result.rawIndex}`);
  const audited = audit.entries.find((entry) => entry.rawIndex === result.rawIndex);
  if (!audited || audited.bookSourceUrl !== result.sourceUrl || audited.classification !== 'engine_gap') {
    throw new Error(`audit identity/classification mismatch ${result.rawIndex}`);
  }
  if (result.beforeDistinctBooks !== 0 || result.afterStatus !== 200 || result.afterDistinctBooks !== want) {
    throw new Error(`post-fix result mismatch ${result.rawIndex}`);
  }
  if (!Array.isArray(result.diagnostics) || result.diagnostics.length !== 0) {
    throw new Error(`unexpected diagnostics ${result.rawIndex}`);
  }
}
if (evidence.execution.sourceChangesDuringRerun !== false) throw new Error('sources changed during rerun');
console.log('v14 post-fix evidence valid: raw 669=60, raw 703=15, raw 80=30');
