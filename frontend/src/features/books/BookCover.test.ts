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


describe('same-origin cover delivery', () => {
  it('renders the backend display URL instead of the raw HTTP source URL', () => {
    const wrapper = mount(BookCover, { props: { name: '猫眼看书', url: '/api/covers/signed-reference', alt: 'cover' } });
    expect(wrapper.get('img').attributes('src')).toBe('/api/covers/signed-reference');
  });
});
