import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { expect, it } from 'vitest';
import SearchView from './SearchView.vue';
import { useSearchStore } from './search-store';

it('distinguishes first search, confirmed missing sources and interrupted search', async () => {
  const pinia = createPinia(); setActivePinia(pinia);
  const store = useSearchStore(); store.initialized = true;
  const view = mount(SearchView, { global: { plugins: [pinia], mocks: { $t: (key: string) => key, $route: { query: {} } }, stubs: { RouterLink: { template: '<a><slot /></a>' }, SearchControls: true, SearchStatus: true } } });
  expect(view.text()).toContain('search.guidance.title');
  expect(view.text()).not.toContain('search.guidance.noSources');
  store.searchedQuery = 'Novel'; await view.vm.$nextTick();
  expect(view.text()).toContain('search.guidance.noSources');
  store.retryRequired = true; await view.vm.$nextTick();
  expect(view.text()).not.toContain('search.guidance.noSources');
  store.retryRequired = false; store.eligible = 2; await view.vm.$nextTick();
  expect(view.text()).toContain('search.empty.title');
  view.unmount();
});
