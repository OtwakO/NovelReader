import { IDBFactory } from 'fake-indexeddb';
import { describe, expect, it } from 'vitest';
import { ChapterCacheStorage } from './chapter-cache-storage';
import { cacheScope, type ChapterCacheIdentity, type SavedChapter } from './chapter-cache-policy';

const catalog = { contentRevision: 1, chapters: [{ index: 0, title: 'Synthetic', isVolume: false }] };
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
    expect(await first.putCatalog('reader', value.scope, catalog, epoch)).toBe(true);
    expect(await other.get(value.scope, 0, epoch)).toEqual(value);
    await other.invalidate({ bookId: 'book', index: 0 });
    expect(await first.put(value, epoch)).toBe(false);
    const next = await other.capture('reader');
    expect(await other.get(value.scope, 0, next!)).toBeUndefined();
    expect(await other.getCatalog(value.scope, next!)).toEqual(catalog);
    await other.invalidate({ bookId: 'book' });
    expect(await first.putCatalog('reader', value.scope, catalog, epoch)).toBe(false);
    expect(await other.getCatalog(value.scope, (await other.capture('reader'))!)).toBeUndefined();
    await first.close(); await other.close();
  });

  it('retains three foreground books and their windows without speculative resurrection', async () => {
    const storage = new ChapterCacheStorage('retention', new IDBFactory());
    const epoch = await storage.useReader('reader');
    for (const bookId of ['a', 'b', 'c', 'd']) {
      const value = entry(bookId);
      await storage.retain('reader', { scope: value.scope, window: [0] }, epoch);
      await storage.put(value, epoch);
      await storage.putCatalog('reader', value.scope, catalog, epoch);
    }
    expect(await storage.get(entry('a').scope, 0, epoch)).toBeUndefined();
    expect(await storage.getCatalog(entry('a').scope, epoch)).toBeUndefined();
    expect(await storage.putCatalog('reader', entry('a').scope, catalog, epoch)).toBe(false);
    expect(await storage.put(entry('a'), epoch)).toBe(false);
    expect(await storage.put(entry('d', 8), epoch)).toBe(false);
    expect(await storage.get(entry('b').scope, 0, epoch)).toBeDefined();
    await storage.retain('reader', { scope: entry('d').scope, window: [8] }, epoch);
    expect(await storage.get(entry('d').scope, 0, epoch)).toBeUndefined();
    expect(await storage.getCatalog(entry('d').scope, epoch)).toEqual(catalog);
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

it('upgrades version 1 without losing chapters and leaves old clients a safe version error', async () => {
  const factory = new IDBFactory();
  const value = entry();
  const old = await new Promise<IDBDatabase>((resolve, reject) => {
    const opening = factory.open('upgrade', 1);
    opening.onupgradeneeded = () => {
      const db = opening.result;
      db.createObjectStore('control').put({ epoch: 1, readerId: 'reader', books: [{ scope: value.scope, window: [0] }] }, 'state');
      db.createObjectStore('chapters', { keyPath: 'id' }).createIndex('position', ['scope', 'index']);
      opening.transaction!.objectStore('chapters').put(value);
    };
    opening.onsuccess = () => resolve(opening.result);
    opening.onerror = () => reject(opening.error);
  });
  old.onversionchange = () => old.close();
  const storage = new ChapterCacheStorage('upgrade', factory);
  expect(await storage.get(value.scope, 0, 1)).toEqual(value);
  expect(await storage.putCatalog('reader', value.scope, catalog, 1)).toBe(true);
  await expect(new Promise((resolve, reject) => {
    const opening = factory.open('upgrade', 1);
    opening.onsuccess = () => { opening.result.close(); resolve(undefined); };
    opening.onerror = () => reject(opening.error);
  })).rejects.toMatchObject({ name: 'VersionError' });
  await storage.close();
});
