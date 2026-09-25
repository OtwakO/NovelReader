import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useSearchStore } from './search-store';
import * as searchApi from '../../api/search';

vi.mock('../../api/search', async () => {
  const actual = await vi.importActual<typeof import('../../api/search')>('../../api/search');
  return { ...actual, searchBooksBatchStream: vi.fn() };
});

const values = new Map<string, string>();
const storage = { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => values.set(key, value), removeItem: (key: string) => values.delete(key), clear: () => values.clear(), key: (index: number) => [...values.keys()][index] ?? null, get length() { return values.size; } };

beforeEach(() => { setActivePinia(createPinia()); values.clear(); vi.stubGlobal('localStorage', storage); vi.stubGlobal('sessionStorage', storage); vi.clearAllMocks(); });

describe('search store', () => {
  it('continues searching in memory when browser storage is unavailable', () => {
    const blocked = () => { throw new DOMException('Storage blocked', 'SecurityError'); };
    vi.stubGlobal('sessionStorage', { getItem: blocked, setItem: blocked, removeItem: blocked });
    vi.stubGlobal('localStorage', { getItem: blocked, setItem: blocked });
    let handlers: searchApi.SearchBatchHandlers | undefined;
    vi.mocked(searchApi.searchBooksBatchStream).mockImplementation((_query, _options, value) => {
      handlers = value;
      return { close: vi.fn() } as unknown as EventSource;
    });
    const store = useSearchStore();
    expect(() => store.initialize()).not.toThrow();
    store.query = 'Synthetic';
    expect(() => store.persistPreferences()).not.toThrow();
    expect(() => store.search()).not.toThrow();
    handlers?.onResult('a', [{ name: 'Synthetic', author: '', coverUrl: '', intro: '', kind: '', lastChapter: '', bookUrl: '/a', sourceId: 'a', sourceUrl: 'a', sourceName: 'A' }], 1);
    expect(store.results).toHaveLength(1);
    expect(store.storageWarning).toBe(true);
    expect(() => store.resetReaderState()).not.toThrow();
    expect(store.results).toHaveLength(0);
  });

  it('discards corrupt saved state and starts a new search', () => {
    values.set('novelreader.search-session', '{invalid');
    vi.mocked(searchApi.searchBooksBatchStream).mockReturnValue({ close: vi.fn() } as unknown as EventSource);
    const store = useSearchStore();
    expect(() => store.initialize()).not.toThrow();
    expect(values.has('novelreader.search-session')).toBe(false);
    store.query = 'Synthetic';
    store.search();
    expect(searchApi.searchBooksBatchStream).toHaveBeenCalledOnce();
    expect(store.storageWarning).toBe(false);
  });

  it('ignores late events from a stopped stream', () => {
    let handlers: searchApi.SearchBatchHandlers | undefined;
    const close = vi.fn();
    vi.mocked(searchApi.searchBooksBatchStream).mockImplementation((_query, _options, value) => { handlers = value; return { close } as unknown as EventSource; });
    const store = useSearchStore(); store.initialize(); store.query = '凡人'; store.search(); store.stop();
    handlers?.onResult('source-a', [{ name: '凡人修仙传', author: '忘语', coverUrl: '', intro: '', kind: '', lastChapter: '', bookUrl: '/a', sourceId: 'a', sourceUrl: 'a', sourceName: 'A' }], 1);
    expect(store.results).toHaveLength(0); expect(close).toHaveBeenCalled(); expect(store.retryRequired).toBe(true);
  });

  it('preserves results and retry cursor after disconnect', () => {
    let handlers: searchApi.SearchBatchHandlers | undefined;
    vi.mocked(searchApi.searchBooksBatchStream).mockImplementation((_query, _options, value) => { handlers = value; return { close: vi.fn() } as unknown as EventSource; });
    const store = useSearchStore(); store.initialize(); store.query = '凡人'; store.search();
    handlers?.onStart({ offset: 0, eligible: 10, sourcesInBatch: 5, requestedConcurrency: 8, effectiveConcurrency: 5, retryCursor: 'retry-0' });
    handlers?.onResult('a', [{ name: '凡人修仙传', author: '忘语', coverUrl: '', intro: '', kind: '', lastChapter: '', bookUrl: '/a', sourceId: 'a', sourceUrl: 'a', sourceName: 'A' }], 1);
    handlers?.onDisconnect();
    expect(store.results).toHaveLength(1); expect(store.retryRequired).toBe(true); expect(store.retryCursor).toBe('retry-0'); expect(store.errorCode).toBe('disconnect');
  });
});
