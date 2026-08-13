import { beforeEach, describe, expect, it, vi } from 'vitest';
import { defaultReaderPreferences, loadReaderPreferences, saveReaderPreferences } from './reader-preferences';

describe('reader preferences', () => {
  const values = new Map<string, string>();
  beforeEach(() => { values.clear(); vi.stubGlobal('localStorage', { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => values.set(key, value), removeItem: (key: string) => values.delete(key), clear: () => values.clear() }); });
  it('loads defaults when no preference exists', () => expect(loadReaderPreferences()).toEqual(defaultReaderPreferences));
  it('persists and bounds reader settings', () => { saveReaderPreferences({ ...defaultReaderPreferences, fontSize: 25, pageWidth: 900 }); expect(loadReaderPreferences()).toMatchObject({ fontSize: 25, pageWidth: 900 }); localStorage.setItem('novelreader.reader.preferences.v1', JSON.stringify({ fontSize: 99, lineHeight: 0 })); expect(loadReaderPreferences()).toMatchObject({ fontSize: 32, lineHeight: 1.3 }); });
});
