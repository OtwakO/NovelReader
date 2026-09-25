import { getChapterContent, type ReadingContent } from '../../api/reader';
import { readerRequestSignal } from '../../api/transport';
import { chapterCache, type ChapterCacheSession } from './chapter-cache';
import { isReaderRevisionConflict, ReaderRevisionConflict } from './reader-session';

/** One validated reader/book lifetime; the cache owns retention and freshness. */
export function createChapterLoader(bookId: string, contentRevision: number, onRevisionConflict?: () => void, cache: ChapterCacheSession = chapterCache.ephemeral(bookId, contentRevision)) {
  const owner = readerRequestSignal();
  const pending = new Map<number, Promise<ReadingContent>>();
  const refreshing = new Set<number>();
  const controller = new AbortController();
  let closed = false;
  let tail = Promise.resolve();
  let speculative: Promise<void> | null = null;

  function checkCurrent() {
    owner.throwIfAborted();
    if (closed) throw new DOMException('Reader session closed', 'AbortError');
  }
  function load(index: number, refresh = false): Promise<ReadingContent> {
    try { checkCurrent(); } catch (cause) { return Promise.reject(cause); }
    const existing = pending.get(index);
    if (existing && (!refresh || refreshing.has(index))) return existing;
    const cached = !refresh && cache.peek(index);
    if (cached) return Promise.resolve(cached);
    const operation = tail.then(async () => {
      checkCurrent();
      if (refresh) await cache.refresh(index);
      const lookup = await cache.lookup(index, refresh);
      checkCurrent();
      if (lookup.content) return lookup.content;
      const started = { wall: Date.now(), monotonic: performance.now() };
      const content = refresh
        ? await getChapterContent(bookId, index, contentRevision, controller.signal, true)
        : await getChapterContent(bookId, index, contentRevision, controller.signal);
      checkCurrent();
      if (content.contentRevision !== contentRevision) throw new ReaderRevisionConflict();
      cache.accept(index, content, lookup.ticket, started);
      return content;
    }).catch(cause => {
      if (!closed && isReaderRevisionConflict(cause)) onRevisionConflict?.();
      throw cause;
    }).finally(() => {
      if (pending.get(index) === operation) { pending.delete(index); refreshing.delete(index); }
    });
    pending.set(index, operation);
    if (refresh) refreshing.add(index);
    tail = operation.then(() => undefined, () => undefined);
    return operation;
  }

  function prefetch(index: number): void {
    if (owner.aborted || closed || speculative || pending.size || cache.peek(index)) return;
    speculative = load(index).then(() => undefined, () => undefined).finally(() => { speculative = null; });
  }
  function dispose(abort = false): Promise<void> {
    closed = true;
    cache.dispose();
    if (abort) controller.abort();
    return tail;
  }
  return { load, refresh: (index: number) => load(index, true), prefetch, dispose, commit: cache.commit, assertCurrent: cache.assertCurrent };
}
