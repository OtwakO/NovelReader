import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ExploreApiError } from '../../api/transport';
import { useExploreStore } from './explore-store';

const api = vi.hoisted(() => ({ listExploreSources: vi.fn(), openExplore: vi.fn(), updateExploreControl: vi.fn(), getExplorePage: vi.fn() }));
vi.mock('../../api/explore', () => api);

const source = { id: 'source-a', name: 'Source A', group: 'Group A', capabilities: ['explore', 'javascript', 'webview'] };
const catalog = { source, sessionId: 'session-a', entries: [{ id: 'category', title: 'Category', type: 'url', selectable: true }], diagnostics: [] };

function result(name: string) { return { name, author: 'Author', coverUrl: '', intro: '', kind: '', lastChapter: '', sourceId: 'source-a', sourceUrl: 'source-a', sourceName: 'Source A', bookUrl: `/${name}`, alternateSources: [] }; }

describe('Explore store', () => {
  beforeEach(() => { setActivePinia(createPinia()); sessionStorage.clear(); Object.values(api).forEach(mock => mock.mockReset()); api.listExploreSources.mockResolvedValue([source]); api.openExplore.mockResolvedValue(catalog); });
  it('keeps sequential server paging and category cache authoritative', async () => {
    api.getExplorePage.mockResolvedValueOnce({ sourceId: 'source-a', sessionId: 'session-a', categoryId: 'category', page: 1, nextPage: 2, books: [result('One')], exhausted: false, diagnostics: [] }).mockResolvedValueOnce({ sourceId: 'source-a', sessionId: 'session-a', categoryId: 'category', page: 2, nextPage: 3, books: [result('Two')], exhausted: true, diagnostics: [] });
    const store = useExploreStore(); await store.loadSources(); await store.openSource('source-a'); store.selectCategory(catalog.entries[0]!); await vi.waitFor(() => expect(store.results).toHaveLength(1)); await store.loadPage(store.nextPage, false);
    expect(api.getExplorePage.mock.calls.map(call => call.slice(1, 3))).toEqual([['category', 1], ['category', 2]]); expect(store.results.map(item => item.name)).toEqual(['One', 'Two']); expect(store.cache.category?.exhausted).toBe(true);
  });
  it('clears a cached selection when the source is disabled', async () => {
    const store = useExploreStore(); store.sourceId = 'source-a'; store.catalog = catalog; store.results = [result('Old')]; api.listExploreSources.mockResolvedValueOnce([]);
    await store.loadSources();
    expect(store.sources).toEqual([]); expect(store.sourceId).toBe(''); expect(store.catalog).toBeNull(); expect(store.results).toEqual([]);
  });
  it('refreshes only the selected source after an external source action', async () => {
    sessionStorage.setItem('novelreader.explore-session.v1', JSON.stringify({ sourceId: 'source-a', catalog, categoryId: 'category', results: [result('Old')], nextPage: 2, exhausted: false, diagnostics: [], cache: {} }));
    const store = useExploreStore();
    store.refreshSource('source-b'); expect(api.openExplore).not.toHaveBeenCalled();
    store.refreshSource('source-a'); await vi.waitFor(() => expect(api.openExplore).toHaveBeenCalledWith('source-a', expect.any(AbortSignal)));
    expect(store.catalog?.sessionId).toBe('session-a'); expect(store.categoryId).toBe(''); expect(store.results).toEqual([]);
  });
  it('reopens an expired opaque session for the selected source', async () => {
    api.getExplorePage.mockRejectedValue(new ExploreApiError({ code: 'session_not_found', stage: 'session', severity: 'error', message: 'expired' }));
    const store = useExploreStore(); store.sourceId = 'source-a'; store.catalog = catalog; store.categoryId = 'category'; await store.loadPage(1, true); await vi.waitFor(() => expect(api.openExplore).toHaveBeenCalledWith('source-a', expect.any(AbortSignal)));
    expect(store.catalog?.sessionId).toBe('session-a'); expect(store.categoryId).toBe('');
  });
  it('retries the backend-provided expected page after a page conflict', async () => {
    api.getExplorePage.mockRejectedValueOnce(new ExploreApiError({ code: 'page_conflict', stage: 'page', severity: 'error', message: 'expected page 3', nextPage: 3 })).mockResolvedValueOnce({ sourceId: 'source-a', sessionId: 'session-a', categoryId: 'category', page: 3, nextPage: 4, books: [result('Three')], exhausted: true, diagnostics: [] });
    const store = useExploreStore(); store.catalog = catalog; store.sourceId = 'source-a'; store.categoryId = 'category'; await store.loadPage(2, false); expect(store.nextPage).toBe(3); expect(store.retryable).toBe(true); store.retry(); await vi.waitFor(() => expect(store.results[0]?.name).toBe('Three')); expect(api.getExplorePage.mock.calls[1]?.[2]).toBe(3);
  });
});
