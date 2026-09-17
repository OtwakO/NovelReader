import { beforeEach, expect, it, vi } from 'vitest';
import { getBook, type LibraryBook } from '../../api/books';
import { waitForCatalog } from '../../api/reader';
import { checkReaderLink, loadReaderSnapshot, readerLocation, readerResumeLocation, ReaderRevisionConflict } from './reader-session';

vi.mock('../../api/books', () => ({ getBook: vi.fn() }));
vi.mock('../../api/reader', () => ({ waitForCatalog: vi.fn() }));
const book = { id: 'txt', provider: 'txt', contentRevision: 2, durChapterIndex: 3, durChapterPos: .4 } as LibraryBook;
beforeEach(() => vi.resetAllMocks());

it('retries a mixed pair once and uses only the coherent saved location', async () => {
  vi.mocked(getBook).mockResolvedValueOnce({ ...book, contentRevision: 1, durChapterIndex: 19 }).mockResolvedValue(book);
  vi.mocked(waitForCatalog).mockResolvedValue({ contentRevision: 2, chapters: [] });
  expect((await loadReaderSnapshot('txt')).book).toEqual(book);
  expect(getBook).toHaveBeenCalledTimes(2);
  vi.mocked(getBook).mockResolvedValue({ ...book, contentRevision: 1 });
  await expect(loadReaderSnapshot('txt')).rejects.toBeInstanceOf(ReaderRevisionConflict);
  expect(getBook).toHaveBeenCalledTimes(4);
});

it('accepts unqualified current-catalog links but never reuses stale or malformed qualification', () => {
  expect(readerResumeLocation({ ...book, totalChapterNum: 0 })).toEqual({ name: 'reader', params: { bookId: 'txt' } });
  expect(readerResumeLocation({ ...book, totalChapterNum: 4 })).toEqual({ name: 'reader', params: { bookId: 'txt', chapterIndex: 3 }, query: { contentRevision: '2', position: '0.4' } });
  expect(() => checkReaderLink(undefined, 2)).not.toThrow();
  expect(() => checkReaderLink('2', 2)).not.toThrow();
  for (const revision of ['1', '', ['2'], '2.0']) expect(() => checkReaderLink(revision, 2)).toThrow(ReaderRevisionConflict);
});

it('qualifies anchor links and preserves note intent for a new tab or reload', () => {
  expect(readerLocation('book', 2, 7, undefined, { anchor: 'a1', note: true })).toEqual({
    name: 'reader', params: { bookId: 'book', chapterIndex: 2 }, query: { contentRevision: '7', anchor: 'a1', note: '1' },
  });
});
