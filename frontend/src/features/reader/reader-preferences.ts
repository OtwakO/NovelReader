export type ChineseConversionMode = 'original' | 'simplified' | 'traditional';

export interface ReaderPreferences {
  fontSize: number;
  lineHeight: number;
  fontWeight: number;
  pageWidth: number;
  background: string;
  textColor: string;
  fontId: string;
  chineseConversion: ChineseConversionMode;
  keepScreenAwake: boolean;
  showImages: boolean;
}

const key = 'novelreader.reader.preferences.v1';
export const readerPreferenceRanges = {
  fontSize: { min: 10, max: 48, step: 1 },
  lineHeight: { min: 1, max: 3.5, step: 0.05 },
  pageWidth: { min: 280, max: 1600, step: 20 },
} as const;
export const defaultReaderPreferences: ReaderPreferences = { fontSize: 19, lineHeight: 1.85, fontWeight: 400, pageWidth: 720, background: '#f4eedc', textColor: '#24333a', fontId: 'system', chineseConversion: 'original', keepScreenAwake: false, showImages: true };

function storage(): Storage | null {
  try { return typeof window === 'undefined' ? null : window.localStorage; } catch { return null; }
}

function bounded(value: unknown, fallback: number, min: number, max: number): number {
  const number = Number(value); return Number.isFinite(number) ? Math.min(max, Math.max(min, number)) : fallback;
}

export function loadReaderPreferences(): ReaderPreferences {
  try {
    const parsed = JSON.parse(storage()?.getItem(key) || '{}') as Partial<ReaderPreferences>;
    const chineseConversion = parsed.chineseConversion === 'simplified' || parsed.chineseConversion === 'traditional' ? parsed.chineseConversion : 'original';
    return { fontSize: bounded(parsed.fontSize, 19, readerPreferenceRanges.fontSize.min, readerPreferenceRanges.fontSize.max), lineHeight: bounded(parsed.lineHeight, 1.85, readerPreferenceRanges.lineHeight.min, readerPreferenceRanges.lineHeight.max), fontWeight: bounded(parsed.fontWeight, 400, 300, 700), pageWidth: bounded(parsed.pageWidth, 720, readerPreferenceRanges.pageWidth.min, readerPreferenceRanges.pageWidth.max), background: /^#[0-9a-f]{6}$/i.test(parsed.background || '') ? String(parsed.background) : defaultReaderPreferences.background, textColor: /^#[0-9a-f]{6}$/i.test(parsed.textColor || '') ? String(parsed.textColor) : defaultReaderPreferences.textColor, fontId: typeof parsed.fontId === 'string' ? parsed.fontId : 'system', chineseConversion, keepScreenAwake: parsed.keepScreenAwake === true, showImages: parsed.showImages !== false };
  } catch { return { ...defaultReaderPreferences }; }
}

export function saveReaderPreferences(preferences: ReaderPreferences): void {
  try { storage()?.setItem(key, JSON.stringify(preferences)); } catch { /* preference persistence is optional */ }
}
