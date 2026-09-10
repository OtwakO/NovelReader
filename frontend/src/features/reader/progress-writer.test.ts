import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getProgressVersion, queueReadingStateWrite, queueProgressWrite, resetProgressWriter, setProgressVersion, waitForProgressWrites } from './progress-writer';

const saveProgress = vi.fn();
vi.mock('../../api/reader', () => ({ saveProgress: (...args: unknown[]) => saveProgress(...args) }));

describe('progress writer', () => {
  beforeEach(() => { resetProgressWriter(); saveProgress.mockReset(); });
  it('serializes writes per book and advances state versions', async () => {
    let release!: () => void;
    saveProgress.mockImplementationOnce(() => new Promise((resolve) => { release = () => resolve({ status: 'ok', stateVersion: 2 }); })).mockResolvedValueOnce({ status: 'ok', stateVersion: 3 });
    setProgressVersion('book', 1);
    const first = queueProgressWrite('book', { contentRevision: 7, chapterIndex: 1, position: .4 });
    const second = queueProgressWrite('book', { contentRevision: 7, chapterIndex: 2, position: .1 });
    await vi.waitFor(() => expect(saveProgress).toHaveBeenCalledTimes(1)); release(); await Promise.all([first, second]);
    expect(saveProgress.mock.calls[1]).toEqual(['book', 7, 2, 2, .1]); expect(getProgressVersion('book')).toBe(3);
  });
  it('drops queued writes and late versions from a cleared reader lifetime', async () => {
    let resolve!: (value: { stateVersion: number }) => void;
    saveProgress.mockReturnValue(new Promise(done => { resolve = done; }));
    setProgressVersion('book', 1);
    const first = queueProgressWrite('book', { contentRevision: 7, chapterIndex: 1, position: .2 });
    const second = queueProgressWrite('book', { contentRevision: 7, chapterIndex: 2, position: .3 });
    await vi.waitFor(() => expect(saveProgress).toHaveBeenCalledOnce());
    resetProgressWriter();
    setProgressVersion('book', 10);
    resolve({ stateVersion: 2 });
    await Promise.all([first, second]);
    expect(saveProgress).toHaveBeenCalledOnce();
    expect(getProgressVersion('book')).toBe(10);
  });
  it('orders bookmark mutations between progress writes', async () => {
    setProgressVersion('book', 0);
    saveProgress.mockResolvedValueOnce({ stateVersion: 1 }).mockResolvedValueOnce({ stateVersion: 3 });
    const bookmark = vi.fn().mockResolvedValue({ stateVersion: 2 });
    const first = queueProgressWrite('book', { contentRevision: 7, chapterIndex: 1, position: .2 });
    const mark = queueReadingStateWrite('book', bookmark);
    const last = queueProgressWrite('book', { contentRevision: 7, chapterIndex: 1, position: .3 });
    await Promise.all([first, mark, last]);
    expect(bookmark).toHaveBeenCalledWith(1);
    expect(saveProgress.mock.calls[1]).toEqual(['book', 7, 2, 1, .3]);
    expect(getProgressVersion('book')).toBe(3);
  });
  it('allows source switching to wait for pending progress', async () => {
    let release!: () => void;
    saveProgress.mockImplementation(() => new Promise((resolve) => { release = () => resolve({ status: 'ok', stateVersion: 2 }); })); setProgressVersion('book', 1);
    void queueProgressWrite('book', { contentRevision: 7, chapterIndex: 1, position: .8 }); let complete = false; void waitForProgressWrites('book').then(() => { complete = true; }); await vi.waitFor(() => expect(saveProgress).toHaveBeenCalledTimes(1)); expect(complete).toBe(false); release(); await waitForProgressWrites('book'); expect(complete).toBe(true);
  });
});
