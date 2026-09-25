import { getChapterContent, type ReadingContent } from '../../api/reader';
import { readerRequestSignal } from '../../api/transport';

import { isReaderRevisionConflict, ReaderRevisionConflict } from './reader-session';

const maxRecentChapters = 5;

/** One reader/book interpretation revision. Dispose and drain before replacing that binding. */
export function createChapterLoader(bookId: string, contentRevision: number, onRevisionConflict?: () => void) {
  const owner = readerRequestSignal();
  const cache = new Map<number, { content: ReadingContent; expiresAt: number; expiresAtWall: number }>();
  const pending = new Map<number, Promise<ReadingContent>>();
  const controller = new AbortController();
  let closed = false;
  let tail = Promise.resolve();
  let speculative: Promise<void> | null = null;

  function load(index: number, refresh = false): Promise<ReadingContent> {
    if (owner.aborted) return Promise.reject(owner.reason);
    if (closed) return Promise.reject(new DOMException('Reader session closed', 'AbortError'));
    const existing = pending.get(index);
    if (existing && !refresh) return existing;
    const cached = cache.get(index);
    if (!refresh && cached && performance.now() < cached.expiresAt && Date.now() < cached.expiresAtWall) {
      cache.delete(index);
      cache.set(index, cached);
      return Promise.resolve(cached.content);
    }
    cache.delete(index);

    // Chapter scripts share source-session state: do not overlap speculative and foreground fetches.
    const operation = tail.then(async () => {
      owner.throwIfAborted();
      if (closed) throw new DOMException('Reader session closed', 'AbortError');
      // Earlier queued work may have repopulated this entry since Refresh was requested.
      if (refresh) cache.delete(index);
      const startedAt = performance.now();
      const startedAtWall = Date.now();
      const content = refresh
        ? await getChapterContent(bookId, index, contentRevision, controller.signal, true)
        : await getChapterContent(bookId, index, contentRevision, controller.signal);
      owner.throwIfAborted();
      if (closed) throw new DOMException('Reader session closed', 'AbortError');
      if (content.contentRevision !== contentRevision) throw new ReaderRevisionConflict();
      // Subtract the whole request duration conservatively; receipt time must
      // not extend the backend's image-resource/freshness promise.
      const expiresAt = content.version === 1 && content.freshForMs !== undefined ? startedAt + content.freshForMs : Infinity;
      // Wall time also covers platforms whose monotonic clock pauses in sleep.
      const expiresAtWall = content.version === 1 && content.freshForMs !== undefined ? startedAtWall + content.freshForMs : Infinity;
      const unavailableImages = content.version === 1 && content.document.blocks.some(block => block.kind === 'image' && block.resource.unavailable);
      if (!content.offlineCopy && !unavailableImages && performance.now() < expiresAt && Date.now() < expiresAtWall) {
        cache.set(index, { content, expiresAt, expiresAtWall });
        if (cache.size > maxRecentChapters) cache.delete(cache.keys().next().value!);
      }
      return content;
    }).catch(cause => {
      if (!closed && isReaderRevisionConflict(cause)) onRevisionConflict?.();
      throw cause;
    }).finally(() => { if (pending.get(index) === operation) pending.delete(index); });
    pending.set(index, operation);
    tail = operation.then(() => undefined, () => undefined);
    return operation;
  }

  function prefetch(index: number): void {
    if (owner.aborted || closed || speculative || pending.size) return;
    const cached = cache.get(index);
    if (cached && performance.now() < cached.expiresAt && Date.now() < cached.expiresAtWall) return;
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

  return { load, refresh: (index: number) => load(index, true), prefetch, dispose };
}
