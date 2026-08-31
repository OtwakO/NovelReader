import { mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { describe, expect, it } from 'vitest';
import ReaderSourceSheet from './ReaderSourceSheet.vue';

const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: 'en',
  messages: {
    en: {
      reader: {
        close: 'Close',
        sources: { title: 'Sources', current: 'Current: {source}' },
      },
    },
  },
});

describe('ReaderSourceSheet', () => {
  it('keeps rounded clipping on a stable shell outside the scroll container', () => {
    const wrapper = mount(ReaderSourceSheet, {
      global: {
        plugins: [i18n],
        stubs: { SourceRecoveryPanel: true },
      },
      props: {
        book: { name: 'Book', author: 'Author' },
        currentSource: 'Current source',
        currentSourceId: 'https://current.example',
        currentBookUrl: '/current-book',
        onClearAndRescan: async () => undefined,
      },
    });

    const sheet = wrapper.get('.sheet');
    const scrollContainer = wrapper.get('.sheet-scroll');

    expect(scrollContainer.element.parentElement).toBe(sheet.element);
    expect(sheet.element.classList).not.toContain('sheet-scroll');
  });
});
