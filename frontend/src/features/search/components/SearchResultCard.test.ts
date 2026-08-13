import { mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { describe, expect, it } from 'vitest';
import SearchResultCard from './SearchResultCard.vue';

const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: 'en',
  messages: {
    en: {
      search: { results: { detailsFor: 'View {name}', sources: '{count} sources', details: 'Details', shelve: 'Add', shelving: 'Adding' } },
      app: { common: { unknownAuthor: 'Unknown' } },
    },
  },
});

describe('SearchResultCard', () => {
  it('exposes separate detail and shelf actions', async () => {
    const wrapper = mount(SearchResultCard, { global: { plugins: [i18n] }, props: { result: { name: 'Book', author: 'Author', coverUrl: '', intro: '', kind: '', lastChapter: '', bookUrl: '/book', sourceUrl: 'source', sourceName: 'Source', alternateSources: [{ sourceUrl: 'other', bookUrl: '/other', sourceName: 'Other' }] } } });
    await wrapper.get('.main').trigger('click'); await wrapper.findAll('button')[1]?.trigger('click');
    expect(wrapper.emitted('open')).toHaveLength(1); expect(wrapper.emitted('shelve')).toHaveLength(1); expect(wrapper.text()).toContain('2 sources');
  });
});
