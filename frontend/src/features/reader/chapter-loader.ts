import { getChapterContent, type ReadingContent } from '../../api/reader';
import { readerRequestSignal } from '../../api/transport';

import { isReaderRevisionConflict, ReaderRevisionConflict } from './reader-session';

const maxRecentChapters = 5;

/** One reader/book interpretation revision. Dispose and drain before replacing that binding. */
export function createChapterLoader(bookId: string, contentRevision: number, onRevisionConflict?: () => void) {
  const owner = readerRequestSignal();
  const cache = new Map<number, ReadingContent>();
  const pending = new Map<number, Promise<ReadingContent>>();
  const controller = new AbortController();
  let closed = false;
  let tail = Promise.resolve();
  let speculative: Promise<void> | null = null;

  function load(index: number): Promise<ReadingContent> {
    if (owner.aborted) return Promise.reject(owner.reason);
    if (closed) return Promise.reject(new DOMException('Reader session closed', 'AbortError'));
    const cached = cache.get(index);
    if (cached) {
      cache.delete(index);
      cache.set(index, cached);
      return Promise.resolve(cached);
    }
    const existing = pending.get(index);
    if (existing) return existing;

    // Chapter scripts share source-session state: do not overlap speculative and foreground fetches.
    const operation = tail.then(async () => {
      owner.throwIfAborted();
      if (closed) throw new DOMException('Reader session closed', 'AbortError');
      const content = await getChapterContent(bookId, index, contentRevision, controller.signal);
      owner.throwIfAborted();
      if (closed) throw new DOMException('Reader session closed', 'AbortError');
      if (content.contentRevision !== contentRevision) throw new ReaderRevisionConflict();
      if (!content.offlineCopy) {
        cache.set(index, content);
        if (cache.size > maxRecentChapters) cache.delete(cache.keys().next().value!);
      }
      return content;
    }).catch(cause => {
      if (!closed && isReaderRevisionConflict(cause)) onRevisionConflict?.();
      throw cause;
    }).finally(() => { pending.delete(index); });
    pending.set(index, operation);
    tail = operation.then(() => undefined, () => undefined);
    return operation;
  }

  function prefetch(index: number): void {
    if (owner.aborted || closed || speculative || pending.size || cache.has(index)) return;
    // Speculative errors are deliberately non-blocking; a foreground visit can retry normally.
    speculative = load(index).then(() => undefined, () => undefined).finally(() => { speculative = null; });
  }

  function dispose(abort = false): Promise<void> {
    closed = true;
    cache.clear();
    if (abort) controller.abort();
    // Source switching/refetch waits for started work to finish, avoiding late source-state writes.
    return tail;
  }

  return { load, prefetch, dispose };
}
