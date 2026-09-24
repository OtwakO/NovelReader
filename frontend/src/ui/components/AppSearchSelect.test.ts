import { mount, type VueWrapper } from '@vue/test-utils';
import { afterEach, expect, it } from 'vitest';
import AppSearchSelect from './AppSearchSelect.vue';

let view: VueWrapper;
afterEach(() => view?.unmount());
function picker() {
  view = mount(AppSearchSelect, { attachTo: document.body, props: {
    id: 'sources', modelValue: 'a', label: 'Source', placeholder: 'Choose', searchLabel: 'Find source', emptyLabel: 'No matches',
    options: [{ value: 'a', label: 'Alpha', description: 'English' }, { value: 'b', label: '乙書源', description: '中文' }, { value: 'c', label: 'Gamma', description: 'English' }],
  } });
  return view;
}
it('filters names and groups locally, then commits only the keyboard-selected option', async () => {
  picker();
  await view.get('button').trigger('click');
  const input = view.get('input');
  expect(document.activeElement).toBe(input.element);
  await input.setValue(' ENGLISH ');
  expect(view.findAll('[role="option"]')).toHaveLength(2);
  expect(view.get('button').text()).toContain('Alpha');
  expect(view.emitted('update:modelValue')).toBeUndefined();
  await input.trigger('keydown', { key: 'ArrowDown' });
  expect(input.attributes('aria-activedescendant')).toBe('sources-option-1');
  await input.trigger('keydown', { key: 'Enter' });
  expect(view.emitted('update:modelValue')).toEqual([['c']]);
  expect(view.find('input').exists()).toBe(false);
  expect(document.activeElement).toBe(view.get('button').element);
});
it('supports CJK search, empty results and cancellation without changing selection', async () => {
  picker();
  await view.get('button').trigger('keydown', { key: 'ArrowDown' });
  await view.get('input').setValue('乙');
  await view.get('input').trigger('keydown', { key: 'Enter', isComposing: true });
  expect(view.emitted('update:modelValue')).toBeUndefined();
  expect(view.findAll('[role="option"]')).toHaveLength(1);
  await view.get('input').setValue('missing');
  expect(view.get('[role="status"]').text()).toBe('No matches');
  expect(view.get('input').attributes('aria-activedescendant')).toBeUndefined();
  await view.get('input').trigger('keydown', { key: 'Enter' });
  await view.get('input').trigger('keydown', { key: 'Escape' });
  expect(view.emitted('update:modelValue')).toBeUndefined();
  expect(document.activeElement).toBe(view.get('button').element);
  await view.get('button').trigger('click');
  expect((view.get('input').element as HTMLInputElement).value).toBe('');
  await view.get('input').trigger('keydown', { key: 'Tab' });
  expect(view.find('input').exists()).toBe(false);
});
it('dismisses outside, closes when disabled and commits pointer selections', async () => {
  picker();
  await view.get('button').trigger('click');
  document.body.dispatchEvent(new Event('pointerdown', { bubbles: true }));
  await view.vm.$nextTick();
  expect(view.find('input').exists()).toBe(false);
  await view.get('button').trigger('click');
  await view.setProps({ disabled: true });
  expect(view.find('input').exists()).toBe(false);
  expect(view.get('button').attributes('disabled')).toBeDefined();
  await view.setProps({ disabled: false });
  await view.get('button').trigger('click');
  await view.findAll('[role="option"]')[1]!.trigger('click');
  expect(view.emitted('update:modelValue')).toEqual([['b']]);
});
