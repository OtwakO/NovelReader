import { createPinia } from 'pinia';
import { createMemoryHistory, createRouter } from 'vue-router';
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import ShelfView from '../shelf/ShelfView.vue';
import ImportWorkspace from './ImportWorkspace.vue';
import { useImportQueue } from './import-queue';
import * as api from '../../api/txt-imports';
import * as books from '../../api/books';

let view: VueWrapper | undefined;
let queue: ReturnType<typeof useImportQueue>;
const receipt = (state: api.TXTState): api.TXTReceipt => ({ id: 'file', originalName: 'Story.txt', state, size: 100, createdAt: 0, updatedAt: 0, analysisVersion: 1, encoding: '', preset: '', hasError: false });
async function setup(component: typeof ShelfView | typeof ImportWorkspace) {
  const pinia = createPinia(); queue = useImportQueue(pinia);
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/shelf', component: { template: '<div />' } },
    { path: '/books/:bookId/read', name: 'reader', component: { template: '<div />' } },
  ] });
  await router.push('/shelf'); await router.isReady();
  view = mount(component, { global: { plugins: [pinia, router], mocks: { $t: (key: string) => key } } });
  await flushPromises();
  return router;
}
async function choose() {
  const input = view!.get('input[type="file"]');
  Object.defineProperty(input.element, 'files', { value: [new File(['text'], 'Story.txt')] });
  await input.trigger('change'); await flushPromises();
}
beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
  vi.spyOn(api, 'requestTXTAdmission').mockResolvedValue({ id: 'file', state: 'granted', expiresAt: '', maxInputBytes: 1000 });
  vi.spyOn(api, 'acquireTXT').mockResolvedValue({ receipt: receipt('received') });
});
afterEach(async () => { view?.unmount(); queue?.resetReaderState(); await flushPromises(); vi.restoreAllMocks(); });

it('imports from the shelf and offers Read without navigation, review or an inbox scan', async () => {
  const list = vi.spyOn(books, 'listBooks').mockResolvedValue([]);
  const inbox = vi.spyOn(api, 'scanTXTInbox');
  vi.spyOn(api, 'getTXTReceipt').mockResolvedValue(receipt('ready'));
  const accept = vi.spyOn(api, 'acceptTXT').mockResolvedValue({ libraryId: 'book' });
  const router = await setup(ShelfView); await choose();
  expect(accept).toHaveBeenCalledOnce();
  expect(list).toHaveBeenCalledTimes(2);
  expect(router.currentRoute.value.path).toBe('/shelf');
  expect(view!.get('.import-read').attributes('href')).toBe('/books/book/read');
  expect(view!.find('.import-editor').exists()).toBe(false);
  expect(inbox).not.toHaveBeenCalled();
});

it('handles a warning inline with advanced controls collapsed and adds only on explicit approval', async () => {
  const status = vi.spyOn(api, 'getTXTReceipt').mockResolvedValue(receipt('needs_review'));
  vi.spyOn(api, 'previewTXT').mockResolvedValue({ analysisVersion: 1, encoding: 'utf-8', preset: 'generated-sections', parserVersion: 1, reviewReasons: ['no-headings'], totalSections: 1, headings: [{ index: 0, title: 'Section 1', generated: true }], hasMore: false, sample: '<b>literal text</b>', sampleTruncated: false });
  const accept = vi.spyOn(api, 'acceptTXT').mockResolvedValue({ libraryId: 'book' });
  const router = await setup(ImportWorkspace); await choose();
  expect(accept).not.toHaveBeenCalled();
  await view!.findAll('button').find(button => button.text() === 'imports.flow.checkBook')!.trigger('click'); await flushPromises();
  expect(router.currentRoute.value.path).toBe('/shelf');
  expect(view!.get('pre').text()).toBe('<b>literal text</b>');
  expect(view!.get('.import-editor details').attributes('open')).toBeUndefined();
  status.mockResolvedValue({ ...receipt('published'), libraryId: 'book' });
  await view!.get('.import-editor form').trigger('submit'); await flushPromises();
  expect(accept).toHaveBeenCalledExactlyOnceWith('file', 1, 'Story', '', expect.any(AbortSignal));
  expect(view!.find('.import-editor').exists()).toBe(false);
  expect(view!.find('.import-read').exists()).toBe(true);
});
