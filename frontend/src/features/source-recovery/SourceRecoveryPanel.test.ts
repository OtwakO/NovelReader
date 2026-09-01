import { mount, type VueWrapper } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AltSource, Book } from '../../api/models';
import * as searchApi from '../../api/search';
import SourceRecoveryPanel from './SourceRecoveryPanel.vue';

vi.mock('../../api/search', async () => {
  const actual = await vi.importActual<typeof import('../../api/search')>('../../api/search');
  return { ...actual, searchBooksBatchStream: vi.fn(() => ({ close: vi.fn() })), searchInstalledSource: vi.fn(async () => []) };
});

beforeEach(() => vi.clearAllMocks());

const i18n = createI18n({ legacy: false, globalInjection: true, locale: 'en', messages: { en: { sources: { capabilities: { search: 'Search', explore: 'Explore', headers: 'Headers', javascript: 'JavaScript', webview: 'WebView' } }, sourceRecovery: { eyebrow: 'Recovery', title: 'Sources', description: 'Description', stop: 'Stop', rescan: 'Clear', clearing: 'Clearing', confirmTitle: 'Confirm', confirmDescription: 'Description', cancel: 'Cancel', confirm: 'Confirm', switching: 'Switching', use: 'Use', currentSource: 'Current source', empty: 'Empty', find: 'Find', filterLabel: 'Filter', filterPlaceholder: 'Search sources', filterKinds: 'Kinds', all: 'All', stored: 'Stored', new: 'New', noFilterMatches: 'No matches', targetedTitle: 'Search this source', targetedDescription: 'Opaque query', targetedLabel: 'Query', targetedPlaceholder: 'Custom query', targetedAction: 'Run query', targetedSaved: 'Saved {count}', targetedEmpty: 'No new matches', targetedFailed: 'Search failed', discoveredByQuery: 'Found with query: {query}' }, search: { controls: { title: 'Controls', batchSize: 'Batch', intensity: 'Intensity', gentle: 'Gentle', balanced: 'Balanced', fast: 'Fast', advanced: 'Advanced', concurrency: 'Concurrency' }, status: { checkedOf: '', checked: '', results: '', concurrency: '', failures: '', disconnected: '', stale: '', storage: '' }, actions: { restart: '', retry: '', more: '' } } } } });

const active: AltSource = { sourceId: 'aggregate', sourceUrl: 'aggregate', bookUrl: '/current', sourceName: 'Aggregate', lastChapter: 'Initial provider hint' };

function shelfBook(activeSource = active, alternateSources: AltSource[] = []): Book {
  return { id: 'book', name: 'Book', author: 'Author', coverUrl: '', intro: '', kind: '', sourceId: activeSource.sourceId, sourceUrl: activeSource.sourceUrl, bookUrl: activeSource.bookUrl, origin: activeSource.sourceName, lastChapter: activeSource.lastChapter || '', durChapterIndex: 0, durChapterPos: 0, totalChapterNum: 0, stateVersion: 1, activeSource, alternateSources };
}

function mountPanel(options: { book?: Book; onClearAndRescan?: () => Promise<void> } = {}): VueWrapper {
  return mount(SourceRecoveryPanel, {
    global: { plugins: [i18n] },
    props: {
      book: options.book ?? shelfBook(),
      onClearAndRescan: options.onClearAndRescan ?? vi.fn(async () => undefined),
    },
  });
}

