import { beforeEach, describe, expect, it, vi } from 'vitest';
import { defaultReaderPreferences, loadReaderPreferences, readerPreferenceRanges, saveReaderPreferences } from './reader-preferences';

describe('reader preferences', () => {
  const values = new Map<string, string>();
  beforeEach(() => { values.clear(); vi.stubGlobal('localStorage', { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => values.set(key, value), removeItem: (key: string) => values.delete(key), clear: () => values.clear() }); });
  it('loads defaults when no preference exists', () => expect(loadReaderPreferences()).toEqual(defaultReaderPreferences));
  it('persists and bounds reader settings', () => { saveReaderPreferences({ ...defaultReaderPreferences, fontSize: 40, lineHeight: 3, pageWidth: 1400, chineseConversion: 'traditional', keepScreenAwake: true, showImages: false, prefetchNextChapter: false }); expect(loadReaderPreferences()).toMatchObject({ fontSize: 40, lineHeight: 3, pageWidth: 1400, chineseConversion: 'traditional', keepScreenAwake: true, showImages: false, prefetchNextChapter: false }); localStorage.setItem('novelreader.reader.preferences.v1', JSON.stringify({ fontSize: 99, lineHeight: 0, pageWidth: 9999, chineseConversion: 'invalid' })); expect(loadReaderPreferences()).toMatchObject({ fontSize: readerPreferenceRanges.fontSize.max, lineHeight: readerPreferenceRanges.lineHeight.min, pageWidth: readerPreferenceRanges.pageWidth.max, chineseConversion: 'original', showImages: true, prefetchNextChapter: true }); });
});
