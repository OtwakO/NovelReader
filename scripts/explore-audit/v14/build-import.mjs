import fs from 'node:fs';

const preservedPath = 'testdata/booksource/audits/explore/explore-live-audit-v14-frozen-sources-2026-08-11.json';
let sources;
if (fs.existsSync(preservedPath)) {
  sources = JSON.parse(fs.readFileSync(preservedPath, 'utf8'));
} else {
  const corpus = JSON.parse(fs.readFileSync('test-booksources/test_booksource4.json', 'utf8'));
  const frozen = JSON.parse(fs.readFileSync('/tmp/explore-v14-frozen.json', 'utf8'));
  sources = frozen.selection.identities.map(({ rawIndex, sourceUrl }) => {
    const source = corpus[rawIndex];
    if (source.bookSourceUrl !== sourceUrl) throw new Error(`identity mismatch at raw ${rawIndex}`);
    return source;
  });
}
if (sources.length !== 50) throw new Error(`expected 50 frozen sources, got ${sources.length}`);
if (new Set(sources.map((source) => source.bookSourceUrl)).size !== sources.length) throw new Error('frozen sample contains duplicate bookSourceUrl storage keys');
fs.writeFileSync('/tmp/explore-v14-import.json', `${JSON.stringify(sources)}\n`);
console.log(`built exact frozen import: ${sources.length} sources`);
