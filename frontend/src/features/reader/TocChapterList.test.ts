import { mount, RouterLinkStub } from '@vue/test-utils';
import { describe, expect, test } from 'vitest';
import type { Chapter } from '../../api/models';
import TocChapterList from './TocChapterList.vue';

const chapters: Chapter[] = [
  { index: 0, title: 'Volume One', isVolume: true },
  { index: 1, title: 'A deliberately long chapter title', isVolume: false },
];

describe('TocChapterList', () => {
  test('renders editorial rows with canonical indexes and current state', () => {
    const wrapper = mount(TocChapterList, {
      props: { chapters, currentIndex: 1 },
      global: { stubs: { RouterLink: { props: ['to'], template: '<a :href="String(to)"><slot /></a>' } } },
    });

    expect(wrapper.find('.volume-title').text()).toBe('Volume One');
    expect(wrapper.find('.current').attributes('data-current')).toBe('true');
    expect(wrapper.find('.current small').text()).toBe('2');
    expect(wrapper.find('.current .chapter-title').text()).toBe('A deliberately long chapter title');
    expect(wrapper.find('.row-arrow').exists()).toBe(true);
  });

  test('emits the canonical index when used as the Reader action list', async () => {
    const wrapper = mount(TocChapterList, {
      props: { chapters, currentIndex: 1 },
      global: { stubs: { RouterLink: RouterLinkStub } },
    });
    await wrapper.find('button').trigger('click');
    expect(wrapper.emitted('open')).toEqual([[1]]);
  });
});
