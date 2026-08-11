import crypto from 'node:crypto';
import fs from 'node:fs';
const evidence=JSON.parse(fs.readFileSync('testdata/booksource/audits/explore/explore-live-v12-fixes-rerun-2026-07-31.json','utf8'));
const bytes=fs.readFileSync(evidence.corpus.path);
if(crypto.createHash('sha256').update(bytes).digest('hex')!==evidence.corpus.sha256)throw Error('corpus hash mismatch');
const corpus=JSON.parse(bytes);const expected=new Map([[197,15],[163,20],[17,10]]);
if(evidence.results.length!==3)throw Error('result count mismatch');
for(const result of evidence.results){if(result.bookSourceUrl!==corpus[result.rawIndex].bookSourceUrl)throw Error(`identity mismatch ${result.rawIndex}`);if(result.page?.status!==200||result.page?.distinctBookUrls!==expected.get(result.rawIndex))throw Error(`failed result ${result.rawIndex}`)}
const raw197=evidence.results.find(r=>r.rawIndex===197);if(raw197.page.diagnostics?.[0]?.code!=='http_status')throw Error('raw 197 did not preserve HTTP status diagnostic');
console.log('v12 fixes valid: raw 197=15 with HTTP diagnostic, raw 163=20, raw 17=10');
