import fs from 'node:fs';
import { setTimeout as delay } from 'node:timers/promises';

const base = 'http://127.0.0.1:8899';
const initial = JSON.parse(fs.readFileSync('/tmp/explore-v13-initial.json', 'utf8'));
const cookie = process.env.AUDIT_COOKIE;
if (!cookie) throw new Error('AUDIT_COOKIE is required');

async function post(path, body, timeoutMs = 95_000) {
  const started = Date.now();
  try {
    const response = await fetch(base + path, {
      method: 'POST',
      headers: { 'content-type': 'application/json', cookie, origin: base },
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

function choice(catalog) {
  return (catalog?.entries ?? []).find((entry) => entry.selectable && entry.type === 'url') ?? (catalog?.entries ?? []).find((entry) => entry.selectable);
}

async function run(entry) {
  const catalog = await post('/api/explore/catalog', { sourceId: entry.sourceUrl });
  const selected = catalog.status === 200 ? choice(catalog.body) : null;
  if (!selected) return { catalog, category: null, page: null };
  const page = await post('/api/explore/page', { sessionId: catalog.body.sessionId, categoryId: selected.id, page: 1 });
  const books = Array.isArray(page.body?.books) ? page.body.books : [];
  return { catalog, category: { id: selected.id, title: selected.title, type: selected.type }, page: { ...page, bookCount: books.length, distinctBookUrls: new Set(books.map((book) => book.bookUrl).filter(Boolean)).size, exhausted: page.body?.exhausted ?? null, diagnostics: page.body?.diagnostics ?? [], sampleBooks: books.slice(0, 2).map((book) => ({ name: book.name, author: book.author, bookUrl: book.bookUrl })) } };
}

const targets = initial.entries.filter((entry) => entry.initial.catalog.status !== 200 || !entry.initial.page || entry.initial.page.status !== 200 || entry.initial.page.bookCount === 0 || entry.initial.page.distinctBookUrls === 0 || entry.initial.page.diagnostics?.length);
const results = [];
for (let index = 0; index < targets.length; index++) {
  const entry = targets[index];
  const sequential = await run(entry);
  results.push({ ...entry, sequential });
  console.log(`${index + 1}/${targets.length} raw=${entry.rawIndex} catalog=${sequential.catalog.status} page=${sequential.page?.status ?? '-'} books=${sequential.page?.bookCount ?? 0}`);
  fs.writeFileSync('/tmp/explore-v13-rerun.json', `${JSON.stringify({ targets: results }, null, 2)}\n`);
  await delay(500);
}
