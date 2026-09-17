import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import { createMemoryHistory, createRouter } from 'vue-router';
import type { CatalogNavigation } from '../../api/catalog-navigation';
import ReaderTocSheet from './ReaderTocSheet.vue';
import BookDetailToc from '../books/BookDetailToc.vue';

const navigation: CatalogNavigation = { source: 'publication', entries: [
  { label: 'Part', unavailable: false, children: [
    { label: 'Opening', unavailable: false, children: [], target: { chapterIndex: 2, contentRevision: 7, anchor: 'a1' } },
    { label: 'Later heading', unavailable: false, children: [], target: { chapterIndex: 2, contentRevision: 7, anchor: 'a2' } },
    { label: 'Missing parent', unavailable: true, children: [
      { label: 'Notes', unavailable: false, children: [], target: { chapterIndex: 1, contentRevision: 7, anchor: 'note' } },
    ] },
  ] },
] };
const chapters = [0, 1, 2].map(index => ({ index, title: `Chapter ${index}`, isVolume: false, auxiliary: index === 1 }));
let wrapper: VueWrapper;
const scrollTo = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'scrollTo');
beforeEach(() => Object.defineProperty(HTMLElement.prototype, 'scrollTo', { configurable: true, value: vi.fn() }));
afterEach(() => {
  wrapper?.unmount();
  if (scrollTo) Object.defineProperty(HTMLElement.prototype, 'scrollTo', scrollTo);
  else Reflect.deleteProperty(HTMLElement.prototype, 'scrollTo');
});
const mocks = { $t: (key: string) => key };

it('searches an expanded outline with ancestor context and emits the exact qualified target', async () => {
  wrapper = mount(ReaderTocSheet, { props: { chapters, navigation, currentIndex: 2, currentAnchor: 'a2' }, global: { mocks } });
  await flushPromises();
  expect(wrapper.findAll('[aria-current="location"]')).toHaveLength(1);
  expect(wrapper.get('[aria-current="location"]').text()).toBe('Later heading');
  expect(wrapper.text()).not.toContain('reader.toc.ascending');
  expect(wrapper.findAll('.navigation-row button').map(button => button.text())).toEqual(['Opening', 'Later heading', 'Notes']);
  await wrapper.get('input').setValue('notes');
  expect(wrapper.findAll('.navigation-row').map(row => row.text())).toEqual(['Part', 'Missing parentreader.toc.unavailable', 'Notes']);
  await wrapper.get('.navigation-row button').trigger('click');
  expect(wrapper.emitted('openTarget')).toEqual([[{ chapterIndex: 1, contentRevision: 7, anchor: 'note' }]]);
  await wrapper.get('.toc-tools button:last-child').trigger('click');
  expect((wrapper.get('input').element as HTMLInputElement).value).toBe('');
  expect(wrapper.get('[aria-current="location"]').text()).toBe('Later heading');
  await wrapper.get('input').setValue('not present');
  expect(wrapper.text()).toContain('reader.toc.outlineNoMatches');
});

it('uses qualified links in Book Detail, labels generated sections, and disables stale actions', async () => {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div />' } }, { name: 'reader', path: '/read/:bookId/:chapterIndex', component: { template: '<div />' } }] });
  wrapper = mount(BookDetailToc, { props: { chapters, navigation: { ...navigation, source: 'sections' }, currentIndex: 2, bookId: 'book', contentRevision: 7 }, global: { mocks, plugins: [router] } });
  expect(wrapper.text()).toContain('reader.toc.sections');
  expect(wrapper.findAll('.navigation-row a')).toHaveLength(3);
  expect(wrapper.get('.navigation-row a').attributes('href')).toBe('/read/book/2?contentRevision=7&anchor=a1');
  expect(wrapper.findAll('.navigation-row.grouping a')).toHaveLength(0);
  await wrapper.setProps({ interactive: false });
  expect(wrapper.findAll('.navigation-row a, .navigation-row button')).toHaveLength(0);
  await wrapper.setProps({ navigation: undefined, interactive: true });
  expect(wrapper.find('.navigation-list').exists()).toBe(false);
  expect(wrapper.text()).toContain('reader.toc.descending');
});
