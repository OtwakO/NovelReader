import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getChapterContent, type ChapterContent } from '../../api/reader';
import { createChapterLoader } from './chapter-loader';

vi.mock('../../api/reader', () => ({ getChapterContent: vi.fn() }));
const content: ChapterContent = { version: 1, offlineCopy: false, document: { kind: 'prose', title: 'Chapter', blocks: [] } };
function deferred() {
  let resolve!: (value: ChapterContent) => void;
  const promise = new Promise<ChapterContent>(done => { resolve = done; });
  return { promise, resolve };
}

beforeEach(() => { vi.mocked(getChapterContent).mockReset().mockResolvedValue(content); });

describe('reading-session chapter loader', () => {
  it('shares prefetch with navigation, reuses recent content, and bounds retention', async () => {
    const loader = createChapterLoader('book');
    loader.prefetch(1);
    await loader.load(1);
    await loader.load(1);
    expect(getChapterContent).toHaveBeenCalledTimes(1);
    for (let index = 2; index <= 6; index++) await loader.load(index);
    await loader.load(1);
    expect(getChapterContent).toHaveBeenCalledTimes(7);
  });

  it('serializes chapter execution and limits speculative work to one request', async () => {
    const first = deferred();
    vi.mocked(getChapterContent).mockReturnValueOnce(first.promise);
    const loader = createChapterLoader('book');
    loader.prefetch(1);
    loader.prefetch(2);
    const foreground = loader.load(3);
    await Promise.resolve();
    expect(getChapterContent).toHaveBeenCalledTimes(1);
    first.resolve(content);
    await foreground;
    expect(vi.mocked(getChapterContent).mock.calls.map(call => call[1])).toEqual([1, 3]);
  });

  it('drains a replaced source binding and rejects late results without contaminating its replacement', async () => {
    const first = deferred();
    vi.mocked(getChapterContent).mockReturnValueOnce(first.promise);
    const old = createChapterLoader('book');
    const pending = old.load(1);
    const rejected = expect(pending).rejects.toMatchObject({ name: 'AbortError' });
    await Promise.resolve();
    const drained = old.dispose();
    first.resolve(content);
    await drained;
    await rejected;
    const replacement = createChapterLoader('book');
    await replacement.load(1);
    expect(getChapterContent).toHaveBeenCalledTimes(2);
  });

  it('does not retain failures or outage fallback, and aborts on unmount', async () => {
    vi.mocked(getChapterContent).mockRejectedValueOnce(new Error('unavailable')).mockResolvedValueOnce({ ...content, offlineCopy: true });
    const loader = createChapterLoader('book');
    await expect(loader.load(1)).rejects.toThrow('unavailable');
    await loader.load(1);
    await loader.load(1);
    expect(getChapterContent).toHaveBeenCalledTimes(3);
    await loader.dispose(true);
    expect(vi.mocked(getChapterContent).mock.calls[0]![2]!.aborted).toBe(true);
  });
});