describe('SourceRecoveryPanel', () => {
  it('renders active and alternate bindings in one known-source list', () => {
    const wrapper = mountPanel({ book: shelfBook(active, [{ sourceId: 'aggregate', sourceUrl: 'aggregate', bookUrl: '/alternate', sourceName: 'Aggregate', sourceGroup: 'Group A', capabilities: ['javascript', 'webview'], lastChapter: 'Alternate provider hint' }]) });
    expect(wrapper.findAll('.sources li')).toHaveLength(2);
    expect(wrapper.text()).toContain('Initial provider hint');
    expect(wrapper.text()).toContain('Alternate provider hint');
    expect(wrapper.text()).toContain('Current source');
    expect(wrapper.text()).toContain('Group A');
    expect(wrapper.text()).toContain('JavaScript');
    expect(wrapper.find('.sources li.current button').exists()).toBe(false);
  });

  it('emits a clean persisted binding when switching sources', async () => {
    const alternate: AltSource = { sourceId: 'aggregate', sourceUrl: 'aggregate', bookUrl: '/alternate', sourceName: 'Aggregate', lastChapter: 'Alternate provider hint' };
    const wrapper = mountPanel({ book: shelfBook(active, [alternate]) });
    const rows = wrapper.findAll('.sources li');
    expect(rows).toHaveLength(2);
    await rows[1]!.get('button').trigger('click');
    expect(wrapper.emitted('select')?.[0]?.[0]).toEqual(alternate);
  });

  it('deduplicates the active binding when it is also present in stored sources', () => {
    const wrapper = mountPanel({ book: shelfBook(active, [active, { sourceId: 'aggregate', sourceUrl: 'aggregate', bookUrl: '/other', sourceName: 'Other' }]) });
    expect(wrapper.findAll('.sources li')).toHaveLength(2);
    expect(wrapper.findAll('.sources li.current')).toHaveLength(1);
  });

  it('runs an opaque query against the active installed source and retains result display text', async () => {
    vi.mocked(searchApi.searchInstalledSource).mockResolvedValue([{ name: 'Book', author: 'Author', coverUrl: '', intro: '', kind: '', lastChapter: 'Provider B', bookUrl: '/provider-b', sourceId: 'aggregate', sourceUrl: 'aggregate', sourceName: 'Aggregate' }]);
    const wrapper = mountPanel();
    await wrapper.get('.targeted-search input').setValue('Book@provider');
    await wrapper.get('.targeted-search form').trigger('submit');
    expect(searchApi.searchInstalledSource).toHaveBeenCalledWith('aggregate', 'Book@provider');
    expect(wrapper.text()).toContain('Provider B');
    expect(wrapper.text()).toContain('Found with query: Book@provider');
    expect(wrapper.emitted('matches')?.[0]?.[0]).toEqual([expect.objectContaining({ bookUrl: '/provider-b', discoveryQuery: 'Book@provider', lastChapter: 'Provider B' })]);
  });

  it('preserves multiple bindings returned by one opaque partial query', async () => {
    vi.mocked(searchApi.searchInstalledSource).mockResolvedValue([
      { name: 'Book', author: 'Author', coverUrl: '', intro: '', kind: '', lastChapter: '69', bookUrl: '/69', sourceId: 'aggregate', sourceUrl: 'aggregate', sourceName: 'Aggregate' },
      { name: 'Book', author: 'Author', coverUrl: '', intro: '', kind: '', lastChapter: 'Fake 69', bookUrl: '/fake-69', sourceId: 'aggregate', sourceUrl: 'aggregate', sourceName: 'Aggregate' },
    ]);
    const wrapper = mountPanel();
    await wrapper.get('.targeted-search input').setValue('Book@69');
    await wrapper.get('.targeted-search form').trigger('submit');
    expect(wrapper.findAll('.sources li')).toHaveLength(3);
    expect(wrapper.emitted('matches')?.[0]?.[0]).toEqual([
      expect.objectContaining({ bookUrl: '/69', lastChapter: '69' }),
      expect.objectContaining({ bookUrl: '/fake-69', lastChapter: 'Fake 69' }),
    ]);
  });

  it('enriches and continues rendering the exact active binding', async () => {
    vi.mocked(searchApi.searchInstalledSource).mockResolvedValue([{ name: 'Book', author: 'Author', coverUrl: '', intro: '', kind: '', lastChapter: 'Refreshed provider hint', bookUrl: '/current', sourceId: 'aggregate', sourceUrl: 'aggregate', sourceName: 'Aggregate' }]);
    const wrapper = mountPanel();
    await wrapper.get('.targeted-search input').setValue('Book@current');
    await wrapper.get('.targeted-search form').trigger('submit');
    expect(wrapper.emitted('matches')?.[0]?.[0]).toEqual([expect.objectContaining({ bookUrl: '/current', discoveryQuery: 'Book@current', lastChapter: 'Refreshed provider hint' })]);
    expect(wrapper.findAll('.sources li')).toHaveLength(1);
    expect(wrapper.text()).toContain('Refreshed provider hint');
    expect(wrapper.find('.sources li.current').exists()).toBe(true);
  });

  it('filters known bindings by source metadata and display text', async () => {
    const wrapper = mountPanel({ book: shelfBook(active, [{ sourceId: 'alpha', sourceUrl: 'alpha', bookUrl: '/a', sourceName: 'Aggregate', lastChapter: 'Alpha provider' }, { sourceId: 'beta', sourceUrl: 'beta', bookUrl: '/b', sourceName: 'Aggregate', lastChapter: 'Beta provider' }]) });
    await wrapper.get('.source-filter input[type="search"]').setValue('Beta');
    expect(wrapper.text()).toContain('Beta provider');
    expect(wrapper.text()).not.toContain('Alpha provider');
  });

  it('keeps an active scan when the same logical book receives refreshed bindings', async () => {
    let handlers: searchApi.SearchBatchHandlers | undefined;
    vi.mocked(searchApi.searchBooksBatchStream).mockImplementation((_query, _options, value) => { handlers = value; return { close: vi.fn() } as unknown as EventSource; });
    const wrapper = mountPanel();
    await wrapper.findAll('button').find(button => button.text() === 'Find')?.trigger('click');
    handlers?.onStart({ offset: 0, eligible: 150, sourcesInBatch: 50, requestedConcurrency: 7, retryCursor: 'retry-0', effectiveConcurrency: 7 });
    handlers?.onDone({ complete: true, checked: 50, eligible: 150, hasMore: true, nextCursor: 'next-50', retryCursor: 'retry-0', sourceFailures: 0 });
    await wrapper.setProps({ book: shelfBook({ sourceId: 'found', sourceUrl: 'found', bookUrl: '/book', sourceName: 'Found' }, [active]) });
    const state = (wrapper.vm as unknown as { state: { checked: number; hasMore: boolean } }).state;
    expect(state.checked).toBe(50);
    expect(state.hasMore).toBe(true);
    expect(wrapper.text()).toContain('Initial provider hint');
  });

  it('requires confirmation before clear and rescan', async () => {
    const clear = vi.fn(async () => undefined);
    const wrapper = mountPanel({ onClearAndRescan: clear });
    await wrapper.findAll('button').find(button => button.text() === 'Clear')?.trigger('click');
    expect(wrapper.text()).toContain('Confirm');
    expect(clear).not.toHaveBeenCalled();
  });
});
