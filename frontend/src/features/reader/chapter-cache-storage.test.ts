import { IDBFactory } from 'fake-indexeddb';
import { describe, expect, it } from 'vitest';
import { ChapterCacheStorage } from './chapter-cache-storage';
import { cacheScope, type ChapterCacheIdentity, type SavedChapter } from './chapter-cache-policy';

const identity: ChapterCacheIdentity = { readerId: 'reader', homeGeneration: 'home', bookId: 'book', revision: 1, provider: 'txt' };
function entry(bookId = 'book', index = 0): SavedChapter {
  const owner = { ...identity, bookId };
  const scope = cacheScope(owner);
  return { id: JSON.stringify([scope, index]), scope, identity: owner, index, storedAt: Date.now(), expiresAt: null,
    content: { version: 1, contentRevision: 1, offlineCopy: false, document: { kind: 'prose', title: 'Synthetic', blocks: [{ kind: 'paragraph', text: 'A chapter' }] } } };
}

describe('persistent chapter ownership', () => {
  it('reuses across connections but rejects a late write after another tab invalidates', async () => {
    const factory = new IDBFactory();
    const first = new ChapterCacheStorage('cache', factory);
    const other = new ChapterCacheStorage('cache', factory);
    const epoch = await first.useReader('reader');
    const value = entry();
    await first.retain('reader', { scope: value.scope, window: [0] }, epoch);
    expect(await first.put(value, epoch)).toBe(true);
    expect(await other.get(value.scope, 0, epoch)).toEqual(value);
    await other.invalidate({ bookId: 'book', index: 0 });
    expect(await first.put(value, epoch)).toBe(false);
    const next = await other.capture('reader');
    expect(await other.get(value.scope, 0, next!)).toBeUndefined();
    await first.close(); await other.close();
  });

  it('retains three foreground books and their windows without speculative resurrection', async () => {
    const storage = new ChapterCacheStorage('retention', new IDBFactory());
    const epoch = await storage.useReader('reader');
    for (const bookId of ['a', 'b', 'c', 'd']) {
      const value = entry(bookId);
      await storage.retain('reader', { scope: value.scope, window: [0] }, epoch);
      await storage.put(value, epoch);
    }
    expect(await storage.get(entry('a').scope, 0, epoch)).toBeUndefined();
    expect(await storage.put(entry('a'), epoch)).toBe(false);
    expect(await storage.put(entry('d', 8), epoch)).toBe(false);
    expect(await storage.get(entry('b').scope, 0, epoch)).toBeDefined();
    await storage.retain('reader', { scope: entry('d').scope, window: [8] }, epoch);
    expect(await storage.get(entry('d').scope, 0, epoch)).toBeUndefined();
    await storage.close();
  });

  it('preserves same-reader reloads and retires writes across logout or reset', async () => {
    const storage = new ChapterCacheStorage('lifecycle', new IDBFactory());
    const epoch = await storage.useReader('reader');
    const value = entry();
    await storage.retain('reader', { scope: value.scope, window: [0] }, epoch);
    await storage.put(value, epoch);
    expect(await storage.useReader('reader')).toBe(epoch);
    await storage.useReader(null);
    expect(await storage.capture('reader')).toBeUndefined();
    const newEpoch = await storage.useReader('reader');
    expect(newEpoch).not.toBe(epoch);
    await storage.retain('reader', { scope: value.scope, window: [0] }, newEpoch);
    expect(await storage.put(value, epoch)).toBe(false);
    expect(await storage.get(value.scope, 0, newEpoch)).toBeUndefined();
    await storage.close();
  });
});
