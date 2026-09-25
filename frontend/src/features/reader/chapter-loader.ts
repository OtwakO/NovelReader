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
  let preparing = false;
  let preparation = 0;
  let targets: number[] = [];
  let warm: (content: ReadingContent) => Promise<unknown> = async () => undefined;
  let attempted = new Set<number>();
  let failed = new Set<number>();
  let timer: ReturnType<typeof setTimeout> | undefined;

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
      queueMicrotask(pump);
    });
    pending.set(index, operation);
    if (refresh) refreshing.add(index);
    tail = operation.then(() => undefined, () => undefined);
    return operation;
  }

  // Only the selected window feeds this pump. Completion never derives more targets.
  // Foreground requests enter the shared queue before any not-yet-started speculation.
  function pump(): void {
    if (owner.aborted || closed || preparing || pending.size) return;
    clearTimeout(timer);
    // A target may expire while another fetch/conversion owns the pump.
    for (const target of targets) if (!failed.has(target) && cache.remaining(target) === 0) attempted.delete(target);
    const index = targets.find(target => !attempted.has(target));
    if (index === undefined) {
      const deadlines = targets.filter(target => !failed.has(target)).map(target => cache.remaining(target)).filter((value): value is number => value !== undefined && Number.isFinite(value) && value > 0);
      if (deadlines.length) timer = setTimeout(pump, Math.ceil(Math.min(...deadlines)));
      return;
    }
    const generation = preparation;
    const convert = warm;
    attempted.add(index);
    preparing = true;
    void load(index).then(content => {
      if (generation === preparation && !closed) return convert(content);
    }).catch(() => {
      // A failure stays attempted until a new window/preference/visibility event.
      if (generation === preparation) failed.add(index);
    }).finally(() => { preparing = false; pump(); });
  }
  function prepare(indices: number[], convert: (content: ReadingContent) => Promise<unknown>): void {
    preparation++;
    targets = indices;
    warm = convert;
    attempted = new Set();
    failed = new Set();
    clearTimeout(timer);
    pump();
  }
  function dispose(abort = false): Promise<void> {
    closed = true;
    targets = [];
    preparation++;
    clearTimeout(timer);
    cache.dispose();
    if (abort) controller.abort();
    return tail;
  }
  return { load, refresh: (index: number) => load(index, true), prepare, dispose, commit: cache.commit, assertCurrent: cache.assertCurrent };
}
