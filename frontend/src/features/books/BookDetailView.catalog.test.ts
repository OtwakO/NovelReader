import { IDBFactory } from 'fake-indexeddb';
import { afterEach, expect, it, vi } from 'vitest';
import BookDetailView from './BookDetailView.vue';
import { getBook, type LibraryBook } from '../../api/books';
import { waitForCatalog } from '../../api/reader';
import { chapterCache } from '../reader/chapter-cache';

vi.mock('../../api/books', async original => ({ ...await original<typeof import('../../api/books')>(), getBook: vi.fn() }));
vi.mock('../../api/reader', async original => ({ ...await original<typeof import('../../api/reader')>(), waitForCatalog: vi.fn() }));
vi.mock('../../api/transport', async original => ({ ...await original<typeof import('../../api/transport')>(), readerHomeGeneration: () => 'home' }));
vi.stubGlobal('indexedDB', new IDBFactory());
afterEach(() => vi.clearAllMocks());

it.each(['book', 'another-book'])('revalidates catalog retry after invalidating %s', async bookId => {
  await chapterCache.useReader('reader', true);
  const book = { id: 'book', provider: 'txt', contentRevision: 1, readingContext: { chineseConversion: { available: false, modes: [] } } } as unknown as LibraryBook;
  const vm = {
    bookId: book.id, book, cacheValidation: await chapterCache.beginValidation(),
    loadGeneration: 1, tocError: '', chapters: [], catalogRevision: 0,
    $t: (key: string) => key,
  };
  vi.mocked(waitForCatalog).mockRejectedValueOnce(new Error('temporary catalog failure'));
  await BookDetailView.methods!.loadCatalog.call(vm as never);
  expect(vm.tocError).toBe('temporary catalog failure');
  await chapterCache.invalidate({ bookId });
  const current = { ...book, contentRevision: 2 };
  const chapters = [{ index: 0, title: 'Current chapter', isVolume: false }];
  vi.mocked(getBook).mockResolvedValue(current);
  vi.mocked(waitForCatalog).mockResolvedValue({ contentRevision: 2, chapters });
  await BookDetailView.methods!.loadCatalog.call(vm as never, true);
  expect(vm.tocError).toBe('');
  expect(getBook).toHaveBeenCalledExactlyOnceWith(book.id);
  expect(vm.book).toBe(current);
  expect(vm.catalogRevision).toBe(2);
  expect(vm.chapters).toEqual(chapters);
});
