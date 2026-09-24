import { createPinia } from 'pinia';
import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, expect, it, vi } from 'vitest';
import * as api from '../../api/books';
import type { LibraryBook } from '../../api/models';
import { useImportQueue } from '../imports/import-queue';
import ShelfView from './ShelfView.vue';

const book = (id: string, updatedAt: number, durChapterIndex: number): LibraryBook => ({
  id, name: id, author: '', provider: 'txt', coverUrl: '', intro: '', kind: '', lastChapter: '',
  durChapterIndex, durChapterPos: 0, totalChapterNum: 100, contentRevision: 1, stateVersion: 1, updatedAt,
});
afterEach(() => vi.restoreAllMocks());

it('does not replace the continue-reading book when a newly added book refreshes the shelf', async () => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
  const read = { ...book('Previously read', 100, 0), lastReadAt: 100 };
  const added = book('Newly added', 200, 0);
  const list = vi.spyOn(api, 'listBooks').mockResolvedValue([read]);
  const pinia = createPinia();
  const view = mount(ShelfView, { global: { plugins: [pinia], stubs: { RouterLink: { template: '<a><slot /></a>' } }, mocks: { $t: (key: string) => key } } });
  try {
    await flushPromises();
    expect(view.get('.continue-section').text()).toContain(read.name);
    list.mockResolvedValue([read, added]);
    useImportQueue(pinia).libraryRevision++;
    await flushPromises();
    expect(view.get('.continue-section').text()).toContain(read.name);
    expect(view.get('.continue-section').text()).not.toContain(added.name);
    list.mockResolvedValue([added]);
    useImportQueue(pinia).libraryRevision++;
    await flushPromises();
    expect(view.find('.continue-section').exists()).toBe(false);
  } finally { view.unmount(); }
});
