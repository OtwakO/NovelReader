import { mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { describe, expect, it } from 'vitest';
import ReaderSettingsSheet from './ReaderSettingsSheet.vue';
import readerMessages from '../../i18n/messages/en/reader';
import { defaultReaderPreferences, readerPreferenceRanges } from './reader-preferences';

const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: 'en',
  messages: { en: { reader: readerMessages } },
});

describe('ReaderSettingsSheet', () => {
  it('uses the shared expanded typography ranges', () => {
    const wrapper = mount(ReaderSettingsSheet, {
      global: { plugins: [i18n] },
      props: { modelValue: { ...defaultReaderPreferences } },
    });

    const sliders = wrapper.findAll('input[type="range"]');
    expect(sliders.map(slider => ({ min: slider.attributes('min'), max: slider.attributes('max'), step: slider.attributes('step') }))).toEqual([
      { min: String(readerPreferenceRanges.fontSize.min), max: String(readerPreferenceRanges.fontSize.max), step: String(readerPreferenceRanges.fontSize.step) },
      { min: String(readerPreferenceRanges.lineHeight.min), max: String(readerPreferenceRanges.lineHeight.max), step: String(readerPreferenceRanges.lineHeight.step) },
      { min: String(readerPreferenceRanges.pageWidth.min), max: String(readerPreferenceRanges.pageWidth.max), step: String(readerPreferenceRanges.pageWidth.step) },
    ]);
  });

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
