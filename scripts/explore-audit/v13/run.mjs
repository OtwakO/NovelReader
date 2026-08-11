import fs from 'node:fs';
import { setTimeout as delay } from 'node:timers/promises';

const base = 'http://127.0.0.1:8899';
const frozen = JSON.parse(fs.readFileSync('/tmp/explore-v13-frozen.json', 'utf8'));
const output = '/tmp/explore-v13-initial.json';

async function post(path, body, timeoutMs = 95_000) {
  const started = Date.now();
  try {
    const response = await fetch(base + path, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(timeoutMs),
    });
    const text = await response.text();
    let json;
    try { json = JSON.parse(text); } catch { json = { rawBody: text.slice(0, 2000) }; }
    return { status: response.status, durationMs: Date.now() - started, body: json };
  } catch (error) {
    return { status: 0, durationMs: Date.now() - started, body: { code: error.name === 'TimeoutError' ? 'audit_timeout' : 'audit_fetch_error', message: error.message } };
  }
}

function category(catalog) {
  const entries = catalog?.entries ?? [];
  return entries.find((entry) => entry.selectable && entry.type === 'url') ?? entries.find((entry) => entry.selectable);
}

async function run(entry) {
  const catalog = await post('/api/explore/catalog', { sourceId: entry.sourceUrl });
  const selected = catalog.status === 200 ? category(catalog.body) : undefined;
  if (!selected) return { ...entry, initial: { catalog, category: null, page: null } };
  const page = await post('/api/explore/page', { sessionId: catalog.body.sessionId, categoryId: selected.id, page: 1 });
  const books = Array.isArray(page.body?.books) ? page.body.books : [];
  return { ...entry, initial: { catalog, category: { id: selected.id, title: selected.title, type: selected.type }, page: { ...page, bookCount: books.length, distinctBookUrls: new Set(books.map((book) => book.bookUrl).filter(Boolean)).size, exhausted: page.body?.exhausted ?? null, diagnostics: page.body?.diagnostics ?? [], sampleBooks: books.slice(0, 2).map((book) => ({ name: book.name, author: book.author, bookUrl: book.bookUrl })) } } };
}

const results = new Array(frozen.selection.identities.length);
let cursor = 0;
async function worker() {
  while (true) {
    const index = cursor++;
    if (index >= frozen.selection.identities.length) return;
    results[index] = await run(frozen.selection.identities[index]);
    fs.writeFileSync(output, `${JSON.stringify({ frozen, entries: results.filter(Boolean) }, null, 2)}\n`);
    const page = results[index].initial.page;
    console.log(`${index + 1}/50 raw=${results[index].rawIndex} catalog=${results[index].initial.catalog.status} page=${page?.status ?? '-'} books=${page?.bookCount ?? 0}`);
    await delay(100);
  }
}
await Promise.all(Array.from({ length: 4 }, worker));
fs.writeFileSync(output, `${JSON.stringify({ frozen, entries: results }, null, 2)}\n`);
