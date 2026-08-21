import { describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import WebViewStatusCard from './WebViewStatusCard.vue';

vi.mock('../../api/system', () => ({ getWebViewStatus: vi.fn() }));
import { getWebViewStatus } from '../../api/system';

const messages: Record<string, string> = {
  'settings.webview.eyebrow': 'System capability',
  'settings.webview.title': 'WebView worker',
  'settings.webview.states.ready.title': 'Browser execution verified',
  'settings.webview.states.ready.description': 'Working',
  'settings.webview.checkedAt': 'Checked {time}',
  'settings.webview.checking': 'Checking…',
  'settings.webview.retry': 'Check again',
};

function mountCard() {
  return mount(WebViewStatusCard, { global: { mocks: { $t: (key: string, values?: Record<string, string>) => values?.time ? (messages[key] ?? key).replace('{time}', values.time) : messages[key] ?? key, $i18n: { locale: 'en' } }, stubs: { AppButton: { props: ['busy'], emits: ['click'], template: '<button @click="$emit(\'click\')"><slot /></button>' } } } });
}

describe('WebViewStatusCard', () => {
  it('checks on mount and can retry the browser execution diagnostic', async () => {
    vi.mocked(getWebViewStatus).mockResolvedValue({ status: 'ready', checkedAt: Date.now() });
    const wrapper = mountCard();
    await vi.waitFor(() => expect(wrapper.text()).toContain('Browser execution verified'));
    expect(getWebViewStatus).toHaveBeenCalledTimes(1);
    await wrapper.get('button').trigger('click');
    await vi.waitFor(() => expect(getWebViewStatus).toHaveBeenCalledTimes(2));
  });
});
