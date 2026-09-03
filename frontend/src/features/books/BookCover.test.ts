import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import BookCover from './BookCover.vue';

describe('BookCover', () => {
  it('adds a subdued backdrop only when the source ratio does not fit the standard cover frame', async () => {
    const wrapper = mount(BookCover, { props: { name: 'Book', url: '/cover.jpg', alt: 'cover' } });
    const image = wrapper.get<HTMLImageElement>('.cover-image');
    Object.defineProperties(image.element, {
      naturalWidth: { value: 1000 },
      naturalHeight: { value: 1000 },
    });
    await image.trigger('load');
    expect(wrapper.find('.cover-backdrop').exists()).toBe(true);

    const standardWrapper = mount(BookCover, { props: { name: 'Book', url: '/standard-cover.jpg', alt: 'cover' } });
    const standard = standardWrapper.get<HTMLImageElement>('.cover-image');
    Object.defineProperties(standard.element, {
      naturalWidth: { value: 600 },
      naturalHeight: { value: 800 },
    });
    await standard.trigger('load');
    expect(standardWrapper.find('.cover-backdrop').exists()).toBe(false);
  });

  it('falls back to the book initial when the remote image fails', async () => {
    const wrapper = mount(BookCover, { props: { name: '凡人修仙传', url: 'https://example.invalid/cover.jpg', alt: 'cover' } });
    expect(wrapper.find('img').exists()).toBe(true);
    await wrapper.get('.cover-image').trigger('error');
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
