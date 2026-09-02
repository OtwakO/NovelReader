import { afterEach, describe, expect, it, vi } from 'vitest';
import { getCatalog, getChapterContent, waitForCatalog } from './reader';

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe('reading API', () => {
  it('parses the versioned prose document and opaque resources', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
      version: 1,
      document: {
        kind: 'prose',
        title: 'Chapter',
        blocks: [
          { kind: 'paragraph', text: 'Before' },
          { kind: 'image', resource: { href: '/api/books/book-1/chapters/0/images/0' }, alt: 'Map' },
        ],
      },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } })));

    await expect(getChapterContent('book-1', 0)).resolves.toEqual({
      version: 1,
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
  });

  it('rejects non-NovelReader resource references', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
      version: 1,
      document: { kind: 'prose', title: 'Chapter', blocks: [{ kind: 'image', resource: { href: 'https://source.test/private' } }] },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } })));

    await expect(getChapterContent('book-1', 0)).rejects.toThrow('Invalid content resource');
  });

  describe('catalog', () => {
    it('distinguishes ready chapters from syncing responses', async () => {
      const fetchMock = vi.fn()
        .mockResolvedValueOnce(new Response(JSON.stringify({ state: 'syncing' }), { status: 202, headers: { 'Content-Type': 'application/json' } }))
        .mockResolvedValueOnce(new Response(JSON.stringify([{ id: 'book-1_0', bookId: 'book-1', index: 0, title: 'One', url: '/1' }]), { status: 200, headers: { 'Content-Type': 'application/json' } }));
      vi.stubGlobal('fetch', fetchMock);

      await expect(getCatalog('book-1')).resolves.toEqual({ state: 'syncing' });
      await expect(getCatalog('book-1')).resolves.toEqual({ state: 'ready', chapters: [expect.objectContaining({ title: 'One' })] });
    });

    it('starts retry explicitly and polls until the catalog is ready', async () => {
      vi.useFakeTimers();
      const fetchMock = vi.fn()
        .mockResolvedValueOnce(new Response(JSON.stringify({ state: 'syncing' }), { status: 202, headers: { 'Content-Type': 'application/json' } }))
        .mockResolvedValueOnce(new Response(JSON.stringify([{ id: 'book-1_0', bookId: 'book-1', index: 0, title: 'One', url: '/1' }]), { status: 200, headers: { 'Content-Type': 'application/json' } }));
      vi.stubGlobal('fetch', fetchMock);

      const chapters = waitForCatalog('book-1', { retry: true });
      await vi.advanceTimersByTimeAsync(500);

      await expect(chapters).resolves.toEqual([expect.objectContaining({ title: 'One' })]);
      expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/books/book-1/chapters/sync', expect.objectContaining({ method: 'POST' }));
      expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/books/book-1/chapters', expect.any(Object));
    });
  });
});
