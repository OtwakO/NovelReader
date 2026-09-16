import { createPinia } from 'pinia';
import { flushPromises, mount } from '@vue/test-utils';
import { expect, it, vi } from 'vitest';
import * as api from '../../api/explore';
import ExploreView from './ExploreView.vue';

it('searches source names/groups without opening a catalog until selection', async () => {
  sessionStorage.clear();
  const sources = [{ id: 'a', name: 'Alpha', group: 'English' }, { id: 'b', name: '乙書源', group: '中文' }];
  vi.spyOn(api, 'listExploreSources').mockResolvedValue(sources);
  const open = vi.spyOn(api, 'openExplore').mockResolvedValue({ source: sources[1]!, sessionId: 'session', entries: [], diagnostics: [] });
  const view = mount(ExploreView, { global: { plugins: [createPinia()], stubs: { RouterLink: true }, mocks: { $t: (key: string) => key } } });
  try {
    await flushPromises();
    await view.get('#explore-source').trigger('click');
    await view.get('[role="combobox"]').setValue('中文');
    expect(view.findAll('[role="option"]')).toHaveLength(1);
    expect(open).not.toHaveBeenCalled();
    await view.get('[role="option"]').trigger('click');
    await flushPromises();
    expect(open).toHaveBeenCalledExactlyOnceWith('b', expect.any(AbortSignal));
    expect(view.get('#explore-source').text()).toContain('乙書源');
  } finally { view.unmount(); vi.restoreAllMocks(); sessionStorage.clear(); }
});
