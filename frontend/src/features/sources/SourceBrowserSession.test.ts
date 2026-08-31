import { mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import SourceBrowserSession from './SourceBrowserSession.vue';

const startSourceBrowser = vi.fn();
const closeSourceBrowser = vi.fn();

vi.mock('../../api/sources', async () => {
  const actual = await vi.importActual<typeof import('../../api/sources')>('../../api/sources');
  return {
    ...actual,
    startSourceBrowser: (...args: unknown[]) => startSourceBrowser(...args),
    getSourceBrowserFrame: vi.fn(),
    sendSourceBrowserInput: vi.fn(),
    closeSourceBrowser: (...args: unknown[]) => closeSourceBrowser(...args),
  };
});

describe('SourceBrowserSession', () => {
  beforeEach(() => {
    startSourceBrowser.mockReset().mockResolvedValue({
      sessionId: 'browser-1', image: 'frame', mediaType: 'image/jpeg',
      width: 430, height: 640, url: 'https://source.test', title: 'Source',
    });
    closeSourceBrowser.mockReset().mockResolvedValue({ closed: true });
  });

  it('sizes the remote viewport from the mounted screenshot panel', async () => {
    const bounds = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
      return this.classList.contains('viewport')
        ? ({ width: 430, height: 640 } as DOMRect)
        : ({ width: 0, height: 0 } as DOMRect);
    });

    const wrapper = mount(SourceBrowserSession, {
      props: { sourceId: 'source-a', browserRequestId: 'request-1' },
      attachTo: document.body,
    });

    await vi.waitFor(() => expect(startSourceBrowser).toHaveBeenCalledWith('source-a', 'request-1', 430, 640, expect.any(Number)));
    wrapper.unmount();
    bounds.mockRestore();
  });
});
