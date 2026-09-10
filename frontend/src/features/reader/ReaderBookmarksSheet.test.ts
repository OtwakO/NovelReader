import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, expect, it, vi } from 'vitest';
import { deleteBookmark, listBookmarks, type Bookmark } from '../../api/reader';
import { i18n } from '../../i18n';
import ReaderBookmarksSheet from './ReaderBookmarksSheet.vue';
import { resetProgressWriter, setProgressVersion } from './progress-writer';

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
