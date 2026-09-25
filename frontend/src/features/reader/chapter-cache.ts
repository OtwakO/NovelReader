import type { Chapter } from '../../api/models';
import { parseChapterContent, type ReadingContent } from '../../api/reader';
import { cacheScope, chapterWindow, retainedBooks, retainedChapters, savedChapter, savedChapterIsFresh, type ChapterCacheIdentity, type SavedChapter } from './chapter-cache-policy';
import { ChapterCacheStorage, type CacheInvalidation } from './chapter-cache-storage';

export class ChapterCacheInvalidated extends Error {
  constructor() { super('Reading cache identity changed; reopen the book'); }
}
export interface CacheValidation { readerId?: string; epoch?: number; local: number }
interface Ticket { version: number; epoch?: number }
interface Receipt extends Ticket { content: ReadingContent; entry?: SavedChapter; monotonic: number }
interface MemoryBook { identity: ChapterCacheIdentity; scope: string; version: number; entries: Map<number, Receipt>; window: number[]; retained: boolean; epoch?: number }

/** Owns memory, persistent eligibility and invalidation; it never starts HTTP work. */
export class ChapterCache {
  private readerId?: string;
  private local = 0;
  private books = new Map<string, MemoryBook>();
  private pending: Promise<unknown> = Promise.resolve();
  private disabled = false;
  private channel?: BroadcastChannel;
  constructor(private readonly storage = new ChapterCacheStorage()) {}

  connect() {
    if (this.channel || typeof window.BroadcastChannel !== 'function') return;
    this.channel = new window.BroadcastChannel('novelreader.chapter-cache');
    this.channel.onmessage = event => {
      const value = event.data;
      if (!value || typeof value !== 'object' || (value.bookId !== undefined && typeof value.bookId !== 'string') || (value.index !== undefined && !Number.isSafeInteger(value.index))) return;
      this.clearMemory(value as CacheInvalidation);
      if (Number.isSafeInteger(value.epoch)) this.advanceRetainedEpoch(value.epoch);
    };
  }

  private clearMemory(filter: CacheInvalidation = {}) {
    this.local++;
    for (const book of this.books.values()) {
      if (filter.bookId && filter.bookId !== book.identity.bookId) continue;
      if (filter.provider && filter.provider !== book.identity.provider) continue;
      book.version++;
      if (filter.index === undefined) {
        book.entries.clear(); book.retained = false; this.books.delete(book.scope);
      }
      else book.entries.delete(filter.index);
    }
  }

  // Only completed, retained documents survive a known invalidation transition.
  // Never renew the tickets held by requests or writes already in flight.
  private advanceRetainedEpoch(epoch: number) {
    for (const book of this.books.values()) {
      if (book.epoch === epoch - 1) book.epoch = epoch;
      for (const [index, receipt] of book.entries) {
        if (receipt.epoch === epoch - 1) book.entries.set(index, { ...receipt, epoch });
        else if (receipt.epoch !== undefined && receipt.epoch < epoch) book.entries.delete(index);
      }
    }
  }

  private async disk<T>(work: () => Promise<T>): Promise<T | undefined> {
    if (this.disabled) return;
    try { return await work(); }
    catch (cause) {
      this.disabled = true;
      console.warn('Persistent chapter cache unavailable; continuing with memory/network', cause);
    }
  }

  useReader(readerId?: string, reset = false): Promise<unknown> {
    this.connect();
    const changed = this.readerId !== readerId;
    this.readerId = readerId;
    if (changed || reset) { this.clearMemory(); this.books.clear(); }
    this.pending = this.pending.then(() => this.disk(() => this.storage.useReader(readerId ?? null, reset)));
    if (reset) this.channel?.postMessage({});
    return this.pending;
  }

  invalidate(filter: CacheInvalidation = {}): Promise<unknown> {
    this.clearMemory(filter);
    this.pending = this.pending.then(async () => {
      const epoch = await this.disk(() => this.storage.invalidate(filter));
      if (epoch !== undefined) this.advanceRetainedEpoch(epoch);
      this.channel?.postMessage({ ...filter, epoch });
    });
    return this.pending;
  }

