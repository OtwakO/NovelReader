import { getBook, type LibraryBook } from '../../api/books';
import { waitForCatalog, type ChapterCatalog, type CatalogPollingOptions } from '../../api/reader';
import { chapterCache, ChapterCacheInvalidated, type CacheValidation } from './chapter-cache';
import { ApiError, readerHomeGeneration } from '../../api/transport';

export class ReaderRevisionConflict extends Error {
  constructor() { super('Reading state changed; reopen the current saved location'); }
}
export class ReaderCatalogError extends Error {
  constructor(cause: unknown, readonly book: LibraryBook) { super(cause instanceof Error ? cause.message : 'Chapter list unavailable', { cause }); }
}
export function isReaderRevisionConflict(cause: unknown): boolean {
  return cause instanceof ReaderRevisionConflict || cause instanceof ChapterCacheInvalidated || (cause instanceof ApiError && cause.code === 'state_changed');
}

// Only pending network work is shared here; completed catalogs belong to the cache.
const pendingCatalogs = new Map<string, { consumers: Set<() => boolean>; promise: Promise<ChapterCatalog> }>();
async function sharedCatalog(key: string, bookId: string, options: CatalogPollingOptions) {
  const current = () => options.isCurrent?.() !== false;
  let pending = pendingCatalogs.get(key);
  if (!pending) {
    const consumers = new Set([current]);
    const promise = waitForCatalog(bookId, { retry: options.retry, isCurrent: () => [...consumers].some(check => check()) })
      .finally(() => pendingCatalogs.delete(key));
    pending = { consumers, promise };
    pendingCatalogs.set(key, pending);
  } else pending.consumers.add(current);
  try { return await pending.promise; }
  finally { pending.consumers.delete(current); }
}

export async function loadValidatedCatalog(book: LibraryBook, validation: CacheValidation, options: CatalogPollingOptions = {}) {
  const identity = { bookId: book.id, provider: book.provider, revision: book.contentRevision, homeGeneration: readerHomeGeneration() ?? '', sourceIdentity: book.readingContext?.sourceIdentity };
  if (!book.readingContext) return { catalog: await waitForCatalog(book.id, options), cacheValidation: validation };
  const cached = await chapterCache.catalog(identity, validation);
  const key = JSON.stringify([identity, cached.validation, Boolean(options.retry)]);
  const catalog = !options.retry && cached.catalog ? cached.catalog : await sharedCatalog(key, book.id, options);
  if (options.isCurrent?.() === false) throw new DOMException('Reader superseded', 'AbortError');
  if (catalog !== cached.catalog) await cached.accept(catalog);
  return { catalog, cacheValidation: cached.validation };
}

// Validate fresh state before reuse. Retry mismatched state/catalog qualification
// once; never combine saved ordinals with another interpretation's TOC.
export async function loadReaderSnapshot(bookId: string, options: CatalogPollingOptions = {}) {
  for (let attempt = 0; attempt < 2; attempt++) {
    const cacheValidation = await chapterCache.beginValidation();
    const book = await getBook(bookId);
    if (options.isCurrent?.() === false) throw new DOMException('Reader superseded', 'AbortError');
    let result;
    try { result = await loadValidatedCatalog(book, cacheValidation, { ...options, retry: attempt === 0 && options.retry }); }
    catch (cause) {
      if (isReaderRevisionConflict(cause)) throw cause;
      throw new ReaderCatalogError(cause, book);
    }
    if (options.isCurrent?.() === false) throw new DOMException('Reader superseded', 'AbortError');
    const { catalog } = result;
    if (book.contentRevision === catalog.contentRevision && (!book.readingContext || book.readingContext.sourceIdentity === catalog.sourceIdentity)) return { book, ...result, homeGeneration: readerHomeGeneration() };
  }
  throw new ReaderRevisionConflict();
}

/** Missing qualification is a legacy current-catalog link, not historical recovery. */
export function checkReaderLink(value: unknown, contentRevision: number): void {
  if (value === undefined) return;
  if (typeof value !== 'string' || !/^\d+$/.test(value) || Number(value) !== contentRevision) throw new ReaderRevisionConflict();
}

export function readerLocation(bookId: string, chapterIndex: number, contentRevision: number, position?: number, visit?: { anchor?: string; note?: boolean }) {
  return { name: 'reader', params: { bookId, chapterIndex }, query: { contentRevision: String(contentRevision), ...(position === undefined ? {} : { position: String(position) }), ...(visit?.anchor === undefined ? {} : { anchor: visit.anchor }), ...(visit?.note ? { note: '1' } : {}) } };
}

// A book awaiting its first catalog has no section location to qualify. Open its
// current saved resume after synchronization instead of inventing a revision-0 URL.
export function readerResumeLocation(book: LibraryBook) {
  return book.totalChapterNum > 0
    ? readerLocation(book.id, book.durChapterIndex, book.contentRevision, book.durChapterPos)
    : { name: 'reader', params: { bookId: book.id } };
}
