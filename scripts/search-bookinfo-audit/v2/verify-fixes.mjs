import fs from 'node:fs';

const evidencePath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-v2-fixes-rerun-2026-08-12.json';
const auditPath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v2-2026-08-11.json';
const frozenPath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v2-frozen-sources-2026-08-11.json';
const evidence = JSON.parse(fs.readFileSync(evidencePath, 'utf8'));
const audit = JSON.parse(fs.readFileSync(auditPath, 'utf8'));
const frozen = JSON.parse(fs.readFileSync(frozenPath, 'utf8'));

if (evidence.sourceAudit !== 'search-bookinfo-live-audit-v2-2026-08-11.json') throw new Error('source audit mismatch');
if (evidence.entries.length !== 2) throw new Error('expected two resolved gaps');
for (const expected of [
  { rawIndex: 49, count: 4, name: '凡人修仙之仙界篇', url: 'https://www.yodu.org/book/17927/?for-search' },
  { rawIndex: 179, count: 1, name: '凡人修仙传', url: 'https://www.23hh.la/book/3/3713/' },
]) {
  const result = evidence.entries.find((item) => item.rawIndex === expected.rawIndex);
  if (!result) throw new Error(`missing raw ${expected.rawIndex}`);
  const originalEntry = audit.entries.find((item) => item.identity.rawIndex === expected.rawIndex);
  if (!originalEntry || originalEntry.identity.bookSourceUrl !== result.bookSourceUrl) throw new Error(`identity mismatch raw ${expected.rawIndex}`);
  const selectedPosition = audit.selection.rawIndices.indexOf(expected.rawIndex);
  if (selectedPosition < 0 || frozen[selectedPosition]?.bookSourceUrl !== result.bookSourceUrl) throw new Error(`frozen definition mismatch raw ${expected.rawIndex}`);
  if (result.classification !== 'success') throw new Error(`raw ${expected.rawIndex} did not pass`);
  if (result.searchResultCount !== expected.count) throw new Error(`raw ${expected.rawIndex} count mismatch`);
  if (result.selectedName !== expected.name || result.bookInfoName !== expected.name) throw new Error(`raw ${expected.rawIndex} name mismatch`);
  if (result.selectedBookUrl !== expected.url) throw new Error(`raw ${expected.rawIndex} URL mismatch`);
  if (result.selectedBookUrl.includes('\n') || result.diagnostics.length) throw new Error(`raw ${expected.rawIndex} retained diagnostics or multiline URL`);
}
if (!evidence.implementation.defaultJsoupSingularBookUrl.includes('Do not change XPath, JSONPath, Explore')) throw new Error('URL scope guarantee missing');
if (!evidence.implementation.emptySearchBookUrl.includes('final response URL')) throw new Error('response URL guarantee missing');
console.log('verified Search → Book Info v2 fixes: raw 49 4 results, raw 179 1 result, both detail passes');
