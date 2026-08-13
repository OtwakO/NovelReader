import { beforeEach, describe, expect, it, vi } from 'vitest';
import { addSearchResultToShelf } from './books';

beforeEach(() => { vi.restoreAllMocks(); });

describe('addSearchResultToShelf', () => {
  it('uses canonical preview data before shelving and retains discovered alternatives', async () => {
    const requests: Array<{ url: string; body: Record<string, unknown> }> = [];
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const body = init?.body ? JSON.parse(String(init.body)) as Record<string, unknown> : {};
      requests.push({ url, body });
      if (url.endsWith('/books/preview')) return new Response(JSON.stringify({ book: { name: 'Canonical', author: 'Author', coverUrl: 'cover', intro: 'intro', kind: 'kind', sourceUrl: 'source', bookUrl: '/book', lastChapter: 'latest' }, chapters: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } });
      return new Response(JSON.stringify({ id: 'stored', ...body }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    }));

    await addSearchResultToShelf({ name: 'Search title', author: 'Author', coverUrl: '', intro: '', kind: '', lastChapter: '', sourceUrl: 'source', sourceName: 'Source', bookUrl: '/book', alternateSources: [{ sourceUrl: 'other', sourceName: 'Other', bookUrl: '/other' }] });

    expect(requests.map((request) => request.url)).toEqual(['/api/books/preview', '/api/books/shelve']);
    expect(requests[1]?.body.name).toBe('Canonical');
    expect(requests[1]?.body.alternateSources).toEqual([{ sourceUrl: 'other', sourceName: 'Other', bookUrl: '/other' }]);
  });
});
