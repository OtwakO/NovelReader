import { createPinia } from 'pinia';
import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, expect, it, vi } from 'vitest';
import SettingsView from './SettingsView.vue';

vi.mock('../../api/reader', () => ({ listFonts: vi.fn().mockResolvedValue([]), getFontUrl: vi.fn(), uploadFont: vi.fn(), deleteFont: vi.fn() }));
vi.mock('../../i18n', () => ({ localeFromValue: vi.fn(), setLocale: vi.fn() }));
afterEach(() => localStorage.clear());

it('puts everyday preferences first and mounts diagnostics only when opened', async () => {
  const view = mount(SettingsView, { global: { plugins: [createPinia()], mocks: { $t: (key: string) => key, $i18n: { locale: 'en' } }, stubs: { WebViewStatusCard: { template: '<div data-diagnostics />' } } } });
  await flushPromises();
  const sections = view.findAll('.settings > section');
  expect(sections.map(section => section.classes()[1])).toEqual(['language', 'reader-defaults', 'imports', 'fonts']);
  expect(view.find('[data-diagnostics]').exists()).toBe(false);
  const details = view.get('details');
  (details.element as HTMLDetailsElement).open = true;
  await details.trigger('toggle');
  expect(view.find('[data-diagnostics]').exists()).toBe(true);
  await view.get('input[type="range"]').setValue('24');
  expect(view.get('.preview').attributes('style')).toContain('--preview-size: 24px');
  view.unmount();
});
