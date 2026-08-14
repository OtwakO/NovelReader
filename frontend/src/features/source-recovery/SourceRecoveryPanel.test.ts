import { mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as searchApi from '../../api/search';
import SourceRecoveryPanel from './SourceRecoveryPanel.vue';

vi.mock('../../api/search', async () => { const actual = await vi.importActual<typeof import('../../api/search')>('../../api/search'); return { ...actual, searchBooksBatchStream: vi.fn(() => ({ close: vi.fn() })) }; });
beforeEach(() => vi.clearAllMocks());
const i18n = createI18n({ legacy: false, globalInjection: true, locale: 'en', messages: { en: { sourceRecovery: { eyebrow: 'Recovery', title: 'Sources', description: 'Description', stop: 'Stop', rescan: 'Clear', clearing: 'Clearing', confirmTitle: 'Confirm', confirmDescription: 'Description', cancel: 'Cancel', confirm: 'Confirm', switching: 'Switching', use: 'Use', empty: 'Empty', find: 'Find' }, search: { controls: { title: 'Controls', batchSize: 'Batch', intensity: 'Intensity', gentle: 'Gentle', balanced: 'Balanced', fast: 'Fast', advanced: 'Advanced', concurrency: 'Concurrency' }, status: { checkedOf: '', checked: '', results: '', concurrency: '', failures: '', disconnected: '', stale: '', storage: '' }, actions: { restart: '', retry: '', more: '' } } } } });

describe('SourceRecoveryPanel', () => {
  it('shows persisted alternate sources immediately', () => {
    const wrapper = mount(SourceRecoveryPanel, { global: { plugins: [i18n] }, props: { book: { name: 'Book', author: 'Author' }, currentSourceUrl: 'current', storedSources: [{ sourceUrl: 'stored', bookUrl: '/book', sourceName: 'Stored source' }], onClearAndRescan: vi.fn(async () => undefined) } });
    expect(wrapper.text()).toContain('Stored source');
  });
  it('keeps an active scan when the same logical book receives refreshed props', async () => {
    let handlers: searchApi.SearchBatchHandlers | undefined;
    vi.mocked(searchApi.searchBooksBatchStream).mockImplementation((_query, _options, value) => { handlers = value; return { close: vi.fn() } as unknown as EventSource; });
    const wrapper = mount(SourceRecoveryPanel, { global: { plugins: [i18n] }, props: { book: { name: 'Book', author: 'Author' }, currentSourceUrl: 'current', storedSources: [], onClearAndRescan: vi.fn(async () => undefined) } });
    await wrapper.findAll('button').find((button) => button.text() === 'Find')?.trigger('click');
    handlers?.onStart({ offset: 0, eligible: 150, sourcesInBatch: 50, requestedConcurrency: 7, retryCursor: 'retry-0', effectiveConcurrency: 7 });
    handlers?.onDone({ complete: true, checked: 50, eligible: 150, hasMore: true, nextCursor: 'next-50', retryCursor: 'retry-0', sourceFailures: 0 });
    await wrapper.setProps({ book: { name: 'Book', author: 'Author' }, currentSourceUrl: 'found', storedSources: [{ sourceUrl: 'found', bookUrl: '/book', sourceName: 'Found' }, { sourceUrl: 'current', bookUrl: '/old', sourceName: 'Previous' }] });
    expect(wrapper.vm.state.checked).toBe(50); expect(wrapper.vm.state.hasMore).toBe(true); expect(wrapper.text()).toContain('Previous');
  });
  it('requires an explicit confirmation before clearing stored sources', async () => {
    const wrapper = mount(SourceRecoveryPanel, { global: { plugins: [i18n] }, props: { book: { name: 'Book', author: 'Author' }, currentSourceUrl: 'current', storedSources: [], onClearAndRescan: vi.fn(async () => undefined) } });
    await wrapper.findAll('button').find((button) => button.text() === 'Clear')?.trigger('click');
    expect(wrapper.text()).toContain('Confirm'); expect(wrapper.props('onClearAndRescan')).not.toHaveBeenCalled();
  });
});
