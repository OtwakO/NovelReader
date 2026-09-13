import { createPinia, setActivePinia } from 'pinia';
import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import * as api from '../../api/txt-imports';
import { resetReaderState } from '../../app/reader-state';
import { useImportQueue } from './import-queue';
import ImportQueuePanel from './ImportQueuePanel.vue';

function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(done => { resolve = done; }); return { promise, resolve }; }
const acquired = (id: string): api.TXTAcquisition => ({ receipt: { id, state: 'received', originalName: `${id}.txt`, size: 1, createdAt: 0, updatedAt: 0, analysisVersion: 0, encoding: '', preset: '', hasError: false } });
let serial = 0;
beforeEach(() => {
  setActivePinia(createPinia()); serial = 0;
  vi.spyOn(api, 'requestTXTAdmission').mockImplementation(async () => ({ id: String(++serial), state: 'granted', expiresAt: '', maxInputBytes: 1000 }));
});
afterEach(() => { useImportQueue().resetReaderState(); vi.useRealTimers(); vi.restoreAllMocks(); });

it('continues across view unmount, releases file references, and pauses only at a file boundary', async () => {
  const first = deferred<api.TXTAcquisition>();
  const transfer = vi.spyOn(api, 'acquireTXT').mockReturnValueOnce(first.promise).mockResolvedValue(acquired('2'));
  const queue = useImportQueue();
  const files = [new File(['a'], 'a.txt'), new File(['b'], 'b.txt')];
  const view = mount(ImportQueuePanel, { global: { mocks: { $t: (key: string) => key }, stubs: { RouterLink: true } } });
  queue.enqueue(files);
  await flushPromises();
  expect(transfer).toHaveBeenCalledTimes(1);
  expect(transfer.mock.calls[0]?.[1]).toBe(files[0]);
  view.unmount(); queue.pause(); first.resolve(acquired('1'));
  await flushPromises();
  expect(queue.entries[0]?.input).toBeUndefined();
  expect(queue.entries[1]?.state).toBe('queued');
  expect(transfer).toHaveBeenCalledTimes(1);
  queue.resume(); await flushPromises();
  expect(transfer).toHaveBeenCalledTimes(2);
  expect(queue.entries.every(item => item.state === 'acquired' && !item.input)).toBe(true);
});

it('waits on reader admission without sending bytes or taking over another tab transfer', async () => {
  vi.useFakeTimers();
  vi.mocked(api.requestTXTAdmission).mockResolvedValueOnce({ id: 'other', state: 'transferring', expiresAt: '', maxInputBytes: 1000 })
    .mockResolvedValueOnce({ id: 'mine', state: 'waiting', expiresAt: '', maxInputBytes: 1000 });
  vi.spyOn(api, 'getTXTAdmission').mockResolvedValue({ id: 'mine', state: 'granted', expiresAt: '', maxInputBytes: 1000 });
  const transfer = vi.spyOn(api, 'acquireTXT').mockResolvedValue(acquired('mine'));
  useImportQueue().enqueue(['inbox.txt']);
  await vi.advanceTimersByTimeAsync(0); expect(transfer).not.toHaveBeenCalled();
  await vi.advanceTimersByTimeAsync(5000); expect(transfer).not.toHaveBeenCalled();
  await vi.advanceTimersByTimeAsync(5000);
  expect(transfer).toHaveBeenCalledExactlyOnceWith('mine', 'inbox.txt', expect.any(AbortSignal));
});

it('pauses on a lost acquisition response without replaying it when the remaining queue resumes', async () => {
  const transfer = vi.spyOn(api, 'acquireTXT').mockRejectedValueOnce(new Error('Response lost')).mockResolvedValue(acquired('2'));
  const queue = useImportQueue(); queue.enqueue(['uncertain.txt', 'next.txt']);
  await flushPromises();
  expect(transfer).toHaveBeenCalledTimes(1);
  expect(queue.paused).toBe(true);
  expect(queue.entries[0]).toMatchObject({ state: 'attention', receiptId: '1', input: undefined });
  expect(queue.entries[1]?.state).toBe('queued');
  queue.resume(); await flushPromises(); expect(transfer).toHaveBeenCalledTimes(2);
  expect(queue.entries[1]?.state).toBe('acquired');
});

it('reader reset aborts work and prevents late completion from replacing the new reader queue', async () => {
  const pinia = createPinia(); setActivePinia(pinia);
  const old = deferred<api.TXTAcquisition>();
  const transfer = vi.spyOn(api, 'acquireTXT').mockReturnValueOnce(old.promise).mockResolvedValue(acquired('new'));
  const queue = useImportQueue(); queue.enqueue(['old.txt', 'never-start.txt']); await flushPromises();
  const signal = transfer.mock.calls[0]![2];
  resetReaderState(pinia);
  expect(signal.aborted).toBe(true);
  queue.enqueue(['new.txt']); await flushPromises(); old.resolve(acquired('old')); await flushPromises();
  expect(queue.entries.map(item => item.name)).toEqual(['new.txt']);
  expect(queue.entries[0]?.receiptId).toBe('new');
  expect(transfer).toHaveBeenCalledTimes(2);
});

it('keeps a representative batch as lightweight references with a bounded rendered page', async () => {
  const transfer = vi.spyOn(api, 'acquireTXT');
  const queue = useImportQueue(); queue.pause();
  const files = Array.from({ length: 250 }, (_, index) => new File(['synthetic'], `sample-${index}.txt`));
  queue.enqueue(files);
  const view = mount(ImportQueuePanel, { global: { mocks: { $t: (key: string) => key }, stubs: { RouterLink: true } } });
  expect(view.findAll('.import-list > li')).toHaveLength(25);
  expect(transfer).not.toHaveBeenCalled();
  expect(queue.entries[249]?.input).toBe(files[249]);
  view.unmount();
});

it('preserves unsent selections when admission is unavailable', async () => {
  vi.mocked(api.requestTXTAdmission).mockRejectedValueOnce(new Error('Offline'));
  const transfer = vi.spyOn(api, 'acquireTXT').mockResolvedValue(acquired('received'));
  const file = new File(['synthetic'], 'keep.txt'); const queue = useImportQueue();
  queue.enqueue([file]); await flushPromises();
  expect(queue.paused).toBe(true); expect(queue.entries[0]?.input).toBe(file);
  expect(transfer).not.toHaveBeenCalled();
  queue.resume(); await flushPromises();
  expect(transfer).toHaveBeenCalledOnce(); expect(queue.entries[0]?.state).toBe('acquired');
});
