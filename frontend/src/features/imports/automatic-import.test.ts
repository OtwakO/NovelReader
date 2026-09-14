import { createPinia, setActivePinia } from 'pinia';
import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import * as api from '../../api/txt-imports';
import { useImportQueue } from './import-queue';
import ImportQueuePanel from './ImportQueuePanel.vue';

const receipt = (id: string, changes: Partial<api.TXTReceipt> = {}): api.TXTReceipt => ({ id, originalName: `${id}.txt`, state: 'received', size: 1, createdAt: 0, updatedAt: 0, analysisVersion: 1, encoding: '', preset: '', hasError: false, ...changes });
beforeEach(() => {
  vi.useFakeTimers(); setActivePinia(createPinia()); let sequence = 0;
  vi.spyOn(api, 'requestTXTAdmission').mockImplementation(async () => ({ id: String(++sequence), state: 'granted', expiresAt: '', maxInputBytes: 1000 }));
  vi.spyOn(api, 'acquireTXT').mockImplementation(async id => ({ receipt: receipt(id) }));
});
afterEach(async () => { useImportQueue().resetReaderState(); await flushPromises(); vi.useRealTimers(); vi.restoreAllMocks(); });

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
