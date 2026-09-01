import { mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as searchApi from '../../api/search';
import SourceRecoveryPanel from './SourceRecoveryPanel.vue';

vi.mock('../../api/search', async () => { const actual = await vi.importActual<typeof import('../../api/search')>('../../api/search'); return { ...actual, searchBooksBatchStream: vi.fn(() => ({ close: vi.fn() })), searchInstalledSource: vi.fn(async () => []) }; });
beforeEach(() => vi.clearAllMocks());
const i18n = createI18n({ legacy: false, globalInjection: true, locale: 'en', messages: { en: { sources: { capabilities: { search: 'Search', explore: 'Explore', headers: 'Headers', javascript: 'JavaScript', webview: 'WebView' } }, sourceRecovery: { eyebrow: 'Recovery', title: 'Sources', description: 'Description', stop: 'Stop', rescan: 'Clear', clearing: 'Clearing', confirmTitle: 'Confirm', confirmDescription: 'Description', cancel: 'Cancel', confirm: 'Confirm', switching: 'Switching', use: 'Use', empty: 'Empty', find: 'Find', filterLabel: 'Filter', filterPlaceholder: 'Search sources', filterKinds: 'Kinds', all: 'All', stored: 'Stored', new: 'New', noFilterMatches: 'No matches', targetedTitle: 'Search this source', targetedDescription: 'Opaque query', targetedLabel: 'Query', targetedPlaceholder: 'Custom query', targetedAction: 'Run query', targetedSaved: 'Saved {count}', targetedEmpty: 'No new matches', targetedFailed: 'Search failed', discoveredByQuery: 'Found with query: {query}' }, search: { controls: { title: 'Controls', batchSize: 'Batch', intensity: 'Intensity', gentle: 'Gentle', balanced: 'Balanced', fast: 'Fast', advanced: 'Advanced', concurrency: 'Concurrency' }, status: { checkedOf: '', checked: '', results: '', concurrency: '', failures: '', disconnected: '', stale: '', storage: '' }, actions: { restart: '', retry: '', more: '' } } } } });

describe('SourceRecoveryPanel', () => {
  it('shows persisted alternate sources immediately', () => {
    const wrapper = mount(SourceRecoveryPanel, { global: { plugins: [i18n] }, props: { book: { name: 'Book', author: 'Author' }, currentSourceId: 'current', currentBookUrl: '/current', storedSources: [{ sourceId: 'stored', sourceUrl: 'stored', bookUrl: '/book', sourceName: 'Stored source', sourceGroup: 'Group A', capabilities: ['javascript', 'webview'] }], onClearAndRescan: vi.fn(async () => undefined) } });
    expect(wrapper.text()).toContain('Stored source'); expect(wrapper.text()).toContain('Group A'); expect(wrapper.text()).toContain('JavaScript'); expect(wrapper.text()).toContain('WebView');
  });
  it('keeps alternate bindings from the current installed source', () => {
    const wrapper = mount(SourceRecoveryPanel, { global: { plugins: [i18n] }, props: { book: { name: 'Book', author: 'Author' }, currentSourceId: 'aggregate', currentBookUrl: '/provider-a', storedSources: [{ sourceId: 'aggregate', sourceUrl: 'aggregate', bookUrl: '/provider-a', sourceName: 'Current provider' }, { sourceId: 'aggregate', sourceUrl: 'aggregate', bookUrl: '/provider-b', sourceName: 'Alternate provider' }], onClearAndRescan: vi.fn(async () => undefined) } });
    expect(wrapper.text()).toContain('Alternate provider');
    expect(wrapper.text()).not.toContain('Current provider');
  });
  it('runs an opaque query only against the current source and accepts exact alternate bindings', async () => {
    vi.mocked(searchApi.searchInstalledSource).mockResolvedValue([{ name: 'Book', author: 'Author', coverUrl: '', intro: '', kind: '', lastChapter: '', bookUrl: '/provider-b', sourceId: 'aggregate', sourceUrl: 'aggregate', sourceName: 'Alternate provider' }]);
    const wrapper = mount(SourceRecoveryPanel, { global: { plugins: [i18n] }, props: { book: { name: 'Book', author: 'Author' }, currentSourceId: 'aggregate', currentBookUrl: '/provider-a', storedSources: [], onClearAndRescan: vi.fn(async () => undefined) } });
    await wrapper.get('details summary').trigger('click');
    await wrapper.get('.targeted-search input').setValue('Book@provider');
    await wrapper.get('.targeted-search form').trigger('submit');
    expect(searchApi.searchInstalledSource).toHaveBeenCalledWith('aggregate', 'Book@provider');
    expect(wrapper.text()).toContain('Alternate provider');
    expect(wrapper.text()).toContain('Found with query: Book@provider');
    expect(wrapper.emitted('matches')?.[0]?.[0]).toEqual([expect.objectContaining({ sourceId: 'aggregate', bookUrl: '/provider-b', discoveryQuery: 'Book@provider' })]);
  });
  it('preserves multiple bindings returned by one opaque partial query', async () => {
    vi.mocked(searchApi.searchInstalledSource).mockResolvedValue([
      { name: 'Book', author: 'Author', coverUrl: '', intro: '', kind: '', lastChapter: '', bookUrl: '/69', sourceId: 'aggregate', sourceUrl: 'aggregate', sourceName: 'Aggregate' },
      { name: 'Book', author: 'Author', coverUrl: '', intro: '', kind: '', lastChapter: '', bookUrl: '/fake-69', sourceId: 'aggregate', sourceUrl: 'aggregate', sourceName: 'Aggregate' },
    ]);
    const wrapper = mount(SourceRecoveryPanel, { global: { plugins: [i18n] }, props: { book: { name: 'Book', author: 'Author' }, currentSourceId: 'aggregate', currentBookUrl: '/current', storedSources: [], onClearAndRescan: vi.fn(async () => undefined) } });
    await wrapper.get('.targeted-search input').setValue('Book@69');
    await wrapper.get('.targeted-search form').trigger('submit');
    expect(wrapper.findAll('.sources li')).toHaveLength(2);
    expect(wrapper.emitted('matches')?.[0]?.[0]).toEqual([
      expect.objectContaining({ bookUrl: '/69', discoveryQuery: 'Book@69' }),
      expect.objectContaining({ bookUrl: '/fake-69', discoveryQuery: 'Book@69' }),
    ]);
  });
  it('persists metadata for the current binding without showing it as an alternate', async () => {
    vi.mocked(searchApi.searchInstalledSource).mockResolvedValue([{ name: 'Book', author: 'Author', coverUrl: '', intro: '', kind: '', lastChapter: '', bookUrl: '/provider-a', sourceId: 'aggregate', sourceUrl: 'aggregate', sourceName: 'Aggregate' }]);
    const wrapper = mount(SourceRecoveryPanel, { global: { plugins: [i18n] }, props: { book: { name: 'Book', author: 'Author' }, currentSourceId: 'aggregate', currentBookUrl: '/provider-a', storedSources: [], onClearAndRescan: vi.fn(async () => undefined) } });
    await wrapper.get('.targeted-search input').setValue('Book@current');
    await wrapper.get('.targeted-search form').trigger('submit');
    expect(wrapper.emitted('matches')?.[0]?.[0]).toEqual([expect.objectContaining({ bookUrl: '/provider-a', discoveryQuery: 'Book@current' })]);
    expect(wrapper.findAll('.sources li')).toHaveLength(0);
  });
  it('enriches an already stored binding with its targeted discovery query', async () => {
    vi.mocked(searchApi.searchInstalledSource).mockResolvedValue([{ name: 'Book', author: 'Author', coverUrl: '', intro: '', kind: '', lastChapter: '', bookUrl: '/provider-b', sourceId: 'aggregate', sourceUrl: 'aggregate', sourceName: 'Aggregate' }]);
    const wrapper = mount(SourceRecoveryPanel, { global: { plugins: [i18n] }, props: { book: { name: 'Book', author: 'Author' }, currentSourceId: 'aggregate', currentBookUrl: '/provider-a', storedSources: [{ sourceId: 'aggregate', sourceUrl: 'aggregate', bookUrl: '/provider-b', sourceName: 'Aggregate' }], onClearAndRescan: vi.fn(async () => undefined) } });
    await wrapper.get('.targeted-search input').setValue('Book@provider');
    await wrapper.get('.targeted-search form').trigger('submit');
    expect(wrapper.text()).toContain('Found with query: Book@provider');
    expect(wrapper.emitted('matches')?.[0]?.[0]).toEqual([expect.objectContaining({ bookUrl: '/provider-b', discoveryQuery: 'Book@provider' })]);
  });
  it('filters known sources locally without changing discovery state', async () => {
    const wrapper = mount(SourceRecoveryPanel, { global: { plugins: [i18n] }, props: { book: { name: 'Book', author: 'Author' }, currentSourceId: 'current', currentBookUrl: '/current', storedSources: [{ sourceId: 'alpha', sourceUrl: 'alpha', bookUrl: '/a', sourceName: 'Alpha' }, { sourceId: 'beta', sourceUrl: 'beta', bookUrl: '/b', sourceName: 'Beta' }], onClearAndRescan: vi.fn(async () => undefined) } });
    await wrapper.get('.source-filter input[type="search"]').setValue('Beta');
    expect(wrapper.text()).toContain('Beta'); expect(wrapper.text()).not.toContain('Alpha'); expect(wrapper.vm.state.checked).toBe(0);
  });
  it('keeps an active scan when the same logical book receives refreshed props', async () => {
    let handlers: searchApi.SearchBatchHandlers | undefined;
    vi.mocked(searchApi.searchBooksBatchStream).mockImplementation((_query, _options, value) => { handlers = value; return { close: vi.fn() } as unknown as EventSource; });
    const wrapper = mount(SourceRecoveryPanel, { global: { plugins: [i18n] }, props: { book: { name: 'Book', author: 'Author' }, currentSourceId: 'current', currentBookUrl: '/current', storedSources: [], onClearAndRescan: vi.fn(async () => undefined) } });
    await wrapper.findAll('button').find((button) => button.text() === 'Find')?.trigger('click');
    handlers?.onStart({ offset: 0, eligible: 150, sourcesInBatch: 50, requestedConcurrency: 7, retryCursor: 'retry-0', effectiveConcurrency: 7 });
    handlers?.onDone({ complete: true, checked: 50, eligible: 150, hasMore: true, nextCursor: 'next-50', retryCursor: 'retry-0', sourceFailures: 0 });
    await wrapper.setProps({ book: { name: 'Book', author: 'Author' }, currentSourceId: 'found', currentBookUrl: '/book', storedSources: [{ sourceId: 'found', sourceUrl: 'found', bookUrl: '/book', sourceName: 'Found' }, { sourceId: 'current', sourceUrl: 'current', bookUrl: '/old', sourceName: 'Previous' }] });
    expect(wrapper.vm.state.checked).toBe(50); expect(wrapper.vm.state.hasMore).toBe(true); expect(wrapper.text()).toContain('Previous');
  });
  it('requires an explicit confirmation before clearing stored sources', async () => {
    const wrapper = mount(SourceRecoveryPanel, { global: { plugins: [i18n] }, props: { book: { name: 'Book', author: 'Author' }, currentSourceId: 'current', currentBookUrl: '/current', storedSources: [], onClearAndRescan: vi.fn(async () => undefined) } });
    await wrapper.findAll('button').find((button) => button.text() === 'Clear')?.trigger('click');
    expect(wrapper.text()).toContain('Confirm'); expect(wrapper.props('onClearAndRescan')).not.toHaveBeenCalled();
  });
});
