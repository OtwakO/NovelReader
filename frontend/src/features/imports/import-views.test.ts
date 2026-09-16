import { createPinia } from 'pinia';
import { createMemoryHistory, createRouter } from 'vue-router';
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import { afterEach, expect, it, vi } from 'vitest';
import * as api from '../../api/txt-imports';
import * as books from '../../api/books';
import { ApiError } from '../../api/transport';
import ImportReviewView from './ImportReviewView.vue';
import ImportInboxPanel from './ImportInboxPanel.vue';
import ImportReceiptsPanel from './ImportReceiptsPanel.vue';
import { useImportQueue } from './import-queue';
const wrappers: VueWrapper[] = [];
afterEach(() => { wrappers.splice(0).forEach(view => view.unmount()); vi.restoreAllMocks(); });
const receipt = (changes: Partial<api.TXTReceipt> = {}): api.TXTReceipt => ({ id: 'sample', originalName: 'sample.txt', state: 'needs_review', size: 100, createdAt: 0, updatedAt: 0, analysisVersion: 1, encoding: '', preset: '', hasError: false, ...changes });
const preview = (version = 1): api.TXTPreview => ({ analysisVersion: version, encoding: 'utf-8', preset: 'generated-sections', parserVersion: 1, reviewReasons: ['no-headings'], totalSections: 1, headings: [{ index: 0, title: 'Section 1', generated: true }], hasMore: false, sample: '<script>literal prose</script>', sampleTruncated: true });
async function routerFor(path: string) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/imports/:id?', component: { template: '<div />' } }, { path: '/books/:bookId/read', name: 'reader', component: { template: '<div />' } }] });
  await router.push(path); await router.isReady(); return router;
}
function button(view: VueWrapper, key: string) { return view.findAll('button').find(item => item.text() === key)!; }

it('renders literal bounded preview and only admits the explicitly reviewed version', async () => {
  const get = vi.spyOn(api, 'getTXTReceipt').mockResolvedValue(receipt());
  const sample = vi.spyOn(api, 'previewTXT').mockResolvedValue(preview());
  const analyze = vi.spyOn(api, 'analyzeTXT').mockResolvedValue({});
  const accept = vi.spyOn(api, 'acceptTXT').mockResolvedValue({ libraryId: 'sample' });
  const view = mount(ImportReviewView, { props: { receiptId: 'sample' }, global: { plugins: [createPinia(), await routerFor('/imports/sample?state=needs_review')], mocks: { $t: (key: string) => key } } }); wrappers.push(view);
  await flushPromises();
  expect(view.get('pre').text()).toBe('<script>literal prose</script>');
  expect(view.find('script').exists()).toBe(false);
  expect(view.text()).toContain('imports.flow.reviewHint');
  expect(view.text()).toContain('imports.flow.chapterCount');
  expect(button(view, 'imports.refresh')).toBeUndefined();
  expect(view.find('.import-review-status').exists()).toBe(false);
  const add = button(view, 'imports.confirmAdd');
  expect(add.element.parentElement).toBe(button(view, 'imports.discard').element.parentElement);
  expect(add.attributes('form')).toBe(view.get('form').attributes('id'));
  expect(view.get('.import-editor-heading').text()).not.toContain('imports.confirmAdd');
  await view.findAll('select')[0]!.setValue('big5');
  expect(button(view, 'imports.confirmAdd').attributes('disabled')).toBeDefined();
  expect(analyze).not.toHaveBeenCalled();
  get.mockResolvedValue(receipt({ state: 'received', analysisVersion: 2, encoding: 'big5' }));
  await button(view, 'imports.analyze').trigger('click'); await flushPromises();
  expect(analyze).toHaveBeenCalledWith('sample', 1, { encoding: 'big5', preset: '', pattern: '' }, expect.any(AbortSignal));
  expect(view.find('pre').exists()).toBe(false);
  get.mockResolvedValue(receipt({ state: 'ready', analysisVersion: 3, encoding: 'big5' })); sample.mockResolvedValue(preview(3));
  await button(view, 'imports.refresh').trigger('click'); await flushPromises();
  expect(button(view, 'imports.refresh')).toBeUndefined();
  get.mockResolvedValue(receipt({ state: 'published', analysisVersion: 3, encoding: 'big5', libraryId: 'sample' }));
  await view.get('form').trigger('submit'); await flushPromises();
  expect(accept).toHaveBeenCalledWith('sample', 3, 'sample', '', expect.any(AbortSignal));
  expect(view.text()).toContain('imports.flow.read');
});

