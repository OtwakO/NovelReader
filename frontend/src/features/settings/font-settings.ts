import type { Font } from '../../api/reader';
import { defaultReaderPreferences, type ReaderPreferences } from '../reader/reader-preferences';

const fontExtensions = /\.(ttf|otf|woff|woff2)$/i;
export const maximumFontBytes = 20 * 1024 * 1024;

export type FontValidation = 'ok' | 'type' | 'size' | 'empty';

export function validateFontFile(file: Pick<File, 'name' | 'size'>): FontValidation {
  if (file.size <= 0) return 'empty';
  if (file.size > maximumFontBytes) return 'size';
  return fontExtensions.test(file.name) ? 'ok' : 'type';
}

export function fontNameFromFile(fileName: string): string {
  return fileName.replace(/\.[^.]+$/, '').trim();
}

export function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  if (bytes < 1024) return `${Math.round(bytes)} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(bytes < 10 * 1024 ? 1 : 0)} KiB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
}

export function preferencesAfterFontDeletion(preferences: ReaderPreferences, deletedFontId: string): ReaderPreferences {
  return preferences.fontId === deletedFontId ? { ...preferences, fontId: defaultReaderPreferences.fontId } : preferences;
}

export function availablePreferenceFont(preferences: ReaderPreferences, fonts: Font[]): ReaderPreferences {
  return preferences.fontId === 'system' || fonts.some(font => font.id === preferences.fontId) ? preferences : { ...preferences, fontId: 'system' };
}
