import fs from 'node:fs';
const initial=JSON.parse(fs.readFileSync('/tmp/explore-v10-initial.json','utf8'));
const rerun=JSON.parse(fs.readFileSync('/tmp/explore-v10-rerun.json','utf8'));
const sequential=new Map(rerun.targets.map(e=>[e.rawIndex,e.sequential]));
const diagnoses={
180:['source_incomplete_or_invalid','Exact catalog JS uses invalid unparenthesized destructured arrow parameters; pinned goja and Node both reject it, while the API endpoint itself is live.'],
484:['stale_source_contract','Imported Explore result rules are blank and fallback rules target the removed iQiyi manga section; current paths redirect to a generic homepage without expected nodes.'],
268:['stale_source_contract','Current site redirects from qudushu.com to qudushu.la and exposes 15 matching rows, but every current book URL is rejected by the imported old-host bookUrlPattern.'],
642:['stale_source_contract','Live page exposes 30 matching cards, but imported bookUrlPattern requires literal `for-search` while current URLs use `/book/<id>/`.'],
767:['stale_source_contract','Current powenxue3 site no longer serves the imported total-click ranking route/content; live navigation now exposes recent-update and newest-entry lists.'],
263:['stale_source_contract','Source redirects to ijjxsxzw.com with changed navigation/routes; imported first-category endpoint no longer matches the live site contract.'],
688:['upstream_http','Direct current request returns Cloudflare HTTP 521.'],
63:['blocked_or_auth','The JS-built signed API request reaches api-bc.wtzw.com but receives HTTP 401 Unauthorized.'],
59:['upstream_http','The current category URL enters a 301 self-redirect loop; direct browser reports too many redirects.'],
937:['upstream_http','Direct production-style request returns Cloudflare HTTP 522; a browser-rendered fetch can reach catalog content, so this is an upstream/access-path failure rather than parser evidence.'],
927:['engine_gap','Live HTTP 200 contains 28 current `ul.mh-list.col7 > li` book cards. `ruleExplore` contains only coverUrl, so Legado whole-object fallback should use ruleSearch; NovelReader instead treats the partial Explore object as complete and returns zero.'],
243:['stale_source_contract','Imported category uses HTTPS mapi.xiaoshuo7.cn, which fails with 404/certificate-host mismatch; the same endpoint over HTTP currently returns JSON books.'],
415:['upstream_http','Current HTTP 200 body is a DedeCMS database error page with no expected list nodes.'],
};
function compact(run){if(!run)return null;const c=run.catalog,p=run.page;return{catalog:{status:c.status,durationMs:c.durationMs,entryCount:c.body?.entries?.length??0,error:c.status===200?null:{code:c.body?.code,stage:c.body?.stage,severity:c.body?.severity,retryable:c.body?.retryable,message:c.body?.message}},category:run.category,page:p?{status:p.status,durationMs:p.durationMs,books:p.bookCount,distinctBookUrls:p.distinctBookUrls,exhausted:p.exhausted,diagnostics:p.diagnostics,samples:p.sampleBooks,error:p.status===200?null:{code:p.body?.code,stage:p.body?.stage,severity:p.body?.severity,retryable:p.body?.retryable,message:p.body?.message}}:null}}
const entries=initial.entries.map(e=>{const run=compact(e.initial);const pass=run.page?.status===200&&run.page.books>0;const [classification,evidence]=pass?['credible_nonempty',`Production Explore returned ${run.page.books} books with ${run.page.distinctBookUrls} distinct book URLs and no suspicious diagnostic.`]:diagnoses[e.rawIndex];if(!classification)throw Error(`missing diagnosis ${e.rawIndex}`);return{rawIndex:e.rawIndex,sourceUrl:e.sourceUrl,name:e.name,rank:e.rank,...run,sequential:sequential.has(e.rawIndex)?{rawIndex:e.rawIndex,name:e.name,sourceUrl:e.sourceUrl,...compact(sequential.get(e.rawIndex))}:null,classification,evidence}});
const counts={};for(const e of entries)counts[e.classification]=(counts[e.classification]??0)+1;
const credible=entries.filter(e=>e.classification==='credible_nonempty');
const audit={schemaVersion:1,auditedAt:new Date().toISOString(),corpus:initial.frozen.corpus,selection:initial.frozen.selection,execution:{method:'Fresh isolated SQLite database; unmodified corpus imported through production API; catalog then first selectable URL category page 1.',initialConcurrency:4,timeoutSeconds:90,sequentialConfirmation:'Every non-pass rerun sequentially against the same fresh environment.',sourceOrParserChanges:false},summary:{sampleSize:entries.length,totalDistinctBooks:credible.reduce((n,e)=>n+e.page.distinctBookUrls,0),counts,sharedEngineGaps:[{family:'Partial ruleExplore object must fall back to complete ruleSearch',rawIndices:[927]}]},entries};
fs.writeFileSync('testdata/booksource/explore-live-audit-v10-2026-07-23.json',JSON.stringify(audit,null,2)+'\n');console.log(audit.summary);