it('keeps incomplete cleanup visible through a failed retry', async () => {
  vi.spyOn(api, 'getTXTReceipt').mockResolvedValue(receipt()); vi.spyOn(api, 'previewTXT').mockResolvedValue(preview());
  const discard = vi.spyOn(api, 'discardTXT').mockResolvedValueOnce({ status: 'removed', warnings: ['txt_cleanup_pending'] }).mockRejectedValueOnce(new Error('unavailable')).mockResolvedValue({ status: 'deleted' });
  const view = mount(ImportReviewView, { props: { receiptId: 'sample' }, global: { plugins: [createPinia(), await routerFor('/imports/sample')], mocks: { $t: (key: string) => key } } }); wrappers.push(view);
  await flushPromises(); await button(view, 'imports.discard').trigger('click'); await button(view, 'imports.confirmDiscard').trigger('click'); await flushPromises();
  expect(view.text()).toContain('imports.cleanupPending'); expect(view.find('form').exists()).toBe(false);
  await button(view, 'imports.retryCleanup').trigger('click'); await flushPromises();
  expect(view.text()).toContain('imports.cleanupPending'); expect(view.find('[role="alert"]').exists()).toBe(true);
  await button(view, 'imports.retryCleanup').trigger('click'); await flushPromises();
  expect(view.text()).toContain('imports.discarded'); expect(discard).toHaveBeenCalledTimes(3);
});

it('consumes inbox approval on failure and requires a fresh explicit review before resolution', async () => {
  vi.spyOn(api, 'scanTXTInbox').mockResolvedValue({ items: [], directory: 'inbox/reader' });
  vi.spyOn(api, 'listTXTInboxClaims').mockResolvedValue({ items: [{ name: 'leftover.txt', receiptId: 'sample' }] });
  const review = vi.spyOn(api, 'reviewTXTInbox').mockResolvedValueOnce({ token: 'old', expiresAt: '', name: 'leftover.txt', receiptId: 'sample', inputPresent: true, canRemove: true }).mockResolvedValue({ token: 'new', expiresAt: '', name: 'leftover.txt', receiptId: 'sample', inputPresent: true, canRemove: false });
  const resolve = vi.spyOn(api, 'resolveTXTInbox').mockRejectedValueOnce(new ApiError(409, { code: 'txt_inbox_changed' })).mockResolvedValue(undefined);
  const view = mount(ImportInboxPanel, { global: { plugins: [createPinia()], mocks: { $t: (key: string) => key } } }); wrappers.push(view);
  await flushPromises(); await button(view, 'imports.reviewLeftover').trigger('click'); await flushPromises();
  await button(view, 'imports.removeDuplicate').trigger('click'); await flushPromises();
  expect(view.text()).toContain('imports.errors.proof'); expect(view.find('.import-confirmation').exists()).toBe(false);
  expect(review).toHaveBeenCalledTimes(1); expect(resolve).toHaveBeenCalledExactlyOnceWith('old', 'confirm', expect.any(AbortSignal));
  await button(view, 'imports.reviewLeftover').trigger('click'); await flushPromises();
  expect(button(view, 'imports.removeDuplicate').attributes('disabled')).toBeDefined();
  await button(view, 'imports.releaseClaim').trigger('click'); await flushPromises();
  expect(resolve).toHaveBeenLastCalledWith('new', 'release', expect.any(AbortSignal));
});

it('bulk admission retains selected versions and reports partial success', async () => {
  const list = vi.spyOn(api, 'listTXTReceipts').mockResolvedValue({ items: [receipt({ id: 'a', state: 'ready' }), receipt({ id: 'b', state: 'ready' }), receipt({ id: 'uncertain' })] });
  const accept = vi.spyOn(api, 'acceptTXT').mockResolvedValueOnce({ libraryId: 'a' }).mockRejectedValueOnce(new ApiError(409, { code: 'txt_state_changed' }));
  const view = mount(ImportReceiptsPanel, { global: { plugins: [createPinia(), await routerFor('/imports')], mocks: { $t: (key: string) => key } } }); wrappers.push(view);
  await flushPromises(); const inputs = view.findAll('input'); await inputs[0]!.setValue(true); await inputs[1]!.setValue(true);
  expect(inputs[2]!.attributes('disabled')).toBeDefined();
  list.mockResolvedValue({ items: [receipt({ id: 'a', state: 'ready', analysisVersion: 9 }), receipt({ id: 'b', state: 'ready' })] });
  await button(view, 'imports.refresh').trigger('click'); await flushPromises();
  await button(view, 'imports.addSelected').trigger('click'); await flushPromises();
  expect(accept.mock.calls.map(call => call.slice(0, 2))).toEqual([['a', 1], ['b', 1]]);
  expect(view.text()).toContain('imports.batchResult'); expect(view.text()).toContain('imports.errors.changed');
});

