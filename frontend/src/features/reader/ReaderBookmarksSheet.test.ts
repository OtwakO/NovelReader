import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, expect, it, vi } from 'vitest';
import { deleteBookmark, listBookmarks, type Bookmark } from '../../api/reader';
import { createI18n } from 'vue-i18n';
import readerMessages from '../../i18n/messages/en/reader';
import ReaderBookmarksSheet from './ReaderBookmarksSheet.vue';
import { resetProgressWriter, setProgressVersion } from './progress-writer';

const i18n = createI18n({ legacy: false, globalInjection: true, locale: 'en', messages: { en: { reader: readerMessages } } });

vi.mock('../../api/reader', () => ({ listBookmarks: vi.fn(), deleteBookmark: vi.fn(), addBookmark: vi.fn(), saveProgress: vi.fn() }));
afterEach(() => { resetProgressWriter(); vi.resetAllMocks(); });

it('deletes an orphan using the current interpretation, not its old location revision', async () => {
  const mark: Bookmark = { id: 'mark', bookId: 'book', contentRevision: 1, chapterIndex: 0, chapterTitle: 'Old chapter', position: .5, note: '', orphaned: true, createdAt: 0 };
  vi.mocked(listBookmarks).mockResolvedValue([mark]);
  vi.mocked(deleteBookmark).mockResolvedValue({ status: 'deleted', stateVersion: 5 });
  setProgressVersion('book', 4);
  const wrapper = mount(ReaderBookmarksSheet, {
    global: { plugins: [i18n] },
    props: { bookId: 'book', currentRevision: 2, capture: async () => ({ contentRevision: 2, chapterIndex: 0, position: 0 }) },
  });
  try {
    await flushPromises();
    await wrapper.vm.remove(mark);
    expect(deleteBookmark).toHaveBeenCalledWith('book', 'mark', 2, 4);
    expect(wrapper.findAll('li')).toHaveLength(0);
  } finally { wrapper.unmount(); }
});

it('labels an untitled bookmark by its section number and reopens its exact location', async () => {
  const mark: Bookmark = { id: 'untitled', bookId: 'book', contentRevision: 7, chapterIndex: 2, chapterTitle: '', position: .4, note: '', orphaned: false, createdAt: 0 };
  vi.mocked(listBookmarks).mockResolvedValue([mark]);
  const wrapper = mount(ReaderBookmarksSheet, {
    global: { plugins: [i18n] },
    props: { bookId: 'book', currentRevision: 7, capture: async () => ({ contentRevision: 7, chapterIndex: 0, position: 0 }) },
  });
  try {
    await flushPromises();
    expect(wrapper.get('li strong').text()).toBe('3');
    await wrapper.get('li button').trigger('click');
    expect(wrapper.emitted('open')).toEqual([[2, .4, 7]]);
    expect(mark.chapterTitle).toBe('');
  } finally { wrapper.unmount(); }
});
