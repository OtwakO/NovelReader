import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getProgressVersion, queueProgressWrite, resetProgressWriter, setProgressVersion, waitForProgressWrites } from './progress-writer';

const saveProgress = vi.fn();
vi.mock('../../api/reader', () => ({ saveProgress: (...args: unknown[]) => saveProgress(...args) }));

describe('progress writer', () => {
  beforeEach(() => { resetProgressWriter(); saveProgress.mockReset(); });
  it('serializes writes per book and advances state versions', async () => {
    let release!: () => void;
    saveProgress.mockImplementationOnce(() => new Promise((resolve) => { release = () => resolve({ status: 'ok', stateVersion: 2 }); })).mockResolvedValueOnce({ status: 'ok', stateVersion: 3 });
    setProgressVersion('book', 1);
    const first = queueProgressWrite('book', { sourceUrl: 'source', chapterIndex: 1, position: .4 });
    const second = queueProgressWrite('book', { sourceUrl: 'source', chapterIndex: 2, position: .1 });
    await vi.waitFor(() => expect(saveProgress).toHaveBeenCalledTimes(1)); release(); await Promise.all([first, second]);
    expect(saveProgress.mock.calls[1]).toEqual(['book', 'source', 2, 2, .1]); expect(getProgressVersion('book')).toBe(3);
  });
  it('allows source switching to wait for pending progress', async () => {
    let release!: () => void;
    saveProgress.mockImplementation(() => new Promise((resolve) => { release = () => resolve({ status: 'ok', stateVersion: 2 }); })); setProgressVersion('book', 1);
    void queueProgressWrite('book', { sourceUrl: 'source', chapterIndex: 1, position: .8 }); let complete = false; void waitForProgressWrites('book').then(() => { complete = true; }); await vi.waitFor(() => expect(saveProgress).toHaveBeenCalledTimes(1)); expect(complete).toBe(false); release(); await waitForProgressWrites('book'); expect(complete).toBe(true);
  });
});
