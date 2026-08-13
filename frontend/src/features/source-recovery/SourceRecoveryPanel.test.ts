import { mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { describe, expect, it, vi } from 'vitest';
import SourceRecoveryPanel from './SourceRecoveryPanel.vue';

vi.mock('../../api/search', () => ({ searchBooksBatchStream: vi.fn(() => ({ close: vi.fn() })) }));
const i18n = createI18n({ legacy: false, globalInjection: true, locale: 'en', messages: { en: { sourceRecovery: { eyebrow: 'Recovery', title: 'Sources', description: 'Description', stop: 'Stop', rescan: 'Clear', clearing: 'Clearing', confirmTitle: 'Confirm', confirmDescription: 'Description', cancel: 'Cancel', confirm: 'Confirm', switching: 'Switching', use: 'Use', empty: 'Empty', find: 'Find' }, search: { controls: { title: 'Controls', batchSize: 'Batch', intensity: 'Intensity', gentle: 'Gentle', balanced: 'Balanced', fast: 'Fast', advanced: 'Advanced', concurrency: 'Concurrency' }, status: { checkedOf: '', checked: '', results: '', concurrency: '', failures: '', disconnected: '', stale: '', storage: '' }, actions: { restart: '', retry: '', more: '' } } } } });

describe('SourceRecoveryPanel', () => {
  it('shows persisted alternate sources immediately', () => {
    const wrapper = mount(SourceRecoveryPanel, { global: { plugins: [i18n] }, props: { book: { name: 'Book', author: 'Author' }, currentSourceUrl: 'current', storedSources: [{ sourceUrl: 'stored', bookUrl: '/book', sourceName: 'Stored source' }], onClearAndRescan: vi.fn(async () => undefined) } });
    expect(wrapper.text()).toContain('Stored source');
  });
  it('requires an explicit confirmation before clearing stored sources', async () => {
    const wrapper = mount(SourceRecoveryPanel, { global: { plugins: [i18n] }, props: { book: { name: 'Book', author: 'Author' }, currentSourceUrl: 'current', storedSources: [], onClearAndRescan: vi.fn(async () => undefined) } });
    await wrapper.findAll('button').find((button) => button.text() === 'Clear')?.trigger('click');
    expect(wrapper.text()).toContain('Confirm'); expect(wrapper.props('onClearAndRescan')).not.toHaveBeenCalled();
  });
});
