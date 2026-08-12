import fs from 'node:fs';

const evidencePath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-v3-fixes-rerun-2026-08-12.json';
const auditPath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v3-2026-08-12.json';
const frozenPath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v3-frozen-sources-2026-08-12.json';
const evidence = JSON.parse(fs.readFileSync(evidencePath, 'utf8'));
const audit = JSON.parse(fs.readFileSync(auditPath, 'utf8'));
const frozen = JSON.parse(fs.readFileSync(frozenPath, 'utf8'));

if (evidence.sourceAudit !== 'search-bookinfo-live-audit-v3-2026-08-12.json') throw new Error('source audit mismatch');
if (evidence.entries.length !== 2) throw new Error('expected two live recoveries');

const expectedGaps = new Map([
  [375, 'regex-only-one-preserves-surrounding-input'],
  [364, 'get-query-option-charset'],
]);
const expectedResults = new Map([
  [375, { frozenIndex: 6, count: 20, name: '凡人修仙传', url: 'https://m.qidian.com/book/107580/' }],
  [364, { frozenIndex: 25, count: 20, name: '我在凡人修仙传开直播', url: 'https://wap.faloo.com/1492008.html' }],
]);

for (const [rawIndex, gapID] of expectedGaps) {
  const gap = audit.sharedGaps.find((item) => item.id === gapID);
  if (!gap || !gap.primaryAffectedRawIndices.includes(rawIndex)) throw new Error(`missing original gap ${gapID}`);
  const originalEntry = audit.entries.find((item) => item.identity.rawIndex === rawIndex);
  const result = evidence.entries.find((item) => item.rawIndex === rawIndex);
  const expected = expectedResults.get(rawIndex);
  if (!originalEntry || !result) throw new Error(`missing raw ${rawIndex}`);
  if (result.frozenIndex !== expected.frozenIndex || audit.selection.rawIndices[expected.frozenIndex] !== rawIndex) {
    throw new Error(`frozen position mismatch raw ${rawIndex}`);
  }
  if (frozen[expected.frozenIndex]?.bookSourceUrl !== result.bookSourceUrl || originalEntry.identity.bookSourceUrl !== result.bookSourceUrl) {
    throw new Error(`identity mismatch raw ${rawIndex}`);
  }
  if (result.classification !== 'success' || result.searchResultCount !== expected.count) throw new Error(`raw ${rawIndex} did not recover`);
  if (result.selectedName !== expected.name || result.bookInfoName !== expected.name || result.selectedBookUrl !== expected.url) {
    throw new Error(`result mismatch raw ${rawIndex}`);
  }
  if (result.diagnostics.length) throw new Error(`raw ${rawIndex} retained diagnostics`);
}

const charsetEntry = evidence.entries.find((item) => item.rawIndex === 364);
if (charsetEntry.requestCharset !== 'GB2312' || charsetEntry.searchRequestUrl.includes('%E5%87%A1')) {
  throw new Error('GB2312 request evidence missing');
}
if (!evidence.implementation.onlyOneRegex.includes('first matched-and-replaced') || !evidence.implementation.getQueryCharset.includes('GET query')) {
  throw new Error('primary implementation contracts missing');
}
if (!evidence.implementation.urlJsTags.includes('Rhino/JVM APIs remain outside') || !evidence.implementation.urlJSLib.includes('Search and Explore')) {
  throw new Error('bounded URL JavaScript contracts missing');
}

console.log('verified Search → Book Info v3 fixes: raws 375/364 each return 20 results and pass Book Info');
