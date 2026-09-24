import { createPinia, setActivePinia } from 'pinia';
import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import * as api from '../../api/txt-imports';
import * as admission from '../../api/file-imports';
import * as epub from '../../api/epub-imports';
import { useImportQueue } from './import-queue';
import { useImportPreferences } from './import-preferences';
import ImportQueuePanel from './ImportQueuePanel.vue';

const receipt = (id: string, changes: Partial<api.TXTReceipt> = {}): api.TXTReceipt => ({ id, originalName: `${id}.txt`, state: 'received', size: 1, createdAt: 0, updatedAt: 0, analysisVersion: 1, encoding: '', preset: '', hasError: false, ...changes });
beforeEach(() => {
  vi.useFakeTimers(); localStorage.clear(); setActivePinia(createPinia()); useImportPreferences().reviewBeforeAdding = false; let sequence = 0;
  vi.spyOn(admission, 'requestImportAdmission').mockImplementation(async () => ({ id: String(++sequence), state: 'granted', expiresAt: '', limits: { txt: 1000, epub: 1000 } }));
  vi.spyOn(api, 'acquireTXT').mockImplementation(async id => ({ receipt: receipt(id) }));
});
afterEach(async () => { useImportQueue().resetReaderState(); await flushPromises(); vi.useRealTimers(); vi.restoreAllMocks(); localStorage.clear(); });

it('adds a newly selected warning-free book after navigation without a review-page action', async () => {
  let ready!: (value: api.TXTReceipt) => void;
  vi.spyOn(api, 'getTXTReceipt').mockReturnValue(new Promise(resolve => { ready = resolve; }));
  const view = mount(ImportQueuePanel, { global: { mocks: { $t: (key: string) => key }, stubs: { RouterLink: true } } });
  const accept = vi.spyOn(api, 'acceptTXT').mockResolvedValue({ libraryId: 'book' });
  const queue = useImportQueue(); queue.enqueue([new File(['chapter'], 'book.txt')]);
  await flushPromises(); view.unmount(); ready(receipt('1', { state: 'ready' })); await flushPromises();
  expect(accept).toHaveBeenCalledExactlyOnceWith('1', 1, '1', '', expect.any(AbortSignal));
  expect(queue.entries[0]).toMatchObject({ state: 'added', libraryId: 'book', input: undefined });
  expect(queue.libraryRevision).toBe(1);
  expect(queue.outstanding).toBe(0);
});

it('never approves review warnings, failed results or a newer interpretation', async () => {
  vi.spyOn(api, 'getTXTReceipt').mockImplementation(async id => receipt(id, id === '1' ? { state: 'needs_review' } : id === '2' ? { state: 'analysis_failed', hasError: true } : { state: 'ready', analysisVersion: 2 }));
  const accept = vi.spyOn(api, 'acceptTXT');
  const queue = useImportQueue(); queue.enqueue(['a.txt', 'b.txt', 'c.txt']);
  await vi.advanceTimersByTimeAsync(5000);
  expect(accept).not.toHaveBeenCalled();
  expect(queue.entries.every(entry => entry.state === 'attention')).toBe(true);
});

it('does not replay uncertain addition; explicit status reconciliation updates the shelf', async () => {
  vi.spyOn(api, 'getTXTReceipt').mockResolvedValue(receipt('1', { state: 'ready' }));
  const accept = vi.spyOn(api, 'acceptTXT').mockRejectedValue(new Error('Response lost'));
  const queue = useImportQueue(); queue.enqueue(['book.txt']);
  await vi.advanceTimersByTimeAsync(5000);
  expect(accept).toHaveBeenCalledTimes(1);
  expect(queue.entries[0]?.state).toBe('attention');
  queue.updateReceipt(receipt('1', { state: 'published', libraryId: 'book' }));
  expect(queue.entries[0]).toMatchObject({ state: 'added', libraryId: 'book', error: undefined });
  expect(queue.libraryRevision).toBe(1);
});

it('reader reset cancels automatic admission before a late status response', async () => {
  let resolve!: (value: api.TXTReceipt) => void;
  const status = vi.spyOn(api, 'getTXTReceipt').mockReturnValue(new Promise(done => { resolve = done; }));
  const accept = vi.spyOn(api, 'acceptTXT');
  const queue = useImportQueue(); queue.enqueue(['book.txt']); await flushPromises();
  expect(status).toHaveBeenCalledOnce();
  queue.resetReaderState();
  expect(status.mock.calls[0]![1].aborted).toBe(true);
  resolve(receipt('1', { state: 'ready' })); await flushPromises();
  expect(accept).not.toHaveBeenCalled(); expect(queue.entries).toEqual([]);
});

