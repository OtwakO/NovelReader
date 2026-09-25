import { beforeEach, expect, it, vi } from 'vitest';
import { getBook } from './books';
import { request } from './transport';

vi.mock('./transport', () => ({ request: vi.fn() }));
beforeEach(() => vi.resetAllMocks());

it('accepts additive entry qualification and older servers without it', async () => {
  const legacy = { id: 'book', contentRevision: 3 };
  vi.mocked(request).mockResolvedValue(legacy);
  expect(await getBook('book')).toEqual(legacy);
  const book = { ...legacy, readingContext: { sourceIdentity: 'definition', chineseConversion: { available: true, engine: 'OpenCC', version: '1', modes: ['traditional'], presets: { traditional: 's2twp' } } } };
  vi.mocked(request).mockResolvedValue(book);
  expect(await getBook('book')).toEqual(book);
});

it('rejects malformed qualification rather than enabling cache reuse', async () => {
  for (const readingContext of [null, { sourceIdentity: '' }, { chineseConversion: { available: true, modes: ['unknown'] } }]) {
    vi.mocked(request).mockResolvedValue({ id: 'book', readingContext });
    await expect(getBook('book')).rejects.toThrow();
  }
});