it('routes a persisted publication cleanup retry through library removal, not pending discard', async () => {
  vi.spyOn(api, 'getTXTReceipt').mockResolvedValue(receipt({ state: 'removing', libraryId: 'sample' }));
  const pending = vi.spyOn(api, 'discardTXT');
  const remove = vi.spyOn(books, 'deleteBook').mockResolvedValue({ status: 'deleted' });
  const view = mount(ImportReviewView, { props: { receiptId: 'sample' }, global: { plugins: [createPinia(), await routerFor('/imports/sample')], mocks: { $t: (key: string) => key } } }); wrappers.push(view);
  await flushPromises(); expect(view.text()).toContain('imports.libraryCleanupPending');
  await button(view, 'imports.retryCleanup').trigger('click'); await flushPromises();
  expect(remove).toHaveBeenCalledWith('sample', expect.any(AbortSignal)); expect(pending).not.toHaveBeenCalled();
  expect(view.text()).toContain('imports.discarded');
});

it('retains custom drafts, shows server validation, and requires their saved preview before acceptance', async () => {
  const original = '(?i)part [0-9]+';
  const corrected = '(?i)part [0-9]+.*';
  const get = vi.spyOn(api, 'getTXTReceipt').mockResolvedValue(receipt({ preset: 'custom', pattern: original }));
  const sample = vi.spyOn(api, 'previewTXT').mockResolvedValue(preview());
  const analyze = vi.spyOn(api, 'analyzeTXT').mockRejectedValueOnce(new ApiError(400, { code: 'txt_invalid_pattern' })).mockResolvedValue({});
  const accept = vi.spyOn(api, 'acceptTXT');
  const view = mount(ImportReviewView, { props: { receiptId: 'sample' }, global: { plugins: [createPinia(), await routerFor('/imports/sample')], mocks: { $t: (key: string) => key } } }); wrappers.push(view);
  await flushPromises();
  expect((view.get('textarea').element as HTMLTextAreaElement).value).toBe(original);
  await view.get('textarea').setValue('(?=part)part');
  expect(button(view, 'imports.confirmAdd').attributes('disabled')).toBeDefined();
  await view.get('form').trigger('submit'); expect(accept).not.toHaveBeenCalled();
  await button(view, 'imports.analyze').trigger('click'); await flushPromises();
  expect(view.get('textarea').attributes('aria-invalid')).toBe('true');
  expect(view.get('#pattern-error').text()).toBe('imports.errors.pattern');
  expect(view.find('pre').exists()).toBe(true); // An invalid request did not revoke the saved result.
  await view.get('textarea').setValue(corrected);
  expect(view.find('#pattern-error').exists()).toBe(false);
  get.mockResolvedValue(receipt({ state: 'received', analysisVersion: 2, preset: 'custom', pattern: corrected }));
  await button(view, 'imports.analyze').trigger('click'); await flushPromises();
  expect(analyze).toHaveBeenLastCalledWith('sample', 1, { encoding: '', preset: 'custom', pattern: corrected }, expect.any(AbortSignal));
  expect(view.find('pre').exists()).toBe(false);
  get.mockResolvedValue(receipt({ state: 'ready', analysisVersion: 2, preset: 'custom', pattern: corrected })); sample.mockResolvedValue({ ...preview(2), preset: 'custom', reviewReasons: [] });
  await button(view, 'imports.refresh').trigger('click'); await flushPromises();
  expect(button(view, 'imports.confirmAdd').attributes('disabled')).toBeUndefined();
  await view.findAll('select')[1]!.setValue('generated-sections');
  expect(view.find('textarea').exists()).toBe(false);
  get.mockResolvedValue(receipt({ state: 'received', analysisVersion: 3, preset: 'generated-sections' }));
  await button(view, 'imports.analyze').trigger('click'); await flushPromises();
  expect(analyze).toHaveBeenLastCalledWith('sample', 2, { encoding: '', preset: 'generated-sections', pattern: '' }, expect.any(AbortSignal));
});

