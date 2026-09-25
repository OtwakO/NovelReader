import { flushPromises } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getChapterContent, type ChapterContent } from '../../api/reader';
import { createChapterLoader } from './chapter-loader';
import { resetReaderRequests } from '../../api/transport';

vi.mock('../../api/reader', () => ({ getChapterContent: vi.fn() }));
const content: ChapterContent = { version: 1, contentRevision: 7, offlineCopy: false, document: { kind: 'prose', title: 'Chapter', blocks: [] } };
function deferred() {
  let resolve!: (value: ChapterContent) => void;
  const promise = new Promise<ChapterContent>(done => { resolve = done; });
  return { promise, resolve };
}

beforeEach(() => { resetReaderRequests(); vi.mocked(getChapterContent).mockReset().mockResolvedValue(content); });

describe('reading-session chapter loader', () => {
  it('shares prefetch with navigation, reuses recent content, and bounds retention', async () => {
    const loader = createChapterLoader('book', 7);
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
    const loader = createChapterLoader('book', 7);
    loader.prefetch(1);
    loader.prefetch(2);
    const foreground = loader.load(3);
    await flushPromises();
    expect(getChapterContent).toHaveBeenCalledTimes(1);
    first.resolve(content);
    await foreground;
    expect(vi.mocked(getChapterContent).mock.calls.map(call => call[1])).toEqual([1, 3]);
  });

  it('drains a replaced source binding and rejects late results without contaminating its replacement', async () => {
    const first = deferred();
    vi.mocked(getChapterContent).mockReturnValueOnce(first.promise);
    const old = createChapterLoader('book', 7);
    const pending = old.load(1);
    const rejected = expect(pending).rejects.toMatchObject({ name: 'AbortError' });
    await flushPromises();
    const drained = old.dispose();
    first.resolve(content);
    await drained;
    await rejected;
    const replacement = createChapterLoader('book', 7);
    await replacement.load(1);
    expect(getChapterContent).toHaveBeenCalledTimes(2);
  });

  it('rejects a different interpretation without admitting it to the cache', async () => {
    const loader = createChapterLoader('book', 7);
    vi.mocked(getChapterContent).mockResolvedValueOnce({ ...content, contentRevision: 8 });
    await expect(loader.load(1)).rejects.toThrow('Reading state changed');
    await expect(loader.load(1)).resolves.toEqual(content);
    expect(getChapterContent).toHaveBeenCalledTimes(2);
  });

  it('does not retain failures or outage fallback, and aborts on unmount', async () => {
    vi.mocked(getChapterContent).mockRejectedValueOnce(new Error('unavailable')).mockResolvedValueOnce({ ...content, offlineCopy: true });
    const loader = createChapterLoader('book', 7);
    await expect(loader.load(1)).rejects.toThrow('unavailable');
    await loader.load(1);
    await loader.load(1);
    expect(getChapterContent).toHaveBeenCalledTimes(3);
    await loader.dispose(true);
    expect(vi.mocked(getChapterContent).mock.calls[0]![3]!.aborted).toBe(true);
  });
});

it('does not reuse memory or pending content across a retired reader lifetime', async () => {
  const loader = createChapterLoader('book', 7);
  await loader.load(0);
  const delayed = deferred();
  vi.mocked(getChapterContent).mockReturnValueOnce(delayed.promise);
  const pending = loader.load(1);
  const rejected = expect(pending).rejects.toMatchObject({ name: 'AbortError' });
  await flushPromises();
  resetReaderRequests();
  delayed.resolve(content);
  await rejected;
  await expect(loader.load(0)).rejects.toMatchObject({ name: 'AbortError' });
  expect(getChapterContent).toHaveBeenCalledTimes(2);
});

it('expires reusable documents without retaining unavailable figures', async () => {
  const clock = vi.spyOn(performance, 'now').mockReturnValue(0);
  const wall = vi.spyOn(Date, 'now').mockReturnValue(0);
  try {
    vi.mocked(getChapterContent).mockResolvedValue({ ...content, freshForMs: 50 });
    const response = deferred();
    vi.mocked(getChapterContent).mockReturnValueOnce(response.promise);
    const loader = createChapterLoader('book', 7);
    const pending = loader.load(0);
    await flushPromises();
    clock.mockReturnValue(40);
    wall.mockReturnValue(40);
    response.resolve({ ...content, freshForMs: 50 });
    await pending;
    await loader.load(0);
    expect(getChapterContent).toHaveBeenCalledTimes(1);
    clock.mockReturnValue(50);
    await loader.load(0);
    expect(getChapterContent).toHaveBeenCalledTimes(2);
    wall.mockReturnValue(100); // Simulated sleep while performance.now pauses.
    await loader.load(0);
    expect(getChapterContent).toHaveBeenCalledTimes(3);
    vi.mocked(getChapterContent).mockResolvedValueOnce({ ...content, document: { ...content.document, blocks: [{ kind: 'image', resource: { href: '', unavailable: true } }] } });
    await loader.load(1);
    await loader.load(1);
    expect(getChapterContent).toHaveBeenCalledTimes(5);
    await loader.dispose();
  } finally { clock.mockRestore(); wall.mockRestore(); }
});

it('Refresh drains an ordinary request and bypasses both caches', async () => {
  const first = deferred();
  vi.mocked(getChapterContent).mockReturnValueOnce(first.promise);
  const loader = createChapterLoader('book', 7);
  const ordinary = loader.load(0);
  await flushPromises();
  const refreshed = loader.refresh(0);
  expect(loader.refresh(0)).toBe(refreshed);
  const follower = loader.load(0);
  expect(follower).toBe(refreshed);
  expect(getChapterContent).toHaveBeenCalledTimes(1);
  const replacement = { ...content, document: { ...content.document, title: 'New' } };
  vi.mocked(getChapterContent).mockResolvedValueOnce(replacement);
  first.resolve(content);
  await ordinary;
  await expect(refreshed).resolves.toEqual(replacement);
  expect(getChapterContent).toHaveBeenLastCalledWith('book', 0, 7, expect.any(AbortSignal), true);
  await expect(loader.load(0)).resolves.toEqual(replacement);
  expect(getChapterContent).toHaveBeenCalledTimes(2);
  await loader.dispose();
});

it('does not retain an earlier pending copy when Refresh fails', async () => {
  const first = deferred();
  vi.mocked(getChapterContent).mockReturnValueOnce(first.promise).mockRejectedValueOnce(new Error('offline'));
  const loader = createChapterLoader('book', 7);
  const ordinary = loader.load(0);
  const failure = expect(loader.refresh(0)).rejects.toThrow('offline');
  first.resolve(content);
  await ordinary;
  await failure;
  await loader.load(0);
  expect(getChapterContent).toHaveBeenCalledTimes(3);
  await loader.dispose();
});
