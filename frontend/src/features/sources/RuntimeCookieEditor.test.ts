import { mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import RuntimeCookieEditor from './RuntimeCookieEditor.vue';

const getSourceRuntimeCookies = vi.fn();
const revealSourceRuntimeCookies = vi.fn();
const replaceSourceRuntimeCookies = vi.fn();

vi.mock('../../api/sources', async () => {
  const actual = await vi.importActual<typeof import('../../api/sources')>('../../api/sources');
  return {
    ...actual,
    getSourceRuntimeCookies: (...args: unknown[]) => getSourceRuntimeCookies(...args),
    revealSourceRuntimeCookies: (...args: unknown[]) => revealSourceRuntimeCookies(...args),
    replaceSourceRuntimeCookies: (...args: unknown[]) => replaceSourceRuntimeCookies(...args),
  };
});

function mountEditor() {
  return mount(RuntimeCookieEditor, {
    props: { sourceId: 'source-a' },
    global: { mocks: { $t: (key: string) => key } },
  });
}

describe('RuntimeCookieEditor', () => {
  beforeEach(() => {
    getSourceRuntimeCookies.mockReset().mockResolvedValue({
      cookies: [{ scope: 'https://reader.example/', names: ['device', 'token'] }],
    });
    revealSourceRuntimeCookies.mockReset().mockResolvedValue({
      cookies: [{ scope: 'https://reader.example/', header: 'device=stable; token=secret' }],
    });
    replaceSourceRuntimeCookies.mockReset().mockResolvedValue({
      cookies: [{ scope: 'https://reader.example/', names: ['device', 'token'] }],
    });
  });

  it('loads only masked metadata before explicit password-protected reveal', async () => {
    const wrapper = mountEditor();
    await vi.waitFor(() => expect(getSourceRuntimeCookies).toHaveBeenCalledWith('source-a'));

    expect(wrapper.text()).toContain('device');
    expect(wrapper.text()).toContain('token');
    expect(wrapper.text()).not.toContain('stable');
    expect(wrapper.text()).not.toContain('secret');

    await wrapper.get('input[type="password"]').setValue('current-password');
    await wrapper.get('[data-action="reveal"]').trigger('click');

    await vi.waitFor(() => expect(revealSourceRuntimeCookies).toHaveBeenCalledWith('source-a', 'current-password'));
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('device=stable; token=secret');
    expect((wrapper.get('input[type="password"]').element as HTMLInputElement).value).toBe('');
  });

  it('requires a fresh password to save and returns to masked metadata', async () => {
    const wrapper = mountEditor();
    await vi.waitFor(() => expect(getSourceRuntimeCookies).toHaveBeenCalled());
    await wrapper.get('input[type="password"]').setValue('reveal-password');
    await wrapper.get('[data-action="reveal"]').trigger('click');
    await vi.waitFor(() => expect(wrapper.find('textarea').exists()).toBe(true));

    await wrapper.get('textarea').setValue('device=stable; token=updated');
    await wrapper.get('input[type="password"]').setValue('save-password');
    await wrapper.get('[data-action="save"]').trigger('click');

    await vi.waitFor(() => expect(replaceSourceRuntimeCookies).toHaveBeenCalledWith('source-a', 'save-password', [
      { scope: 'https://reader.example/', header: 'device=stable; token=updated' },
    ]));
    expect(wrapper.find('textarea').exists()).toBe(false);
    expect(wrapper.text()).toContain('device');
    expect(wrapper.text()).not.toContain('updated');
    expect((wrapper.get('input[type="password"]').element as HTMLInputElement).value).toBe('');
  });
});