it.each([
  ['txt_encoding_required', 'encodingRequired'],
  ['txt_invalid_encoding', 'invalidEncoding'],
  ['txt_unsupported_encoding', 'unsupportedEncoding'],
  ['txt_no_readable_text', 'noReadableText'],
  ['txt_non_text', 'nonText'],
  ['txt_section_limit', 'sectionLimit'],
  ['txt_storage_error', 'storage'],
  [undefined, 'generic'],
  ['/private/error', 'generic'],
])('shows safe analysis guidance for receipt code %s', async (errorCode, key) => {
  vi.spyOn(api, 'getTXTReceipt').mockResolvedValue({ ...receipt({ state: 'analysis_failed', hasError: true }), errorCode });
  const view = mount(ImportReviewView, { props: { receiptId: 'sample' }, global: { plugins: [createPinia(), await routerFor('/imports/sample')], mocks: { $t: (value: string) => value } } }); wrappers.push(view);
  await flushPromises();
  expect(view.text()).toContain(`imports.analysisErrors.${key}`);
  expect(view.text()).not.toContain('/private/error');
  expect(view.text()).not.toContain('imports.analysisHint');
});

it('keeps refresh for failed review loading, then removes the routine ready status row', async () => {
  vi.spyOn(api, 'getTXTReceipt').mockRejectedValueOnce(new Error('offline')).mockResolvedValue(receipt({ state: 'ready' }));
  vi.spyOn(api, 'previewTXT').mockResolvedValue(preview());
  const view = mount(ImportReviewView, { props: { receiptId: 'sample' }, global: { plugins: [createPinia(), await routerFor('/imports/sample')], mocks: { $t: (key: string) => key } } }); wrappers.push(view);
  await flushPromises();
  expect(view.find('[role="alert"]').exists()).toBe(true);
  await button(view, 'imports.refresh').trigger('click'); await flushPromises();
  expect(view.find('[role="alert"]').exists()).toBe(false);
  expect(button(view, 'imports.refresh')).toBeUndefined();
  expect(view.find('.import-review-status').exists()).toBe(false);
  expect(button(view, 'imports.confirmAdd').attributes('disabled')).toBeUndefined();
});

it('shows persisted imports even while their completed transfer remains in this tab', async () => {
  const item = receipt({ state: 'published', libraryId: 'sample' });
  vi.spyOn(api, 'listTXTReceipts').mockResolvedValue({ items: [item] });
  const pinia = createPinia();
  useImportQueue(pinia).entries.push({ key: 1, name: item.originalName, receiptId: item.id, receipt: item, libraryId: item.libraryId, state: 'added' });
  const view = mount(ImportReceiptsPanel, { global: { plugins: [pinia, await routerFor('/imports')], mocks: { $t: (key: string) => key } } }); wrappers.push(view);
  await flushPromises();
  expect(view.get('.import-list').text()).toContain(item.originalName);
  expect(view.get('.import-list a').attributes('href')).toBe('/books/sample/read');
});

it('reloads history when an import finishes during an in-flight history request', async () => {
  let resolve!: (page: Awaited<ReturnType<typeof api.listTXTReceipts>>) => void;
  const list = vi.spyOn(api, 'listTXTReceipts').mockReturnValueOnce(new Promise(done => { resolve = done; })).mockResolvedValue({ items: [receipt({ state: 'published', libraryId: 'sample' })] });
  const pinia = createPinia();
  const view = mount(ImportReceiptsPanel, { global: { plugins: [pinia, await routerFor('/imports')], mocks: { $t: (key: string) => key } } }); wrappers.push(view);
  useImportQueue(pinia).updateReceipt(receipt({ state: 'published', libraryId: 'sample' }));
  await flushPromises();
  resolve({ items: [receipt({ state: 'ready' })] }); await flushPromises();
  expect(list).toHaveBeenCalledTimes(2);
  expect(view.get('.import-list a').attributes('href')).toBe('/books/sample/read');
});
