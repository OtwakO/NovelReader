import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import { createPinia } from 'pinia';
import { afterEach, expect, it, vi } from 'vitest';
import * as api from '../../api/epub-imports';
import * as txt from '../../api/txt-imports';
import EPUBReviewView from './EPUBReviewView.vue';
import ImportReceiptsPanel from './ImportReceiptsPanel.vue';

const receipt = (changes: Partial<api.EPUBReceipt> = {}): api.EPUBReceipt => ({ id: 'epub', originalName: 'Novel.epub', state: 'acquired', generation: 1, preparationState: 'ready', imageMode: 'original', hasError: false, size: 1, createdAt: 0, updatedAt: 0, ...changes });
const preview = (changes: Partial<api.EPUBPreview> = {}): api.EPUBPreview => ({ generation: 1, title: 'From metadata', authors: ['Example author'], totalSections: 1, headings: [{ index: 0, title: 'Chapter', auxiliary: false }], hasMore: false, sample: '<script>Literal prose</script>', sampleTruncated: true, needsReview: true, diagnostics: ['image_unavailable'], images: { mode: 'original', derivativeCount: 0, derivativeBytes: 0 }, ...changes });
let view: VueWrapper;
afterEach(() => { view?.unmount(); vi.restoreAllMocks(); });
const options = () => ({ plugins: [createPinia()], mocks: { $t: (key: string) => key, $te: () => true }, stubs: { RouterLink: true } });
function button(key: string) { return view.findAll('button').find(item => item.text() === key)!; }

it('shows EPUB content warnings and literal samples, then reconciles an uncertain acceptance instead of replaying it', async () => {
  const status = vi.spyOn(api, 'getEPUBReceipt').mockResolvedValue(receipt());
  vi.spyOn(api, 'previewEPUB').mockResolvedValue(preview());
  const accept = vi.spyOn(api, 'acceptEPUB').mockRejectedValue(new Error('Lost response'));
  view = mount(EPUBReviewView, { props: { receiptId: 'epub' }, global: options() }); await flushPromises();
  expect(view.get('pre').text()).toBe('<script>Literal prose</script>'); expect(view.find('script').exists()).toBe(false);
  expect(view.text()).toContain('imports.epub.diagnostics.image_unavailable');
  expect(view.findComponent({ name: 'TXTInterpretationOptions' }).exists()).toBe(false);
  await view.get('form').trigger('submit'); await flushPromises();
  expect(accept).toHaveBeenCalledExactlyOnceWith('epub', 1, 'From metadata', 'Example author', expect.any(AbortSignal));
  expect(button('imports.confirmAdd').attributes('disabled')).toBeDefined();
  status.mockResolvedValue(receipt({ libraryId: 'epub' }));
  await button('imports.refresh').trigger('click'); await flushPromises();
  expect(view.find('form').exists()).toBe(false); expect(accept).toHaveBeenCalledOnce();
  expect(view.emitted('updated')?.at(-1)?.[0]).toMatchObject({ libraryId: 'epub' });
});

it('retries only the observed failed generation and waits for active preparation before discard', async () => {
  const status = vi.spyOn(api, 'getEPUBReceipt').mockResolvedValue(receipt({ preparationState: 'failed', hasError: true, errorCode: 'epub_preparation_failed' }));
  const retry = vi.spyOn(api, 'retryEPUB').mockResolvedValue({ receipt: receipt({ generation: 2, preparationState: 'queued' }) });
  vi.spyOn(api, 'previewEPUB').mockResolvedValue(preview({ generation: 2 }));
  const discard = vi.spyOn(api, 'discardEPUB').mockResolvedValue({ cleanupPending: false });
  view = mount(EPUBReviewView, { props: { receiptId: 'epub' }, global: options() }); await flushPromises();
  status.mockResolvedValue(receipt({ generation: 2, preparationState: 'preparing' }));
  await button('imports.epub.retry').trigger('click'); await flushPromises();
  expect(retry).toHaveBeenCalledExactlyOnceWith('epub', 1, expect.any(AbortSignal));
  expect(button('imports.discard').attributes('disabled')).toBeDefined(); expect(discard).not.toHaveBeenCalled();
  status.mockResolvedValue(receipt({ generation: 2 }));
  await button('imports.refresh').trigger('click'); await flushPromises();
  await button('imports.discard').trigger('click'); await button('imports.confirmDiscard').trigger('click'); await flushPromises();
  expect(discard).toHaveBeenCalledOnce(); expect(view.emitted('removed')).toEqual([['epub']]);
});

it('pages EPUB history independently and refuses bulk addition when its saved preview has content warnings', async () => {
  vi.spyOn(txt, 'listTXTReceipts').mockResolvedValue({ items: [] });
  const list = vi.spyOn(api, 'listEPUBReceipts').mockResolvedValue({ items: [receipt()], nextCursor: 'epub' });
  vi.spyOn(api, 'previewEPUB').mockResolvedValue(preview({ notices: ['epub_portable_encoder'] }));
  const accept = vi.spyOn(api, 'acceptEPUB');
  view = mount(ImportReceiptsPanel, { global: options() }); await flushPromises();
  await view.get('select').setValue('epub'); await flushPromises();
  expect(list).toHaveBeenCalledWith('', expect.any(AbortSignal));
  await view.get('input[type="checkbox"]').setValue(true);
  await button('imports.addSelected').trigger('click'); await flushPromises();
  expect(accept).not.toHaveBeenCalled(); expect(view.text()).toContain('imports.errors.reviewRequired');
  expect(view.text()).toContain('imports.epub.portableEncoder');
  await button('imports.next').trigger('click'); await flushPromises();
  expect(list).toHaveBeenLastCalledWith('epub', expect.any(AbortSignal));
  await button('imports.flow.checkBook').trigger('click'); expect(view.emitted('review')?.at(-1)).toEqual(['epub', 'epub']);
});