  async beginValidation(): Promise<CacheValidation> {
    await this.pending;
    const readerId = this.readerId;
    const local = this.local;
    const epoch = readerId ? await this.disk(() => this.storage.capture(readerId)) : undefined;
    if (local !== this.local || readerId !== this.readerId) throw new ChapterCacheInvalidated();
    return { readerId, epoch, local };
  }

  async bind(identity: Omit<ChapterCacheIdentity, 'readerId'>, chapters: readonly Chapter[], validation: CacheValidation, resume: number) {
    if (validation.local !== this.local || validation.readerId !== this.readerId) throw new ChapterCacheInvalidated();
    if (!validation.readerId || !identity.homeGeneration || (identity.provider === 'booksource' && !identity.sourceIdentity)) return this.ephemeral(identity.bookId, identity.revision);
    const owner = { ...identity, readerId: validation.readerId };
    const scope = cacheScope(owner);
    let epoch = validation.epoch;
    if (epoch !== undefined) {
      const qualification = await this.disk(() => this.storage.qualify(owner.readerId, scope, epoch!));
      if (validation.local !== this.local || (!this.disabled && !qualification)) throw new ChapterCacheInvalidated();
      if (qualification?.changed) {
        const filter = qualification.homeChanged ? {} : { bookId: identity.bookId };
        this.clearMemory(filter);
        this.advanceRetainedEpoch(qualification.epoch);
        this.channel?.postMessage({ ...filter, epoch: qualification.epoch });
      }
      epoch = qualification?.epoch;
    }
    if (validation.readerId !== this.readerId) throw new ChapterCacheInvalidated();
    let book = this.books.get(scope);
    if (!book) {
      book = { identity: owner, scope, version: 0, entries: new Map(), window: chapterWindow(chapters, resume), retained: false, epoch };
      this.books.set(scope, book);
    } else if (book.epoch !== epoch) {
      book.version++; book.entries.clear(); book.epoch = epoch;
    }
    return this.session(book, chapters, Boolean(owner.readerId && owner.homeGeneration), false);
  }

  // Registry-less/legacy callers remain reader-instance-local; never persist an
  // identity that has not passed the online snapshot boundary.
  ephemeral(bookId: string, revision: number) {
    const identity: ChapterCacheIdentity = { readerId: '', homeGeneration: '', bookId, revision, provider: 'txt' };
    return this.session({ identity, scope: cacheScope(identity), version: 0, entries: new Map(), window: [], retained: true }, [], false, true);
  }

