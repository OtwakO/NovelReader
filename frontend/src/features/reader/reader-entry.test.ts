import { IDBFactory } from 'fake-indexeddb';
import { beforeEach, expect, it, vi } from 'vitest';
import { getBook, type LibraryBook } from '../../api/books';
import { getChapterContent, waitForCatalog, type ChapterCatalog, type ChapterContent } from '../../api/reader';
import { readerHomeGeneration } from '../../api/transport';
import { convertChineseTexts } from '../../api/system';
import { chapterCache, ChapterCacheInvalidated } from './chapter-cache';
import { createChapterLoader } from './chapter-loader';
import { loadReaderSnapshot, loadValidatedCatalog } from './reader-session';

vi.mock('../../api/books', () => ({ getBook: vi.fn() }));
vi.mock('../../api/reader', async original => ({ ...await original<typeof import('../../api/reader')>(), getChapterContent: vi.fn(), waitForCatalog: vi.fn() }));
vi.mock('../../api/transport', async original => ({ ...await original<typeof import('../../api/transport')>(), readerHomeGeneration: vi.fn(() => 'home') }));
vi.mock('../../api/system', () => ({ convertChineseTexts: vi.fn(async (_mode, texts: string[]) => texts.map(text => `Converted ${text}`)) }));
vi.stubGlobal('indexedDB', new IDBFactory());
const capability = { available: true, engine: 'OpenCC', version: '1', modes: ['traditional' as const] };
const book = { id: 'entry', provider: 'booksource', contentRevision: 2, stateVersion: 0, durChapterIndex: 0, readingContext: { sourceIdentity: 'rules-a', chineseConversion: capability } } as LibraryBook;
const catalog: ChapterCatalog = { contentRevision: 2, sourceIdentity: 'rules-a', chapters: [{ index: 0, title: 'Chapter', isVolume: false }] };
const content: ChapterContent = { version: 1, contentRevision: 2, sourceIdentity: 'rules-a', freshForMs: 60_000, offlineCopy: false, document: { kind: 'prose', title: 'Chapter', blocks: [{ kind: 'paragraph', text: 'Story' }] } };
const identity = { bookId: 'entry', homeGeneration: 'home', provider: 'booksource', revision: 2, sourceIdentity: 'rules-a' };
beforeEach(async () => {
  vi.clearAllMocks();
  vi.mocked(readerHomeGeneration).mockReturnValue('home');
  vi.mocked(getBook).mockResolvedValue(book);
  vi.mocked(waitForCatalog).mockResolvedValue(catalog);
  vi.mocked(getChapterContent).mockResolvedValue(content);
  await chapterCache.useReader('reader', true);
});

it('warm entry validates once, then reuses catalog, chapter and prepared display', async () => {
  const first = await loadReaderSnapshot(book.id);
  const session = await chapterCache.bind(identity, first.catalog.chapters, first.cacheValidation, 0);
  const loader = createChapterLoader(book.id, 2, undefined, session);
  const document = await loader.load(0);
  loader.commit(0, document);
  const convert = chapterCache.displayConverter(capability);
  const display = await convert(first.catalog.chapters, document, 'traditional');
  await loader.dispose();
  const next = await loadReaderSnapshot(book.id);
  const reopened = await chapterCache.bind(identity, next.catalog.chapters, next.cacheValidation, 0);
  expect(next.catalog).toBe(first.catalog);
  expect(reopened.peek(0)).toBe(document);
  expect(await chapterCache.displayConverter(capability)(next.catalog.chapters, reopened.peek(0)!, 'traditional')).toEqual(display);
  expect(getBook).toHaveBeenCalledTimes(2);
  expect(waitForCatalog).toHaveBeenCalledTimes(1);
  expect(getChapterContent).toHaveBeenCalledTimes(1);
  expect(convertChineseTexts).toHaveBeenCalledTimes(2); // catalog + document, once each
  const refreshLoader = createChapterLoader(book.id, 2, undefined, reopened);
  vi.mocked(getChapterContent).mockResolvedValueOnce({ ...content, document: { ...content.document, title: 'Refreshed' } });
  const fresh = await refreshLoader.refresh(0);
  expect((await convert(next.catalog.chapters, fresh, 'traditional')).content?.document.title).toBe('Converted Refreshed');
  expect(convertChineseTexts).toHaveBeenCalledTimes(3); // Refresh converts the new document, not the unchanged catalog.
  expect((await loadReaderSnapshot(book.id)).catalog).toBe(first.catalog);
  expect(chapterCache.displayConverter({ ...capability, version: '2' })).not.toBe(convert);
  await refreshLoader.dispose();
});

it('shares qualified pending Detail/Reader work without letting a departed consumer cancel the reader', async () => {
  let release!: (catalog: ChapterCatalog) => void;
  let detailCurrent = true;
  vi.mocked(waitForCatalog).mockReturnValue(new Promise(resolve => { release = resolve; }));
  const validation = await chapterCache.beginValidation();
  const detail = loadValidatedCatalog(book, validation, { isCurrent: () => detailCurrent });
  const detailResult = expect(detail).rejects.toThrow('superseded');
  const reader = loadReaderSnapshot(book.id);
  await vi.waitFor(() => expect(waitForCatalog).toHaveBeenCalledOnce());
  detailCurrent = false;
  // The shared poll must still have a live Reader consumer before completion.
  await vi.waitFor(() => expect(vi.mocked(waitForCatalog).mock.calls[0]?.[1]?.isCurrent?.()).toBe(true));
  release(catalog);
  await detailResult;
  expect((await reader).catalog).toEqual(catalog);
  expect(waitForCatalog).toHaveBeenCalledOnce();
});

it('rejects invalidated in-flight catalogs and misses after definition/home replacement', async () => {
  const first = await loadReaderSnapshot(book.id);
  const session = await chapterCache.bind(identity, catalog.chapters, first.cacheValidation, 0);
  const lookup = await session.lookup(0);
  session.accept(0, content, lookup.ticket, { wall: Date.now(), monotonic: performance.now() }); session.commit(0, content);
  await session.settled();
  const replacement = { ...book, readingContext: { ...book.readingContext!, sourceIdentity: 'rules-b' } };
  vi.mocked(getBook).mockResolvedValue(replacement);
  vi.mocked(waitForCatalog).mockResolvedValue({ ...catalog, sourceIdentity: 'rules-b' });
  const next = await loadReaderSnapshot(book.id);
  expect(next.catalog.sourceIdentity).toBe('rules-b');
  expect(() => session.assertCurrent(content)).toThrow(ChapterCacheInvalidated);
  // Qualification returned after internal retirement must still bind successfully.
  (await chapterCache.bind({ ...identity, sourceIdentity: 'rules-b' }, next.catalog.chapters, next.cacheValidation, 0)).dispose();
  vi.mocked(readerHomeGeneration).mockReturnValue('replacement-home');
  await loadReaderSnapshot(book.id);
  expect(waitForCatalog).toHaveBeenCalledTimes(3);
  await chapterCache.invalidate({ bookId: book.id });
  let release!: (catalog: ChapterCatalog) => void;
  vi.mocked(waitForCatalog).mockReturnValue(new Promise(resolve => { release = resolve; }));
  const pending = loadReaderSnapshot(book.id);
  const result = expect(pending).rejects.toBeInstanceOf(ChapterCacheInvalidated);
  await vi.waitFor(() => expect(waitForCatalog).toHaveBeenCalledTimes(4));
  await chapterCache.invalidate({ bookId: book.id });
  release({ ...catalog, sourceIdentity: 'rules-b' });
  await result;
});
