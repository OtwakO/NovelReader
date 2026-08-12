import fs from 'node:fs';

const evidencePath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-v4-fixes-rerun-2026-08-12.json';
const auditPath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v4-2026-08-12.json';
const frozenPath = 'testdata/booksource/audits/search-bookinfo/search-bookinfo-live-audit-v4-frozen-sources-2026-08-12.json';
const evidence = JSON.parse(fs.readFileSync(evidencePath, 'utf8'));
const audit = JSON.parse(fs.readFileSync(auditPath, 'utf8'));
const frozen = JSON.parse(fs.readFileSync(frozenPath, 'utf8'));

if (evidence.sourceAudit !== 'search-bookinfo-live-audit-v4-2026-08-12.json') throw new Error('source audit mismatch');
if (evidence.entries.length !== 1) throw new Error('expected one bridge-fix replay');

const result = evidence.entries[0];
const frozenIndex = audit.selection.rawIndices.indexOf(72);
const original = audit.entries.find((entry) => entry.identity.rawIndex === 72);
const observation = audit.compatibilityObservations.find((item) => item.id === 'java-encode-uri-charset-argument');
if (frozenIndex !== 49 || result.frozenIndex !== frozenIndex) throw new Error('raw 72 frozen position mismatch');
if (!original || !observation) throw new Error('raw 72 audit evidence missing');
if (frozen[frozenIndex]?.bookSourceUrl !== result.bookSourceUrl || original.identity.bookSourceUrl !== result.bookSourceUrl) {
  throw new Error('raw 72 identity mismatch');
}
if (!frozen[frozenIndex].searchUrl.includes("java.encodeURI(key,'gb2312')")) throw new Error('frozen charset-aware bridge call missing');
if (!result.searchRequestUrl.includes('Keyword=%B7%B2%C8%CB%D0%DE%CF%C9%B4%AB') || result.searchRequestUrl.includes('%E5%87%A1')) {
  throw new Error('GB2312 request evidence missing');
}
if (result.requestEncoding !== 'GB2312' || result.responseStatus !== 200 || result.responseQueryText !== '凡人修仙传') {
  throw new Error('raw 72 corrected request/response evidence mismatch');
}
if (result.classification !== 'legitimate_empty' || result.responseResultCount !== 0 || !result.responseMessage.includes('未查询到')) {
  throw new Error('raw 72 legitimate-empty outcome changed');
}
if (audit.sharedEngineGaps.length !== 0) throw new Error('v4 historical audit unexpectedly gained a recoverable gap');
if (!evidence.implementation.javaEncodeURICharset.includes('existing shared charset encoder')) throw new Error('bounded implementation contract missing');

console.log('verified Search → Book Info v4 bridge fix: raw 72 uses GB2312 and remains a legitimate empty result');
