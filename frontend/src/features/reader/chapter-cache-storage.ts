import type { ChapterCatalog } from '../../api/chapter-catalog';
import { retainedBooks, type RetainedBook, type SavedChapter } from './chapter-cache-policy';

interface Control { epoch: number; readerId: string | null; books: RetainedBook[] }
export interface CacheInvalidation { bookId?: string; index?: number; provider?: 'booksource' | 'txt' | 'epub' }
const emptyControl = (): Control => ({ epoch: 0, readerId: null, books: [] });
const request = <T>(value: IDBRequest<T>) => new Promise<T>((resolve, reject) => {
  value.onsuccess = () => resolve(value.result);
  value.onerror = () => reject(value.error);
});

// One disposable database. All writes include control in their transaction, so
// an invalidation in another connection cannot be undone by a late response.
export class ChapterCacheStorage {
  private opening?: Promise<IDBDatabase>;
  constructor(private readonly name = 'novelreader.chapters.v1', private readonly factory?: IDBFactory) {}

  private open(): Promise<IDBDatabase> {
    return this.opening ??= new Promise((resolve, reject) => {
      const opening = (this.factory ?? indexedDB).open(this.name, 2);
      let expired = false;
      const fail = (cause: unknown) => { expired = true; clearTimeout(timer); reject(cause); };
      const timer = setTimeout(() => fail(new Error('Chapter cache open timed out')), 2000);
      opening.onblocked = () => fail(new Error('Chapter cache upgrade blocked'));
      opening.onerror = () => fail(opening.error);
      opening.onupgradeneeded = () => {
        const db = opening.result;
        if (!db.objectStoreNames.contains('control')) {
          db.createObjectStore('control');
          db.createObjectStore('chapters', { keyPath: 'id' }).createIndex('position', ['scope', 'index']);
        }
        db.createObjectStore('catalogs');
      };
      opening.onsuccess = () => {
        clearTimeout(timer);
        const db = opening.result;
        if (expired) { db.close(); return; }
        db.onversionchange = () => { db.close(); this.opening = undefined; };
        resolve(db);
      };
    });
  }

  // Work awaits only requests in this transaction, never network or other I/O.
  private async transaction<T>(mode: IDBTransactionMode, work: (tx: IDBTransaction, control: Control) => Promise<T>): Promise<T> {
    const db = await this.open();
    return new Promise<T>((resolve, reject) => {
      const tx = db.transaction(['control', 'chapters', 'catalogs'], mode);
      let result: T;
      let failure: unknown;
      const timer = setTimeout(() => {
        failure = new Error('Chapter cache transaction timed out');
        try { tx.abort(); } catch { /* already completed; do not wait for a delayed event */ }
        reject(failure);
      }, 2000);
      tx.oncomplete = () => { clearTimeout(timer); resolve(result); };
      tx.onabort = () => { clearTimeout(timer); reject(failure ?? tx.error); };
      tx.onerror = () => { /* onabort supplies the transaction failure */ };
      void request<Control | undefined>(tx.objectStore('control').get('state')).then(control => work(tx, control ?? emptyControl())).then(value => { result = value; }, cause => {
        failure = cause;
        try { tx.abort(); } catch { clearTimeout(timer); reject(cause); } // A timed-out transaction may already be finished.
      });
    });
  }

  private saveControl(tx: IDBTransaction, control: Control) { tx.objectStore('control').put(control, 'state'); }

  // Cursor keys avoid copying every chapter payload just to prune metadata.
  private async prune(tx: IDBTransaction, keep: (scope: string, index: number) => boolean, keepCatalog: (scope: string) => boolean): Promise<void> {
    await new Promise<void>((resolve, reject) => {
      const cursor = tx.objectStore('chapters').index('position').openKeyCursor();
      cursor.onerror = () => reject(cursor.error);
      cursor.onsuccess = () => {
        const row = cursor.result;
        if (!row) { resolve(); return; }
        const [scope, index] = row.key as [string, number];
        if (!keep(scope, index)) tx.objectStore('chapters').delete(row.primaryKey);
        row.continue();
      };
    });
    const catalogs = tx.objectStore('catalogs');
    for (const scope of await request(catalogs.getAllKeys())) if (!keepCatalog(String(scope))) catalogs.delete(scope);
  }

  async useReader(readerId: string | null, reset = false): Promise<number> {
    return this.transaction('readwrite', async (tx, control) => {
      if (reset || control.readerId !== readerId) {
        control.readerId = readerId; control.epoch++; control.books = [];
        tx.objectStore('chapters').clear(); tx.objectStore('catalogs').clear(); this.saveControl(tx, control);
      }
      return control.epoch;
    });
  }

