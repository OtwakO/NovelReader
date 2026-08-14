import { describe, expect, it } from 'vitest';
import { defaultReaderPreferences } from '../reader/reader-preferences';
import { availablePreferenceFont, fontNameFromFile, formatFileSize, maximumFontBytes, preferencesAfterFontDeletion, validateFontFile } from './font-settings';

describe('font settings', () => {
  it('validates supported files against the backend multipart limit', () => { expect(validateFontFile({name:'Novel.ttf',size:1024} as File)).toBe('ok'); expect(validateFontFile({name:'Novel.txt',size:1024} as File)).toBe('type'); expect(validateFontFile({name:'Novel.woff2',size:maximumFontBytes+1} as File)).toBe('size'); expect(validateFontFile({name:'Novel.otf',size:0} as File)).toBe('empty'); });
  it('derives readable names and file sizes', () => { expect(fontNameFromFile('Noto Serif SC.ttf')).toBe('Noto Serif SC'); expect(formatFileSize(1536)).toBe('1.5 KiB'); expect(formatFileSize(2*1024*1024)).toBe('2.0 MiB'); });
  it('resets only unavailable or deleted selected fonts', () => { const selected={...defaultReaderPreferences,fontId:'font-1'}; expect(preferencesAfterFontDeletion(selected,'font-1').fontId).toBe('system'); expect(preferencesAfterFontDeletion(selected,'font-2')).toBe(selected); expect(availablePreferenceFont(selected,[]).fontId).toBe('system'); expect(availablePreferenceFont(selected,[{id:'font-1',name:'Font',fileName:'font-1',fileSize:1}])).toBe(selected); });
});
