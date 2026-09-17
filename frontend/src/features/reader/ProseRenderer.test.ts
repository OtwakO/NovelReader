import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import ProseRenderer from './ProseRenderer.vue';
import { parseStructuredChapterContent } from '../../api/structured-prose';
import { structuredProseFixture } from '../../api/structured-prose.fixture';

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

it('renders structured semantics safely and emits qualified navigation without reader tap propagation', async () => {
  const content = parseStructuredChapterContent(structuredProseFixture());
  const wrapper = mount(ProseRenderer, { props: { document: content.document, showImages: true, fallbackImageAlt: 'Illustration', imageUnavailable: 'Unavailable', targetHref: target => `/proof/${target.chapterIndex}?contentRevision=${target.contentRevision}` } });
  expect(wrapper.find('script').exists()).toBe(false);
  expect(wrapper.text()).toContain('<script>literal</script>');
  expect(wrapper.find('h1').exists()).toBe(false);
  expect(wrapper.find('h2').text()).toBe('章节');
  expect(wrapper.find('ol').attributes('start')).toBe('0');
  expect(wrapper.find('rt').text()).toBe('じ');
  expect(wrapper.find('td').attributes('colspan')).toBe('2');
  expect(wrapper.find('img').attributes('width')).toBe('640');
  expect(wrapper.find('img').attributes('height')).toBe('480');
  expect(wrapper.find('[aria-disabled="true"]').text()).toBe('Unavailable');
  expect(wrapper.find('a[target="_blank"]').attributes('rel')).toBe('noopener noreferrer');
  expect(wrapper.vm.findAnchor('a1')).toBe(wrapper.find('[data-prose-anchor="a1"]').element);
  expect(wrapper.vm.findAnchor('a1"] body')).toBeUndefined();
  let bubbled = false;
  wrapper.element.addEventListener('click', () => { bubbled = true; });
  await wrapper.find('a').trigger('click', { button: 0 });
  expect(bubbled).toBe(false);
  expect(wrapper.emitted('navigate')).toEqual([[{ target: { chapterIndex: 1, contentRevision: 9, anchor: 'a1' }, note: true }]]);
  await wrapper.find('img').trigger('error');
  expect(wrapper.find('[role="status"]').text()).toBe('地图 — Unavailable');
  await wrapper.setProps({ showImages: false });
  expect(wrapper.find('img').exists()).toBe(false);
  expect(wrapper.find('figcaption').text()).toBe('图片说明');
});
