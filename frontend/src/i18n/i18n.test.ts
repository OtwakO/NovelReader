import { beforeEach, describe, expect, it, vi } from 'vitest';
import { detectInitialLocale, persistLocale } from './locales';
import { i18n, setLocale } from './index';
import enApp from './messages/en/app';
import enAccount from './messages/en/account';
import enShelf from './messages/en/shelf';
import zhCnApp from './messages/zh-CN/app';
import zhCnAccount from './messages/zh-CN/account';
import zhCnShelf from './messages/zh-CN/shelf';
import zhTwApp from './messages/zh-TW/app';
import zhTwAccount from './messages/zh-TW/account';
import zhTwShelf from './messages/zh-TW/shelf';

function messageKeys(value: unknown, prefix = ''): string[] {
  if (!value || typeof value !== 'object') return [prefix];
  return Object.entries(value).flatMap(([key, child]) => messageKeys(child, prefix ? `${prefix}.${key}` : key));
}

const values = new Map<string, string>();
const storage = {
  getItem: (key: string) => values.get(key) ?? null,
  setItem: (key: string, value: string) => values.set(key, value),
  removeItem: (key: string) => values.delete(key),
  clear: () => values.clear(),
  key: (index: number) => [...values.keys()][index] ?? null,
  get length() { return values.size; },
};

beforeEach(() => {
  storage.clear();
  vi.stubGlobal('localStorage', storage);
  vi.restoreAllMocks();
});

describe('locale policy', () => {
  it('uses a persisted supported locale before browser preferences', () => {
    persistLocale('zh-TW');
    expect(detectInitialLocale()).toBe('zh-TW');
  });

  it('falls back to English for unsupported browser languages', () => {
    vi.spyOn(window.navigator, 'languages', 'get').mockReturnValue(['fr-FR']);
    expect(detectInitialLocale()).toBe('en');
  });

  it('loads locale messages and updates the document language', async () => {
    await setLocale('zh-CN');
    expect(i18n.global.t('app.navigation.shelf')).toBe('书架');
    expect(document.documentElement.lang).toBe('zh-CN');
  });

  it('keeps every supported locale on the same message contract', () => {
    const english = messageKeys({ app: enApp, account: enAccount, shelf: enShelf }).sort();
    expect(messageKeys({ app: zhCnApp, account: zhCnAccount, shelf: zhCnShelf }).sort()).toEqual(english);
    expect(messageKeys({ app: zhTwApp, account: zhTwAccount, shelf: zhTwShelf }).sort()).toEqual(english);
  });
});
