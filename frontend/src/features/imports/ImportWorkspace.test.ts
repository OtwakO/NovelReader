import { mount, flushPromises } from '@vue/test-utils';
import { createPinia } from 'pinia';
import { createMemoryHistory, createRouter } from 'vue-router';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import * as api from '../../api/txt-imports';
import * as books from '../../api/books';
import ShelfView from '../shelf/ShelfView.vue';
import ImportWorkspace from './ImportWorkspace.vue';
import ImportsView from './ImportsView.vue';
import { useImportQueue } from './import-queue';

let queue: ReturnType<typeof useImportQueue>;
let view: ReturnType<typeof mount> | undefined;
const receipt = (state: api.TXTState): api.TXTReceipt => ({ id: 'file', state, originalName: 'A story.txt', size: 10, createdAt: 0, updatedAt: 0, analysisVersion: 1, encoding: '', preset: '', hasError: false });
beforeEach(() => {
  localStorage.clear();
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
  vi.spyOn(api, 'requestTXTAdmission').mockResolvedValue({ id: 'ticket', expiresAt: '', maxInputBytes: 1000, state: 'granted' });
  vi.spyOn(api, 'acquireTXT').mockResolvedValue({ receipt: receipt('received') });
});
afterEach(async () => { view?.unmount(); queue.resetReaderState(); await flushPromises(); vi.restoreAllMocks(); localStorage.clear(); });

async function setup(component: typeof ShelfView | typeof ImportWorkspace | typeof ImportsView) {
  const pinia = createPinia(); queue = useImportQueue(pinia);
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/shelf', component: { template: '<div />' } },
    { path: '/imports', component: { template: '<div />' } },
    { path: '/search', component: { template: '<div />' } },
    { path: '/books/:bookId/read', name: 'reader', component: { template: '<div />' } },
  ] });
  await router.push(component === ShelfView ? '/shelf' : '/imports'); await router.isReady();
  view = mount(component, { global: { plugins: [pinia, router], mocks: { $t: (key: string) => key } } });
  await flushPromises(); return router;
}
async function choose() {
  const input = view!.get('input[type="file"]');
  Object.defineProperty(input.element, 'files', { configurable: true, value: [new File(['content'], 'A story.txt')] });
  await input.trigger('change'); await flushPromises();
}

it('keeps only a Local import link on the shelf while background additions still refresh books', async () => {
  const list = vi.spyOn(books, 'listBooks').mockResolvedValue([]);
  const inbox = vi.spyOn(api, 'scanTXTInbox');
  const router = await setup(ShelfView);
  expect(view!.find('input[type="file"]').exists()).toBe(false);
  expect(view!.findComponent(ImportWorkspace).exists()).toBe(false);
  queue.libraryRevision++;
  await flushPromises();
  expect(list).toHaveBeenCalledTimes(2);
  const link = view!.get('a[href="/imports"]');
  expect(link.text()).toBe('imports.title');
  await link.trigger('click'); await flushPromises();
  expect(router.currentRoute.value.path).toBe('/imports');
  expect(inbox).not.toHaveBeenCalled();
});

it('opts into automatic addition from the dedicated page and offers Read without opening review or scanning inbox', async () => {
  vi.spyOn(api, 'getTXTReceipt').mockResolvedValue(receipt('ready'));
  const accept = vi.spyOn(api, 'acceptTXT').mockResolvedValue({ libraryId: 'book' });
  const inbox = vi.spyOn(api, 'scanTXTInbox');
  const preview = vi.spyOn(api, 'previewTXT');
  const router = await setup(ImportsView);
  expect(view!.get<HTMLInputElement>('.import-preference input').element.checked).toBe(true);
  await view!.get('.import-preference input').setValue(false);
  await choose();
  expect(accept).toHaveBeenCalledOnce();
  expect(inbox).not.toHaveBeenCalled(); expect(preview).not.toHaveBeenCalled();
  expect(router.currentRoute.value.path).toBe('/imports');
  expect(view!.find('a[href="/books/book/read"]').text()).toBe('imports.flow.read');
  expect(view!.findAll('form')).toHaveLength(0);
});

it('opens exception review inline and closes it after explicit addition', async () => {
  const status = vi.spyOn(api, 'getTXTReceipt').mockResolvedValue(receipt('needs_review'));
  vi.spyOn(api, 'previewTXT').mockResolvedValue({ analysisVersion: 1, encoding: 'utf-8', preset: 'generated-sections', parserVersion: 1, reviewReasons: ['no_headings'], totalSections: 1, headings: [{ index: 0, title: 'Section 1', generated: true }], sample: 'Literal prose', sampleTruncated: false, hasMore: false });
  const accept = vi.spyOn(api, 'acceptTXT').mockResolvedValue({ libraryId: 'book' });
  const router = await setup(ImportWorkspace); await choose();
  expect(accept).not.toHaveBeenCalled();
  await view!.findAll('button').find(button => button.text() === 'imports.flow.checkBook')!.trigger('click'); await flushPromises();
  expect(router.currentRoute.value.path).toBe('/imports');
  expect(view!.find('.import-editor').exists()).toBe(true);
  expect(view!.findAll('summary').map(summary => summary.text())).toContain('imports.flow.adjustChapters');
  expect(view!.text()).toContain('Literal prose');
  status.mockResolvedValue({ ...receipt('published'), libraryId: 'book' });
  await view!.get('form').trigger('submit'); await flushPromises();
  expect(accept).toHaveBeenCalledOnce();
  expect(view!.find('.import-editor').exists()).toBe(false);
  expect(view!.find('a[href="/books/book/read"]').exists()).toBe(true);
});

it('keeps linked review accessible and clears its query when closed', async () => {
  vi.spyOn(api, 'getTXTReceipt').mockResolvedValue(receipt('failed'));
  const router = await setup(ImportsView);
  await router.push('/imports?review=file'); await flushPromises();
  expect(view!.find('.import-editor').exists()).toBe(true);
  await view!.findAll('button').find(button => button.text() === 'imports.flow.closeReview')!.trigger('click'); await flushPromises();
  expect(view!.find('.import-editor').exists()).toBe(false);
  expect(router.currentRoute.value.query.review).toBeUndefined();
});