  private session(book: MemoryBook, chapters: readonly Chapter[], persistent: boolean, ephemeral: boolean) {
    const receipts = new WeakMap<ReadingContent, Receipt>();
    let retired = false;
    let writes: Promise<unknown> = Promise.resolve();
    const current = (ticket: Ticket) => {
      if (retired) throw new DOMException('Reader session closed', 'AbortError');
      if (ticket.version !== book.version) throw new ChapterCacheInvalidated();
    };
    const remember = (receipt: Receipt) => { receipts.set(receipt.content, receipt); return receipt.content; };
    const persist = (receipt: Receipt) => {
      if (!persistent || !receipt.entry || receipt.epoch === undefined) return;
      const entry = receipt.entry;
      writes = writes.then(async () => {
        if (receipt.version !== book.version) return;
        await this.disk(async () => {
          try { await this.storage.put(entry, receipt.epoch!); }
          catch (cause) {
            if (!(cause instanceof DOMException) || cause.name !== 'QuotaExceededError') throw cause;
            await this.storage.evictInactive(book.scope);
            await this.storage.put(entry, receipt.epoch!);
          }
        });
      });
    };
    const admit = (index: number, receipt: Receipt) => {
      if (!receipt.entry || !savedChapterIsFresh(receipt.entry) || performance.now() >= receipt.monotonic) return;
      if (!ephemeral && (!book.retained || !book.window.includes(index))) return;
      book.entries.delete(index); book.entries.set(index, receipt);
      if (book.entries.size > retainedChapters) book.entries.delete(book.entries.keys().next().value!);
      persist(receipt);
    };
    const peek = (index: number) => {
      const cached = book.entries.get(index);
      if (retired || !cached?.entry || !savedChapterIsFresh(cached.entry) || performance.now() >= cached.monotonic) return;
      book.entries.delete(index); book.entries.set(index, cached);
      return remember({ ...cached, version: book.version });
    };
    return {
      peek,
      remaining(index: number): number | undefined {
        const receipt = book.entries.get(index);
        if (retired || !receipt?.entry) return;
        if (!savedChapterIsFresh(receipt.entry)) return 0;
        return Math.max(0, Math.min(receipt.monotonic - performance.now(), receipt.entry.expiresAt === null ? Infinity : receipt.entry.expiresAt - Date.now()));
      },
      lookup: async (index: number, bypass = false): Promise<{ content?: ReadingContent; ticket: Ticket }> => {
        const ticket: Ticket = { version: book.version };
        current(ticket);
        if (!bypass) {
          const cached = book.entries.get(index);
          if (cached?.entry && savedChapterIsFresh(cached.entry) && performance.now() < cached.monotonic) {
            const receipt = { ...cached, version: book.version };
            return { content: remember(receipt), ticket: receipt };
          }
          book.entries.delete(index);
        }
        await this.pending;
        if (persistent) ticket.epoch = await this.disk(() => this.storage.capture(book.identity.readerId));
        current(ticket);
        if (!bypass && ticket.epoch !== undefined) {
          const entry = await this.disk(() => this.storage.get(book.scope, index, ticket.epoch!));
          current(ticket);
          if (entry && entry.scope === book.scope && entry.index === index && savedChapterIsFresh(entry)) {
            try {
              const content = parseChapterContent(entry.content as unknown as Record<string, unknown>);
              if (content.contentRevision === book.identity.revision && (book.identity.provider !== 'booksource' || (content.version === 1 && content.sourceIdentity === book.identity.sourceIdentity && entry.expiresAt !== null))) {
                const receipt = { ...ticket, content, entry: { ...entry, content }, monotonic: entry.expiresAt === null ? Infinity : performance.now() + entry.expiresAt - Date.now() };
                admit(index, receipt);
                return { content: remember(receipt), ticket };
              }
            } catch { /* A malformed disposable entry is a miss, never reader content. */ }
          }
        }
        return { ticket };
      },
      accept(index: number, content: ReadingContent, ticket: Ticket, started: { wall: number; monotonic: number }) {
        current(ticket);
        if (content.version === 1 && content.sourceIdentity && book.identity.sourceIdentity && content.sourceIdentity !== book.identity.sourceIdentity) throw new ChapterCacheInvalidated();
        let entry = savedChapter(book.identity, index, content, started);
        if (ephemeral && entry && content.version === 1 && content.freshForMs !== undefined) entry.expiresAt = started.wall + content.freshForMs;
        const receipt = { ...ticket, content, entry, monotonic: entry?.expiresAt === null ? Infinity : started.monotonic + (content.version === 1 ? content.freshForMs ?? Infinity : Infinity) };
        remember(receipt); admit(index, receipt);
      },
      assertCurrent(content: ReadingContent) { const receipt = receipts.get(content); if (!receipt) throw new ChapterCacheInvalidated(); current(receipt); },
      commit: (index: number, content: ReadingContent, main = true) => {
        const receipt = receipts.get(content); if (!receipt) throw new ChapterCacheInvalidated(); current(receipt);
        if (!ephemeral) {
          if (main) book.window = chapterWindow(chapters, index);
          book.retained = true;
          this.books.delete(book.scope); this.books.set(book.scope, book);
          const retained = [...this.books.values()].filter(value => value.retained);
          for (const old of retained.slice(0, Math.max(0, retained.length - retainedBooks))) { old.retained = false; old.entries.clear(); this.books.delete(old.scope); }
          for (const key of book.entries.keys()) if (!book.window.includes(key)) book.entries.delete(key);
          if (persistent && receipt.epoch !== undefined) writes = writes.then(() => this.disk(() => this.storage.retain(book.identity.readerId, { scope: book.scope, window: [...book.window] }, receipt.epoch!)));
        }
        admit(index, receipt);
      },
      refresh: async (index: number) => {
        if (ephemeral) { book.version++; book.entries.delete(index); return; }
        await this.invalidate({ bookId: book.identity.bookId, index });
      },
      dispose() { retired = true; if (ephemeral) book.entries.clear(); },
      settled: () => writes,
    };
  }
}

export const chapterCache = new ChapterCache();
export type ChapterCacheSession = ReturnType<ChapterCache['ephemeral']>;
