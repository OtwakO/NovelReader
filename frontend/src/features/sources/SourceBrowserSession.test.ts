import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import SourceBrowserSession from './SourceBrowserSession.vue';

const startSourceBrowser = vi.fn();
const closeSourceBrowser = vi.fn();
const getSourceBrowserFrame = vi.fn();
const frame = { sessionId: 'browser-1', image: 'frame', mediaType: 'image/jpeg', width: 430, height: 640, url: 'https://source.test', title: 'Source' };
const mountBrowser = () => mount(SourceBrowserSession, {
  props: { sourceId: 'source-a', browserRequestId: 'request-1' },
  attachTo: document.body,
  global: { mocks: { $t: (key: string) => key } },
});

vi.mock('../../api/sources', async () => {
  const actual = await vi.importActual<typeof import('../../api/sources')>('../../api/sources');
  return {
    ...actual,
    startSourceBrowser: (...args: unknown[]) => startSourceBrowser(...args),
    getSourceBrowserFrame: (...args: unknown[]) => getSourceBrowserFrame(...args),
    sendSourceBrowserInput: vi.fn(),
    closeSourceBrowser: (...args: unknown[]) => closeSourceBrowser(...args),
  };
});

describe('SourceBrowserSession', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    startSourceBrowser.mockReset().mockResolvedValue(frame);
    getSourceBrowserFrame.mockReset().mockResolvedValue(frame);
    closeSourceBrowser.mockReset().mockResolvedValue({ closed: true });
  });

  afterEach(() => { vi.clearAllTimers(); vi.useRealTimers(); vi.restoreAllMocks(); });

  it('sizes the remote viewport from the mounted screenshot panel', async () => {
    const bounds = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
      return this.classList.contains('viewport')
        ? ({ width: 430, height: 640 } as DOMRect)
        : ({ width: 0, height: 0 } as DOMRect);
    });

    const wrapper = mountBrowser();
    await flushPromises();
    expect(startSourceBrowser).toHaveBeenCalledWith('source-a', 'request-1', 430, 640, expect.any(Number));
    wrapper.unmount();
    bounds.mockRestore();
  });

  it.each(['start', 'refresh'])('releases the session without restarting polling when %s resolves after unmount', async (pending) => {
    let resolve!: (value: typeof frame) => void;
    const response = new Promise<typeof frame>((done) => { resolve = done; });
    (pending === 'start' ? startSourceBrowser : getSourceBrowserFrame).mockReturnValue(response);
    const wrapper = mountBrowser();
    await flushPromises();
    if (pending === 'refresh') await vi.advanceTimersByTimeAsync(1200);
    wrapper.unmount();
    resolve(frame);
    await flushPromises();
    expect(closeSourceBrowser).toHaveBeenCalledExactlyOnceWith('source-a', frame.sessionId, false);
    expect(vi.getTimerCount()).toBe(0);
  });
});
