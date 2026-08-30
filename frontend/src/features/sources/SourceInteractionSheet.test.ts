import { mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import SourceInteractionSheet from './SourceInteractionSheet.vue';

const getSourceInteraction = vi.fn();
const runSourceInteractionAction = vi.fn();
vi.mock('../../api/sources', async () => {
  const actual = await vi.importActual<typeof import('../../api/sources')>('../../api/sources');
  return { ...actual, getSourceInteraction: (...args: unknown[]) => getSourceInteraction(...args), runSourceInteractionAction: (...args: unknown[]) => runSourceInteractionAction(...args), resetSourceInteraction: vi.fn() };
});

const view = {
  sourceId: 'source-a', title: 'Source', revision: 'revision', controls: [
    { id: 'control-0', type: 'password', label: 'Password', value: 'secret' },
    { id: 'control-1', type: 'select', label: 'Server', value: 'A', options: ['A', 'B'], actionId: 'action-1' },
    { id: 'control-2', type: 'toggle', label: 'Enabled', value: '0', options: ['0', '1'], actionId: 'action-2' },
  ],
};

function mountSheet() {
  return mount(SourceInteractionSheet, {
    attachTo: document.body,
    props: { source: { sourceId: 'source-a', bookSourceUrl: 'https://source.test', bookSourceName: 'Source', enabled: true, enabledExplore: true } },
    global: { mocks: { $t: (key: string) => key, $router: { push: vi.fn() } } },
  });
}

describe('SourceInteractionSheet', () => {
  beforeEach(() => { getSourceInteraction.mockReset().mockResolvedValue(view); runSourceInteractionAction.mockReset().mockResolvedValue({ view, effects: [] }); });
  it('masks passwords and executes select and toggle actions with current values', async () => {
    const wrapper = mountSheet(); await vi.waitFor(() => expect(wrapper.find('input[type="password"]').exists()).toBe(true));
    expect((wrapper.get('input[type="password"]').element as HTMLInputElement).value).toBe('secret');
    await wrapper.get('select').setValue('B');
    expect(runSourceInteractionAction).toHaveBeenCalledWith('source-a', 'revision', 'action-1', expect.objectContaining({ Server: 'B' }));
    await wrapper.get('input[type="checkbox"]').setValue(true);
    expect(runSourceInteractionAction).toHaveBeenLastCalledWith('source-a', 'revision', 'action-2', expect.objectContaining({ Enabled: '1' }));
    wrapper.unmount();
  });
  it('shows action feedback before the long control list', async () => {
    runSourceInteractionAction.mockResolvedValueOnce({ view, effects: [{ type: 'notice', message: 'Saved' }] });
    const wrapper = mountSheet(); await vi.waitFor(() => expect(wrapper.find('select').exists()).toBe(true));
    await wrapper.get('select').setValue('B');
    const body = wrapper.get('.sheet-body').element;
    const effects = body.querySelector('.effects'); const controls = body.querySelector('.controls');
    expect(effects && controls && effects.compareDocumentPosition(controls) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(wrapper.get('.effects').text()).toContain('Saved');
    wrapper.unmount();
  });
  it('emits Explore refresh effects for the parent state boundary', async () => {
    runSourceInteractionAction.mockResolvedValueOnce({ view, effects: [{ type: 'refresh_explore' }] });
    const wrapper = mountSheet(); await vi.waitFor(() => expect(wrapper.find('select').exists()).toBe(true));
    await wrapper.get('select').setValue('B');
    expect(wrapper.emitted('refresh-explore')).toHaveLength(1);
    wrapper.unmount();
  });
  it('takes focus, locks background scroll, and closes on Escape', async () => {
    const wrapper = mountSheet(); await vi.waitFor(() => expect(wrapper.find('aside').exists()).toBe(true));
    expect(document.activeElement).toBe(wrapper.get('.close').element);
    expect(document.body.style.overflow).toBe('hidden');
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(wrapper.emitted('close')).toHaveLength(1);
    wrapper.unmount();
    expect(document.body.style.overflow).toBe('');
  });
});
