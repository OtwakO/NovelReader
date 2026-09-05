import { mount } from '@vue/test-utils';
import { expect, it } from 'vitest';
import ReaderActionsMenu from './ReaderActionsMenu.vue';

it('supports keyboard dismissal, bookmark action, and disabled refetch', async () => {
  const wrapper=mount(ReaderActionsMenu,{attachTo:document.body,props:{refreshDisabled:true},global:{mocks:{$t:(key:string)=>key}}});
  try {
    const trigger=wrapper.get('.trigger');
    await trigger.trigger('click');
    expect(trigger.attributes('aria-expanded')).toBe('true');
    expect(wrapper.findAll('[role="menuitem"] .icon[aria-hidden="true"]')).toHaveLength(2);
    expect(wrapper.find('.icon-bookmark svg').exists()).toBe(true);
    expect(wrapper.find('.icon-refresh svg').exists()).toBe(true);
    expect(wrapper.findAll('[role="menuitem"]')[1]!.attributes('disabled')).toBeDefined();
    await wrapper.get('[role="menu"]').trigger('keydown',{key:'Escape'});
    expect(wrapper.find('[role="menu"]').exists()).toBe(false);
    expect(document.activeElement).toBe(trigger.element);
    await trigger.trigger('click');
    await wrapper.get('[role="menuitem"]').trigger('click');
    expect(wrapper.emitted('bookmarks')).toHaveLength(1);
    await wrapper.setProps({refreshDisabled:false});
    await trigger.trigger('click');
    await wrapper.findAll('[role="menuitem"]')[1]!.trigger('click');
    expect(wrapper.emitted('refetch')).toHaveLength(1);
  } finally { wrapper.unmount(); }
});
