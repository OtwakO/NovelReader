import { getBook, type LibraryBook } from '../../api/books';
import { waitForCatalog, type CatalogPollingOptions } from '../../api/reader';
import { ApiError } from '../../api/transport';

export class ReaderRevisionConflict extends Error {
  constructor() { super('Reading state changed; reopen the current saved location'); }
}
export class ReaderCatalogError extends Error {
  constructor(cause: unknown, readonly book: LibraryBook) { super(cause instanceof Error ? cause.message : 'Chapter list unavailable', { cause }); }
}
export function isReaderRevisionConflict(cause: unknown): boolean {
  return cause instanceof ReaderRevisionConflict || (cause instanceof ApiError && cause.code === 'state_changed');
}

// State and catalog are separate HTTP resources. Retry their pair once, never
// combine saved ordinals from one interpretation with another interpretation's TOC.
export async function loadReaderSnapshot(bookId: string, options: CatalogPollingOptions = {}) {
  for (let attempt = 0; attempt < 2; attempt++) {
    const [book, result] = await Promise.all([
      getBook(bookId),
      waitForCatalog(bookId, { ...options, retry: attempt === 0 && options.retry })
        .then(catalog => ({ catalog, error: undefined }), error => ({ catalog: undefined, error })),
    ]);
    if (options.isCurrent && !options.isCurrent()) throw new DOMException('Reader superseded', 'AbortError');
    if (!result.catalog) throw new ReaderCatalogError(result.error, book);
    const catalog = result.catalog;
    if (book.contentRevision === catalog.contentRevision) return { book, catalog };
  }
  throw new ReaderRevisionConflict();
}

/** Missing qualification is a legacy current-catalog link, not historical recovery. */
export function checkReaderLink(value: unknown, contentRevision: number): void {
  if (value === undefined) return;
  if (typeof value !== 'string' || !/^\d+$/.test(value) || Number(value) !== contentRevision) throw new ReaderRevisionConflict();
}

export function readerLocation(bookId: string, chapterIndex: number, contentRevision: number, position?: number) {
  return { name: 'reader', params: { bookId, chapterIndex }, query: { contentRevision: String(contentRevision), ...(position === undefined ? {} : { position: String(position) }) } };
}

// A book awaiting its first catalog has no section location to qualify. Open its
// current saved resume after synchronization instead of inventing a revision-0 URL.
export function readerResumeLocation(book: LibraryBook) {
  return book.totalChapterNum > 0
    ? readerLocation(book.id, book.durChapterIndex, book.contentRevision, book.durChapterPos)
    : { name: 'reader', params: { bookId: book.id } };
}
