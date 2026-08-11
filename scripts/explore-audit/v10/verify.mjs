import crypto from 'node:crypto';
import fs from 'node:fs';
const path='testdata/booksource/audits/explore/explore-live-audit-v10-2026-07-23.json';
const audit=JSON.parse(fs.readFileSync(path,'utf8'));
const corpus=fs.readFileSync(audit.corpus.path);
if(crypto.createHash('sha256').update(corpus).digest('hex')!==audit.corpus.sha256)throw Error('corpus hash mismatch');
if(audit.entries.length!==50||audit.selection.rawIndices.length!==50)throw Error('sample size mismatch');
const keys=audit.entries.map(e=>`${e.rawIndex}\0${e.sourceUrl}`);
if(new Set(keys).size!==50)throw Error('duplicate identities');
if(audit.entries.some(e=>!e.classification))throw Error('missing classification');
if(audit.entries.filter(e=>e.classification!=='credible_nonempty').some(e=>!e.sequential))throw Error('missing sequential confirmation');
const counts={};for(const e of audit.entries)counts[e.classification]=(counts[e.classification]??0)+1;
if(JSON.stringify(counts)!==JSON.stringify(audit.summary.counts))throw Error('summary counts mismatch');
const books=audit.entries.filter(e=>e.classification==='credible_nonempty').reduce((n,e)=>n+e.page.distinctBookUrls,0);
if(books!==audit.summary.totalDistinctBooks)throw Error('book total mismatch');
const prior=new Set();for(const name of audit.selection.priorManifests){const j=JSON.parse(fs.readFileSync(`testdata/booksource/audits/explore/${name}`,'utf8'));for(const e of j.entries??[])prior.add(`${e.rawIndex}\0${e.sourceUrl??e.bookSourceUrl}`)}
if(prior.size!==400)throw Error(`prior identity count ${prior.size}`);
if(keys.some(k=>prior.has(k)))throw Error('sample overlaps prior audit');
console.log(`v10 evidence valid: 50 unique, 400 excluded, ${books} distinct books`);
