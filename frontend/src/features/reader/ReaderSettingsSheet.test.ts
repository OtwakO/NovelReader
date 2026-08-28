import { mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { describe, expect, it } from 'vitest';
import ReaderSettingsSheet from './ReaderSettingsSheet.vue';
import { defaultReaderPreferences } from './reader-preferences';

const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: 'en',
  messages: {
    en: {
      reader: {
        close: 'Close',
        settings: {
          title: 'Typography', fontSize: 'Font size', lineHeight: 'Line height', pageWidth: 'Page width', weight: 'Weight', font: 'Font', systemFont: 'System font', chineseConversion: 'Chinese conversion', chineseOriginal: 'Original', chineseSimplified: 'Simplified Chinese', chineseTraditional: 'Traditional Chinese', conversionUnavailable: 'Chinese conversion is unavailable in this server build.', keepAwake: 'Keep awake', background: 'Background', text: 'Text', sepia: 'Sepia', light: 'Light', dark: 'Dark',
        },
      },
    },
  },
});

describe('ReaderSettingsSheet', () => {
  it('disables native conversion choices when the server confirms it is unavailable', () => {
    const wrapper = mount(ReaderSettingsSheet, {
      global: { plugins: [i18n] },
      props: {
        modelValue: { ...defaultReaderPreferences },
        conversionCapability: { available: false, modes: [] },
      },
    });

    const options = wrapper.findAll('select').find(select => select.text().includes('Traditional Chinese'))?.findAll('option');
    expect(options?.map(option => option.attributes('disabled'))).toEqual([undefined, '', '']);
    expect(wrapper.text()).toContain('Chinese conversion is unavailable in this server build.');
  });
});
