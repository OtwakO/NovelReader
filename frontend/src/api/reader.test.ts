import { afterEach, describe, expect, it, vi } from 'vitest';
import { addBookmark, deleteBookmark, getCatalog, getChapterContent, saveProgress, waitForCatalog } from './reader';

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe('reading API', () => {
  it('parses the versioned prose document and opaque resources', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
      version: 1, contentRevision: 7,
      document: {
        kind: 'prose',
        title: 'Chapter',
        blocks: [
          { kind: 'paragraph', text: 'Before' },
          { kind: 'image', resource: { href: '/api/books/book-1/chapters/0/images/0' }, alt: 'Map' },
        ],
      },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } })));

    await expect(getChapterContent('book-1', 0, 7)).resolves.toEqual({
      version: 1, contentRevision: 7,
      document: {
        kind: 'prose',
        title: 'Chapter',
        blocks: [
          { kind: 'paragraph', text: 'Before' },
          { kind: 'image', resource: { href: '/api/books/book-1/chapters/0/images/0' }, alt: 'Map' },
        ],
      },
      offlineCopy: false,
    });
    expect(fetch).toHaveBeenCalledWith('/api/books/book-1/chapters/0/content?contentRevision=7', expect.any(Object));
  });

  it('rejects non-NovelReader resource references', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
      version: 1, contentRevision: 7,
      document: { kind: 'prose', title: 'Chapter', blocks: [{ kind: 'image', resource: { href: 'https://source.test/private' } }] },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } })));

    await expect(getChapterContent('book-1', 0, 7)).rejects.toThrow('Invalid content resource');
  });

  describe('catalog', () => {
    it('distinguishes ready chapters from syncing responses', async () => {
      const fetchMock = vi.fn()
        .mockResolvedValueOnce(new Response(JSON.stringify({ state: 'syncing' }), { status: 202, headers: { 'Content-Type': 'application/json' } }))
        .mockResolvedValueOnce(new Response(JSON.stringify({ chapters: [{ id: 'book-1_0', bookId: 'book-1', index: 0, title: 'One', url: '/1' }], contentRevision: 4 }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
      vi.stubGlobal('fetch', fetchMock);

      await expect(getCatalog('book-1')).resolves.toEqual({ state: 'syncing' });
      await expect(getCatalog('book-1')).resolves.toEqual({ state: 'ready', chapters: [expect.objectContaining({ title: 'One' })], contentRevision: 4 });
    });

    it('starts retry explicitly and polls until the catalog is ready', async () => {
      vi.useFakeTimers();
      const fetchMock = vi.fn()
        .mockResolvedValueOnce(new Response(JSON.stringify({ state: 'syncing' }), { status: 202, headers: { 'Content-Type': 'application/json' } }))
        .mockResolvedValueOnce(new Response(JSON.stringify({ chapters: [{ id: 'book-1_0', bookId: 'book-1', index: 0, title: 'One', url: '/1' }], contentRevision: 4 }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
      vi.stubGlobal('fetch', fetchMock);

      const chapters = waitForCatalog('book-1', { retry: true });
      await vi.advanceTimersByTimeAsync(500);

      await expect(chapters).resolves.toEqual({ chapters: [expect.objectContaining({ title: 'One' })], contentRevision: 4 });
      expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/books/book-1/chapters/sync', expect.objectContaining({ method: 'POST' }));
      expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/books/book-1/chapters', expect.any(Object));
    });
  });
});

it('sends revision-qualified reading mutations and returns their state versions', async () => {
  const fetchMock = vi.fn(async () => new Response(JSON.stringify({ stateVersion: 5 }), { status: 200 }));
  vi.stubGlobal('fetch', fetchMock);
  const bookmark = { id: 'mark', contentRevision: 3, stateVersion: 4, chapterIndex: 2, position: .5, note: '' };
  await expect(addBookmark('book', bookmark)).resolves.toMatchObject({ stateVersion: 5 });
  await expect(deleteBookmark('book', 'mark', 3, 5)).resolves.toMatchObject({ stateVersion: 5 });
  await saveProgress('book', 3, 5, 2, .5);
  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/books/book/bookmarks', expect.objectContaining({ method: 'POST', body: JSON.stringify(bookmark) }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/books/book/bookmarks/mark', expect.objectContaining({ method: 'DELETE', body: JSON.stringify({ contentRevision: 3, stateVersion: 5 }) }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/books/book/progress', expect.objectContaining({ body: JSON.stringify({ contentRevision: 3, stateVersion: 5, chapterIndex: 2, position: .5 }) }));
});
