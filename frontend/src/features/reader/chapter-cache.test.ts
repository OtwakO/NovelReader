import { IDBFactory } from 'fake-indexeddb';
import { afterEach, expect, it, vi } from 'vitest';
import type { Chapter } from '../../api/models';
import type { ChapterContent } from '../../api/reader';
import { ChapterCache, ChapterCacheInvalidated } from './chapter-cache';
import { ChapterCacheStorage } from './chapter-cache-storage';

const chapters = [0, 1, 2, 3, 4, 5, 6].map(index => ({ index, title: String(index), isVolume: false })) as Chapter[];
const identity = { homeGeneration: 'home', bookId: 'book', revision: 1, provider: 'txt' as const };
const content: ChapterContent = { version: 1, contentRevision: 1, offlineCopy: false, document: { kind: 'prose', title: 'Synthetic', blocks: [{ kind: 'paragraph', text: 'Readable' }] } };
const started = () => ({ wall: Date.now(), monotonic: performance.now() });
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it('persists canonical committed/window content across readers without treating speculation as a visit', async () => {
  const factory = new IDBFactory();
  const storage = new ChapterCacheStorage('cache', factory);
  const cache = new ChapterCache(storage);
  await cache.useReader('reader');
  const session = await cache.bind(identity, chapters, await cache.beginValidation(), 0);
  const first = await session.lookup(0);
  session.accept(0, content, first.ticket, started());
  session.commit(0, content);
  const forward = { ...content, document: { ...content.document, title: 'Next' } };
  const second = await session.lookup(1);
  session.accept(1, forward, second.ticket, started());
  await session.settled();
  session.dispose();
  await storage.close();

  const reopenedStorage = new ChapterCacheStorage('cache', factory);
  const reopened = new ChapterCache(reopenedStorage);
  await reopened.useReader('reader');
  const reader = await reopened.bind(identity, chapters, await reopened.beginValidation(), 0);
  expect((await reader.lookup(0)).content).toEqual(content);
  expect((await reader.lookup(1)).content).toEqual(forward);
  expect((await reader.lookup(6)).content).toBeUndefined();
  await reopenedStorage.close();
});

it('does not renew persisted freshness or reuse a different source definition', async () => {
  const wall = vi.spyOn(Date, 'now').mockReturnValue(1000);
  vi.spyOn(performance, 'now').mockReturnValue(1000);
  const storage = new ChapterCacheStorage('freshness', new IDBFactory());
  const cache = new ChapterCache(storage);
  await cache.useReader('reader');
  const source = { ...identity, provider: 'booksource' as const, sourceIdentity: 'rules-a' };
  const session = await cache.bind(source, chapters, await cache.beginValidation(), 0);
  const document = { ...content, sourceIdentity: 'rules-a', freshForMs: 1000 };
  const lookup = await session.lookup(0);
  session.accept(0, document, lookup.ticket, started()); session.commit(0, document);
  await session.settled();
  wall.mockReturnValue(2000);
  expect(session.peek(0)).toBeUndefined();
  expect((await session.lookup(0)).content).toBeUndefined();
  const other = await cache.bind({ ...source, sourceIdentity: 'rules-b' }, chapters, await cache.beginValidation(), 0);
  expect((await other.lookup(0)).content).toBeUndefined();
  expect(() => session.assertCurrent(document)).toThrow(ChapterCacheInvalidated);
  await storage.close();
});

it('rejects a response and entry validation retired by invalidation', async () => {
  const storage = new ChapterCacheStorage('invalidation', new IDBFactory());
  const cache = new ChapterCache(storage);
  await cache.useReader('reader');
  const validation = await cache.beginValidation();
  const session = await cache.bind(identity, chapters, validation, 0);
  const lookup = await session.lookup(0);
  await cache.invalidate({ bookId: 'book' });
  expect(() => session.accept(0, content, lookup.ticket, started())).toThrow(ChapterCacheInvalidated);
  await expect(cache.bind(identity, chapters, validation, 0)).rejects.toThrow(ChapterCacheInvalidated);
  await storage.close();
});

it('retries quota pressure once and degrades to memory when storage becomes unavailable', async () => {
  const storage = new ChapterCacheStorage('failure', new IDBFactory());
  const cache = new ChapterCache(storage);
  await cache.useReader('reader');
  const session = await cache.bind(identity, chapters, await cache.beginValidation(), 0);
  const lookup = await session.lookup(0);
  const put = vi.spyOn(storage, 'put').mockRejectedValueOnce(new DOMException('full', 'QuotaExceededError'));
  const prune = vi.spyOn(storage, 'evictInactive');
  session.accept(0, content, lookup.ticket, started()); session.commit(0, content);
  await session.settled();
  expect(put).toHaveBeenCalledTimes(2);
  expect(prune).toHaveBeenCalledTimes(1);
  vi.spyOn(storage, 'capture').mockRejectedValueOnce(new Error('disk unavailable'));
  const warning = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
  const next = await session.lookup(1);
  const nextContent = { ...content };
  session.accept(1, nextContent, next.ticket, started());
  expect(session.peek(1)).toBe(nextContent);
  expect(warning).toHaveBeenCalledOnce();
  await storage.close();
});

it.each(['refresh', 'other-book', 'other-tab'] as const)('retains the new disk window after %s invalidation', async (invalidation) => {
  const channels: { onmessage?: (event: { data: unknown }) => void }[] = [];
  vi.stubGlobal('BroadcastChannel', class {
    onmessage?: (event: { data: unknown }) => void;
    constructor() { channels.push(this); }
    postMessage(data: unknown) { for (const channel of channels) if (channel !== this) channel.onmessage?.({ data }); }
  });
  const factory = new IDBFactory();
  const storage = new ChapterCacheStorage('refresh-window', factory);
  const cache = new ChapterCache(storage);
  await cache.useReader('reader');
  const session = await cache.bind(identity, chapters, await cache.beginValidation(), 0);
  for (const index of [0, 1, 2]) {
    const lookup = await session.lookup(index);
    const chapter = { ...content };
    session.accept(index, chapter, lookup.ticket, started());
    if (index === 0) session.commit(index, chapter);
  }
  await session.settled();
  if (invalidation === 'other-tab') {
    const otherStorage = new ChapterCacheStorage('refresh-window', factory);
    const other = new ChapterCache(otherStorage);
    await other.useReader('reader');
    await other.invalidate({ bookId: 'book', index: 0 });
    await otherStorage.close();
  } else if (invalidation === 'other-book') await cache.invalidate({ bookId: 'another-book' });
  else await session.refresh(0);
  const neighbor = session.peek(1)!;
  expect(neighbor).toBeDefined();
  session.commit(1, neighbor);
  const next = await session.lookup(3);
  session.accept(3, { ...content }, next.ticket, started());
  await session.settled();
  const reopenedStorage = new ChapterCacheStorage('refresh-window', factory);
  const reopened = new ChapterCache(reopenedStorage);
  await reopened.useReader('reader');
  const reader = await reopened.bind(identity, chapters, await reopened.beginValidation(), 1);
  expect((await reader.lookup(3)).content).toEqual(content);
  await reopenedStorage.close();
  await storage.close();
});
