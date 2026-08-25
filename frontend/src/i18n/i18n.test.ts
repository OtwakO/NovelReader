import { beforeEach, describe, expect, it, vi } from 'vitest';
import { detectInitialLocale, persistLocale } from './locales';
import { i18n, setLocale } from './index';
import enApp from './messages/en/app';
import enAccount from './messages/en/account';
import enShelf from './messages/en/shelf';
import enSearch from './messages/en/search';
import enCandidate from './messages/en/candidate';
import enCandidateBookDetail from './messages/en/candidateBookDetail';
import enBookDetail from './messages/en/bookDetail';
import enSourceRecovery from './messages/en/sourceRecovery';
import enReader from './messages/en/reader';
import enExplore from './messages/en/explore';
import enSources from './messages/en/sources';
import enSettings from './messages/en/settings';
import zhCnApp from './messages/zh-CN/app';
import zhCnAccount from './messages/zh-CN/account';
import zhCnShelf from './messages/zh-CN/shelf';
import zhCnSearch from './messages/zh-CN/search';
import zhCnCandidate from './messages/zh-CN/candidate';
import zhCnCandidateBookDetail from './messages/zh-CN/candidateBookDetail';
import zhCnBookDetail from './messages/zh-CN/bookDetail';
import zhCnSourceRecovery from './messages/zh-CN/sourceRecovery';
import zhCnReader from './messages/zh-CN/reader';
import zhCnExplore from './messages/zh-CN/explore';
import zhCnSources from './messages/zh-CN/sources';
import zhCnSettings from './messages/zh-CN/settings';
import zhTwApp from './messages/zh-TW/app';
import zhTwAccount from './messages/zh-TW/account';
import zhTwShelf from './messages/zh-TW/shelf';
import zhTwSearch from './messages/zh-TW/search';
import zhTwCandidate from './messages/zh-TW/candidate';
import zhTwCandidateBookDetail from './messages/zh-TW/candidateBookDetail';
import zhTwBookDetail from './messages/zh-TW/bookDetail';
import zhTwSourceRecovery from './messages/zh-TW/sourceRecovery';
import zhTwReader from './messages/zh-TW/reader';
import zhTwExplore from './messages/zh-TW/explore';
import zhTwSources from './messages/zh-TW/sources';
import zhTwSettings from './messages/zh-TW/settings';

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
    const english = messageKeys({ app: enApp, account: enAccount, shelf: enShelf, search: enSearch, candidate: enCandidate, candidateBookDetail: enCandidateBookDetail, bookDetail: enBookDetail, sourceRecovery: enSourceRecovery, reader: enReader, explore: enExplore, sources: enSources, settings: enSettings }).sort();
    expect(messageKeys({ app: zhCnApp, account: zhCnAccount, shelf: zhCnShelf, search: zhCnSearch, candidate: zhCnCandidate, candidateBookDetail: zhCnCandidateBookDetail, bookDetail: zhCnBookDetail, sourceRecovery: zhCnSourceRecovery, reader: zhCnReader, explore: zhCnExplore, sources: zhCnSources, settings: zhCnSettings }).sort()).toEqual(english);
    expect(messageKeys({ app: zhTwApp, account: zhTwAccount, shelf: zhTwShelf, search: zhTwSearch, candidate: zhTwCandidate, candidateBookDetail: zhTwCandidateBookDetail, bookDetail: zhTwBookDetail, sourceRecovery: zhTwSourceRecovery, reader: zhTwReader, explore: zhTwExplore, sources: zhTwSources, settings: zhTwSettings }).sort()).toEqual(english);
  });
});
