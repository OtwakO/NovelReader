import crypto from 'node:crypto';
import fs from 'node:fs';
const evidence=JSON.parse(fs.readFileSync('testdata/booksource/audits/explore/explore-live-v11-fixes-rerun-2026-07-31.json','utf8'));
const corpus=JSON.parse(fs.readFileSync(evidence.corpus.path,'utf8'));
const hash=crypto.createHash('sha256').update(fs.readFileSync(evidence.corpus.path)).digest('hex');
if(hash!==evidence.corpus.sha256)throw Error('corpus hash mismatch');
const expected=new Map([[701,30],[54,180],[220,100],[443,12]]);
if(evidence.results.length!==expected.size)throw Error('result count mismatch');
for(const result of evidence.results){if(result.bookSourceUrl!==corpus[result.rawIndex].bookSourceUrl)throw Error(`identity mismatch ${result.rawIndex}`);if(result.page?.status!==200||result.page?.distinctBookUrls!==expected.get(result.rawIndex))throw Error(`failed result ${result.rawIndex}`)}
if(evidence.results.find(r=>r.rawIndex===220).page.durationMs<=10000)throw Error('raw 220 did not exercise extended timeout');
console.log('v11 fixes valid: 701=30, 54=180, 220=100 after >10s, 443=12');
