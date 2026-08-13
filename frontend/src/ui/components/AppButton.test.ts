import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import AppButton from './AppButton.vue';

describe('AppButton', () => {
  it('applies busy and disabled semantics', () => {
    const wrapper = mount(AppButton, { props: { busy: true }, slots: { default: '保存' } });
    expect(wrapper.get('button').attributes('disabled')).toBeDefined();
    expect(wrapper.get('button').attributes('aria-busy')).toBe('true');
    expect(wrapper.text()).toBe('保存');
  });
});