  qualify(readerId: string, scope: string, epoch: number): Promise<{ epoch: number; changed: boolean; homeChanged: boolean } | undefined> {
    return this.transaction('readwrite', async (tx, control) => {
      if (control.readerId !== readerId || control.epoch !== epoch) return;
      const identity = JSON.parse(scope) as unknown[];
      const superseded = (candidate: string) => {
        const previous = JSON.parse(candidate) as unknown[];
        return previous[1] !== identity[1] || (previous[2] === identity[2] && candidate !== scope);
      };
      const homeChanged = control.books.some(book => (JSON.parse(book.scope) as unknown[])[1] !== identity[1]);
      const changed = control.books.some(book => superseded(book.scope));
      if (changed) {
        control.epoch++;
        control.books = control.books.filter(book => !superseded(book.scope));
        this.saveControl(tx, control);
        await this.prune(tx, candidate => !superseded(candidate), candidate => !superseded(candidate));
      }
      return { epoch: control.epoch, changed, homeChanged };
    });
  }

  capture(readerId: string): Promise<number | undefined> {
    return this.transaction('readonly', async (_tx, control) => control.readerId === readerId ? control.epoch : undefined);
  }

  get(scope: string, index: number, epoch: number): Promise<SavedChapter | undefined> {
    return this.transaction('readonly', async (tx, control) => {
      if (epoch !== control.epoch) return;
      return request<SavedChapter | undefined>(tx.objectStore('chapters').get(JSON.stringify([scope, index])));
    });
  }

  put(entry: SavedChapter, epoch: number): Promise<boolean> {
    return this.transaction('readwrite', async (tx, control) => {
      if (control.readerId !== entry.identity.readerId || control.epoch !== epoch || !control.books.some(book => book.scope === entry.scope && book.window.includes(entry.index))) return false;
      tx.objectStore('chapters').put(entry);
      return true;
    });
  }

  getCatalog(scope: string, epoch: number): Promise<ChapterCatalog | undefined> {
    return this.transaction('readonly', async (tx, control) => {
      if (control.epoch !== epoch) return;
      return request<ChapterCatalog | undefined>(tx.objectStore('catalogs').get(scope));
    });
  }

  putCatalog(readerId: string, scope: string, catalog: ChapterCatalog, epoch: number): Promise<boolean> {
    return this.transaction('readwrite', async (tx, control) => {
      if (control.readerId !== readerId || control.epoch !== epoch || !control.books.some(book => book.scope === scope)) return false;
      tx.objectStore('catalogs').put(catalog, scope);
      return true;
    });
  }

  // Only a committed foreground visit changes recency or the retained window.
  retain(readerId: string, book: RetainedBook, epoch: number, catalog?: ChapterCatalog): Promise<boolean> {
    return this.transaction('readwrite', async (tx, control) => {
      if (control.readerId !== readerId || control.epoch !== epoch) return false;
      control.books = [book, ...control.books.filter(value => value.scope !== book.scope)].slice(0, retainedBooks);
      this.saveControl(tx, control);
      await this.prune(tx, (scope, index) => control.books.some(value => value.scope === scope && value.window.includes(index)), scope => control.books.some(value => value.scope === scope));
      // Moving the chapter window does not change a scope's canonical catalog.
      // Check disk rather than a memory flag: another tab may have evicted it.
      const catalogs = tx.objectStore('catalogs');
      if (catalog && await request(catalogs.getKey(book.scope)) === undefined) catalogs.put(catalog, book.scope);
      return true;
    });
  }

  invalidate(filter: CacheInvalidation = {}): Promise<number> {
    return this.transaction('readwrite', async (tx, control) => {
      control.epoch++;
      const affected = (scope: string) => {
        const identity = JSON.parse(scope) as unknown[];
        return (!filter.bookId || identity[2] === filter.bookId) && (!filter.provider || identity[4] === filter.provider);
      };
      if (filter.index === undefined) control.books = control.books.filter(book => !affected(book.scope));
      this.saveControl(tx, control);
      await this.prune(tx, (scope, index) => !affected(scope) || (filter.index !== undefined && index !== filter.index), scope => !affected(scope) || filter.index !== undefined);
      return control.epoch;
    });
  }

  evictInactive(scope: string): Promise<void> {
    return this.transaction('readwrite', async (tx, control) => {
      control.books = control.books.filter(book => book.scope === scope);
      this.saveControl(tx, control);
      await this.prune(tx, value => value === scope, value => value === scope);
    });
  }

  async close(): Promise<void> { (await this.opening)?.close(); this.opening = undefined; }
}
