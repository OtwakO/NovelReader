import fs from 'node:fs';

const corpus = JSON.parse(fs.readFileSync('test-booksources/test_booksource3.json', 'utf8'));
const frozen = JSON.parse(fs.readFileSync('/tmp/search-bookinfo-v2-frozen.json', 'utf8'));
const sources = frozen.selection.identities.map(({ rawIndex, bookSourceUrl }) => {
  const source = corpus[rawIndex];
  if (!source || source.bookSourceUrl !== bookSourceUrl) throw new Error(`identity mismatch at raw ${rawIndex}`);
  return source;
});
if (sources.length !== frozen.selection.size) throw new Error('frozen source count mismatch');
if (new Set(sources.map((source) => source.bookSourceUrl)).size !== sources.length) throw new Error('frozen sample contains duplicate bookSourceUrl storage keys');
fs.writeFileSync('/tmp/search-bookinfo-v2-import.json', `${JSON.stringify(sources)}\n`);
console.log(`built exact frozen import: ${sources.length} sources`);
