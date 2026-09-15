import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import AppDisclosure from './AppDisclosure.vue';

describe('AppDisclosure', () => {
  it('uses native disclosure semantics and forwards open changes without owning feature state', async () => {
    const view = mount(AppDisclosure, { slots: { summary: 'Advanced settings', default: '<input aria-label="Fine control">' } });
    expect(view.element.tagName).toBe('DETAILS');
    expect(view.get('summary').text()).toBe('Advanced settings');
    expect(view.findAll('summary svg')).toHaveLength(1);
    const details = view.element as HTMLDetailsElement;
    expect(details.open).toBe(false);
    details.open = true;
    await view.trigger('toggle');
    expect(view.emitted('update:open')?.at(-1)).toEqual([true]);
    expect((view.emitted('toggle')?.at(-1)?.[0] as Event).target).toBe(details);
    await view.setProps({ open: true });
    await view.setProps({ open: false });
    expect(details.open).toBe(false);
    expect(view.get('input').attributes('aria-label')).toBe('Fine control');
  });
});
