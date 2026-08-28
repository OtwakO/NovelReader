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
