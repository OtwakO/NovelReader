import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import BookCover from './BookCover.vue';

describe('BookCover', () => {
  it('falls back to the book initial when the remote image fails', async () => {
    const wrapper = mount(BookCover, { props: { name: '凡人修仙传', url: 'https://example.invalid/cover.jpg', alt: 'cover' } });
    expect(wrapper.find('img').exists()).toBe(true);
    await wrapper.find('img').trigger('error');
    expect(wrapper.find('img').exists()).toBe(false);
    expect(wrapper.text()).toContain('凡');
  });
});