it('snapshots review preference for each queued file without auto-approving earlier selections', async () => {
  vi.spyOn(api, 'getTXTReceipt').mockImplementation(async id => receipt(id, { state: 'ready' }));
  const accept = vi.spyOn(api, 'acceptTXT').mockResolvedValue({ libraryId: 'automatic' });
  const preferences = useImportPreferences(); preferences.reviewBeforeAdding = true;
  const queue = useImportQueue(); queue.pause();
  queue.enqueue(['review.txt']);
  preferences.reviewBeforeAdding = false;
  queue.enqueue(['automatic.txt']); queue.resume();
  await vi.advanceTimersByTimeAsync(5000);
  expect(queue.entries[0]?.state).toBe('review');
  expect(queue.entries[1]?.state).toBe('added');
  expect(accept).toHaveBeenCalledExactlyOnceWith('2', 1, '2', '', expect.any(AbortSignal));
  expect(queue.outstanding).toBe(0);
});

it('shares one mixed queue, snapshots EPUB image choices and shows fallback notices without forcing review', async () => {
  const make = (id: string, imageMode: epub.EPUBImageMode): epub.EPUBReceipt => ({ id, originalName: `${id}.epub`, state: 'acquired', size: 1, createdAt: 0, updatedAt: 0, generation: 1, preparationState: 'ready', imageMode, hasError: false });
  const upload = vi.spyOn(epub, 'acquireEPUB').mockImplementation(async (id, _file, mode) => ({ receipt: make(id, mode) }));
  vi.spyOn(epub, 'getEPUBReceipt').mockImplementation(async id => make(id, id === '2' ? 'optimized' : 'original'));
  vi.spyOn(epub, 'previewEPUB').mockImplementation(async id => ({ generation: 1, title: `Title ${id}`, authors: ['Author'], totalSections: 1, headings: [], hasMore: false, sample: '', sampleTruncated: false, needsReview: false, diagnostics: [], notices: id === '2' ? ['epub_portable_encoder'] : [], images: { mode: id === '2' ? 'optimized' : 'original', derivativeCount: 0, derivativeBytes: 0 } }));
  vi.spyOn(api, 'getTXTReceipt').mockImplementation(async id => receipt(id, { state: 'ready' }));
  vi.spyOn(api, 'acceptTXT').mockResolvedValue({ libraryId: 'txt' });
  const accept = vi.spyOn(epub, 'acceptEPUB').mockImplementation(async id => ({ libraryId: id }));
  const queue = useImportQueue(); queue.pause();
  const files = [new File(['text'], 'text.txt'), new File(['image'], 'image.epub')];
  queue.enqueue(files, 'optimized'); queue.enqueue([new File(['plain'], 'plain.epub')]);
  queue.resume(); await vi.advanceTimersByTimeAsync(5000);
  expect(upload.mock.calls.map(call => [call[0], call[2]])).toEqual([['2', 'optimized'], ['3', 'original']]);
  expect(upload.mock.calls[0]![1]).toBe(files[1]);
  expect(accept).toHaveBeenCalledWith('2', 1, 'Title 2', 'Author', expect.any(AbortSignal));
  expect(queue.entries.every(entry => entry.state === 'added' && !entry.input)).toBe(true);
  const view = mount(ImportQueuePanel, { global: { mocks: { $t: (key: string) => key }, stubs: { RouterLink: true } } });
  expect(view.text()).toContain('imports.epub.portableEncoder'); view.unmount();
});

it('does not automatically approve EPUB content loss or a replaced preparation generation', async () => {
  const make = (id: string, generation = 1): epub.EPUBReceipt => ({ id, originalName: `${id}.epub`, state: 'acquired', size: 1, createdAt: 0, updatedAt: 0, generation, preparationState: 'ready', imageMode: 'original', hasError: false });
  vi.spyOn(epub, 'acquireEPUB').mockImplementation(async id => ({ receipt: make(id) }));
  vi.spyOn(epub, 'getEPUBReceipt').mockImplementation(async id => make(id, id === '2' ? 2 : 1));
  const preview = vi.spyOn(epub, 'previewEPUB').mockResolvedValue({ generation: 1, title: '', authors: [], totalSections: 1, headings: [], hasMore: false, sample: '', sampleTruncated: false, needsReview: true, diagnostics: ['image_unavailable'], images: { mode: 'original', derivativeCount: 0, derivativeBytes: 0 } });
  const accept = vi.spyOn(epub, 'acceptEPUB');
  const queue = useImportQueue(); queue.enqueue([new File(['a'], 'warning.epub'), new File(['b'], 'changed.epub')]);
  await vi.advanceTimersByTimeAsync(5000);
  expect(queue.entries.every(entry => entry.state === 'attention')).toBe(true);
  expect(preview).toHaveBeenCalledExactlyOnceWith('1', 1, 0, expect.any(AbortSignal));
  expect(accept).not.toHaveBeenCalled();
});
