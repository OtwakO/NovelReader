import { beforeEach, describe, expect, it, vi } from 'vitest';
import { addSearchResultToShelf, previewBook, shelveBook } from './books';

const request = vi.fn();
vi.mock('./transport', () => ({ request: (...args: unknown[]) => request(...args) }));

const result = {
  name: '凡人修仙传', author: '忘语', coverUrl: '', intro: '', kind: '仙侠', lastChapter: '第一章',
  sourceUrl: 'https://source.example', bookUrl: 'https://source.example/book/1', sourceName: '测试书源',
  score: 99, alternateSources: [{ sourceUrl: 'https://alternate.example', bookUrl: 'https://alternate.example/book/1', sourceName: '备用书源' }],
};

describe('book candidate transport', () => {
  beforeEach(() => request.mockReset());
  it('removes client-only result fields before previewing', async () => {
    request.mockResolvedValue({ book: result, chapters: [] });
    await previewBook(result);
    const body = JSON.parse(request.mock.calls[0]?.[1]?.body as string);
    expect(body).toEqual({
      name: '凡人修仙传', author: '忘语', coverUrl: '', intro: '', kind: '仙侠', lastChapter: '第一章',
      sourceName: '测试书源', sourceUrl: 'https://source.example', bookUrl: 'https://source.example/book/1',
      alternateSources: [{ sourceUrl: 'https://alternate.example', bookUrl: 'https://alternate.example/book/1', sourceName: '备用书源' }],
    });
    expect(body).not.toHaveProperty('score');
  });
  it('adds only the required id when shelving the same result', async () => {
    request.mockResolvedValue({ id: 'book-1' });
    await shelveBook({ ...result, id: 'book-1' });
    const body = JSON.parse(request.mock.calls[0]?.[1]?.body as string);
    expect(body.id).toBe('book-1');
    expect(body).not.toHaveProperty('score');
  });
  it('uses canonical preview data before shelving and retains discovered alternatives', async () => {
    request
      .mockResolvedValueOnce({ book: { ...result, name: 'Canonical', score: undefined }, chapters: [] })
      .mockResolvedValueOnce({ id: 'stored' });
    await addSearchResultToShelf(result);
    expect(request.mock.calls.map((call) => call[0])).toEqual(['/books/preview', '/books/shelve']);
    const shelveBody = JSON.parse(request.mock.calls[1]?.[1]?.body as string);
    expect(shelveBody.name).toBe('Canonical');
    expect(shelveBody.alternateSources).toEqual(result.alternateSources);
    expect(shelveBody).not.toHaveProperty('score');
  });
});
