import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import ProseRenderer from './ProseRenderer.vue';

const document = {
  kind: 'prose' as const,
  title: 'A chapter',
  blocks: [
    { kind: 'paragraph' as const, text: 'Before the map.' },
    { kind: 'image' as const, resource: { href: '/api/reading/map' }, alt: 'Map of the northern road' },
    { kind: 'image' as const, resource: { href: '/api/reading/portrait' } },
  ],
};

describe('ProseRenderer', () => {
  it('renders centered semantic figures and only visible source captions', async () => {
    const wrapper = mount(ProseRenderer, {
      props: { document, fallbackImageAlt: 'Illustration from A chapter', imageUnavailable: 'Image unavailable', showImages: true },
    });

    const figures = wrapper.findAll('figure');
    expect(figures).toHaveLength(2);
    const [captionedFigure, uncaptionedFigure] = figures;
    if (!captionedFigure || !uncaptionedFigure) throw new Error('expected two figures');
    expect(captionedFigure.find('img').attributes('alt')).toBe('Map of the northern road');
    expect(captionedFigure.find('figcaption').text()).toBe('Map of the northern road');
    expect(uncaptionedFigure.find('img').attributes('alt')).toBe('Illustration from A chapter');
    expect(uncaptionedFigure.find('figcaption').exists()).toBe(false);

    await captionedFigure.find('img').trigger('error');
    expect(captionedFigure.text()).toContain('Image unavailable');
    expect(wrapper.text()).toContain('Before the map.');
  });

  it('omits image blocks entirely when images are disabled', () => {
    const wrapper = mount(ProseRenderer, {
      props: { document, fallbackImageAlt: 'Illustration from A chapter', imageUnavailable: 'Image unavailable', showImages: false },
    });

    expect(wrapper.findAll('figure')).toHaveLength(0);
    expect(wrapper.findAll('img')).toHaveLength(0);
    expect(wrapper.text()).toContain('Before the map.');
    expect(wrapper.text()).not.toContain('Map of the northern road');
  });
});
